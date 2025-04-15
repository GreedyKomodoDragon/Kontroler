package workers

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"kontroler-controller/api/v1alpha1"
	"kontroler-controller/internal/db"
	"kontroler-controller/internal/object"
	"kontroler-controller/internal/queue"
	"kontroler-controller/internal/webhook"

	"github.com/google/uuid"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

type Worker[T any] interface {
	Push(item T, event string) error
	Run(ctx context.Context) error
	Queue() queue.Queue
	ID() string
}

type worker struct {
	queue           queue.Queue
	dbManager       db.DBDAGManager
	clientSet       *kubernetes.Clientset
	taskAllocator   TaskAllocator
	logStore        object.LogStore
	webhookNotifier webhook.WebhookNotifier
	id              string
	pollDuration    time.Duration
}

func NewWorker(queue queue.Queue, logStore object.LogStore, webhookChan chan webhook.WebhookPayload,
	dbManager db.DBDAGManager, clientSet *kubernetes.Clientset, taskAllocator TaskAllocator,
	pollDuration time.Duration) Worker[*v1.Pod] {
	return &worker{
		queue:           queue,
		logStore:        logStore,
		webhookNotifier: webhook.NewWebhookNotifier(webhookChan),
		dbManager:       dbManager,
		clientSet:       clientSet,
		taskAllocator:   taskAllocator,
		id:              uuid.NewString(),
		pollDuration:    pollDuration,
	}
}

func (w *worker) ID() string {
	return w.id
}

func (w *worker) Queue() queue.Queue {
	return w.queue
}

func (w *worker) Push(pod *v1.Pod, event string) error {
	return w.queue.Push(&queue.PodEvent{
		Pod:   pod,
		Event: event,
	})
}

func (w *worker) Run(ctx context.Context) error {
	log.Log.Info("worker started")
	tmr := time.NewTimer(w.pollDuration)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tmr.C:
			tmr.Reset(w.pollDuration)

			podEvent, err := w.queue.Pop()
			if err != nil {
				if errors.Is(err, queue.ErrQueueIsEmpty) {
					continue
				}

				log.Log.Error(err, "failed to pop pod event from queue")
				continue
			}

			switch podEvent.Event {
			case "add":
				log.Log.Info("pod was added", "podUID", podEvent.Pod.UID, "name", podEvent.Pod.Name)
				w.handleAdd(podEvent.Pod, podEvent.EventTime)
			case "update":
				log.Log.Info("pod was updated", "podUID", podEvent.Pod.UID, "name", podEvent.Pod.Name)
				w.handleUpdate(podEvent.Pod, podEvent.EventTime)
			default:
				log.Log.Info("unknown event", "event", podEvent.Event)
			}
		}
	}
}

func (w *worker) handleAdd(pod *v1.Pod, eventTime *time.Time) {
	w.handleOutcome(pod, "add", eventTime)

}

func (t *worker) handleUpdate(pod *v1.Pod, eventTime *time.Time) {
	t.handleOutcome(pod, "update", eventTime)
}

func (w *worker) handleOutcome(pod *v1.Pod, event string, eventTime *time.Time) {
	ctx := context.Background()
	log.Log.Info("pod event", "worker", w.id, "podUID", pod.UID, "name", pod.Name, "event", event, "eventTime", eventTime)

	taskRunId, err := w.getTaskRunID(pod)
	if err != nil {
		log.Log.Error(err, "failed to get task run ID", "pod", pod.Name)
		return
	}

	w.handleLogCollection(ctx, pod)

	writeState := true

	switch pod.Status.Phase {
	case v1.PodSucceeded:
		w.handleSuccessfulTaskRun(ctx, pod, taskRunId)
	case v1.PodFailed:
		w.handleFailedTaskRun(ctx, pod, taskRunId)
	case v1.PodRunning:
		w.handleStartedTaskRun(ctx, pod, taskRunId)
	case v1.PodPending:
		// there is a special case if config error is detected
		// in that case we treat it as a failure
		// and do not write to db as handleConfigError will do that
		writeState = w.handlePendingTaskRun(ctx, pod, taskRunId)
	case v1.PodUnknown:
		log.Log.Info("pod status unknown", "podUID", pod.UID, "name", pod.Name, "event", event)
	}

	if writeState {
		if err := w.writeStatusToDB(pod, eventTime); err != nil {
			log.Log.Error(err, "failed to writeStatusToDB")
		}
	}
}

func (w *worker) handleSuccessfulTaskRun(ctx context.Context, pod *v1.Pod, taskRunId int) {
	log.Log.Info("task succeeded", "podUID", pod.UID, "name", pod.Name, "taskRunId", taskRunId)

	dagRunId, err := w.getDagRunID(pod)
	if err != nil {
		log.Log.Error(err, "failed to get dag run ID", "pod", pod.Name)
		return
	}

	if err := w.deletePod(ctx, pod, false); err != nil {
		log.Log.Info("pod has already been deleted/handled, skipping", "podUId", pod.UID)
		return
	}

	tasks, err := w.dbManager.MarkSuccessAndGetNextTasks(ctx, taskRunId)
	if err != nil {
		log.Log.Error(err, "failed to mark outcome and get next task", "podUID", pod.UID, "name", pod.Name, "event", "add/update")
		return
	}

	webhook, err := w.dbManager.GetWebhookDetails(ctx, dagRunId)
	if err != nil {
		log.Log.Error(err, "failed to get webhook details", "runId", dagRunId)
	} else if webhook.URL != "" {
		go w.webhookNotifier.NotifyTaskRun(pod.Spec.Containers[0].Name, "success", dagRunId, taskRunId, webhook.URL, webhook.VerifySSL)
	}

	log.Log.Info("number of tasks", "tasks", len(tasks))

	if len(tasks) == 0 {
		complete, err := w.checkIfDagRunIsComplete(ctx, dagRunId)
		if err != nil {
			log.Log.Error(err, "failed to check if dag run is complete", "runId", dagRunId)
			return
		}

		if !complete {
			return
		}

		if err := w.deletePVC(ctx, pod); err != nil {
			log.Log.Error(err, "failed to delete PVC", "pod", pod.Name, "namespace", pod.Namespace, "dagRunId", dagRunId, "status", pod.Status.Phase)
		}
		return
	}

	for _, task := range tasks {
		taskRunId, err := w.dbManager.MarkTaskAsStarted(ctx, dagRunId, task.Id)
		if err != nil {
			log.Log.Error(err, "failed to mark task as started", "dagRun_id", dagRunId, "task_id", task.Id)
			continue
		}

		newPod, err := w.taskAllocator.AllocateTask(ctx, &task, dagRunId, taskRunId, pod.Namespace)
		if err != nil {
			log.Log.Error(err, "failed to allocate task", "task.Id", task.Id, "task.Name", task.Name)
			continue
		}

		log.Log.Info("allocated task", "newPodUID", newPod, "task.Id", task.Id, "task.Name", task.Name)
	}
}

func (t *worker) handleFailedTaskRun(ctx context.Context, pod *v1.Pod, taskRunId int) {
	// mark exitcode as -1 if pod was deleted before it started - means something odd happened
	var exitcode int32 = -1
	if len(pod.Status.ContainerStatuses) == 0 {
		log.Log.Info("task failed without container status", "podUID", pod.UID, "name", pod.Name, "taskRunId", taskRunId)
	} else {
		log.Log.Info("task failed", "podUID", pod.UID, "name", pod.Name, "taskRunId", taskRunId, "exitcode", pod.Status.ContainerStatuses[0].State.Terminated.ExitCode)
		exitcode = pod.Status.ContainerStatuses[0].State.Terminated.ExitCode
	}

	dagRunId, err := t.getDagRunID(pod)
	if err != nil {
		log.Log.Error(err, "failed to get dag run ID", "pod", pod.Name)
		return
	}

	if err := t.deletePod(ctx, pod, false); err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Log.Info("pod has already been deleted/handled, skipping", "podUId", pod.UID)
		} else {
			log.Log.Error(err, "failed to delete pod in failed,", "podUId", pod.UID)
			return
		}
	}

	// For pods that failed before container start, always attempt a retry
	ok, err := t.dbManager.ShouldRerun(ctx, taskRunId, exitcode)
	if err != nil {
		log.Log.Error(err, "failed to determine if pod should be re-ran", "pod", pod.Name)
		return
	}

	container := pod.Spec.Containers[0]

	if !ok {
		t.handleUnretryablePod(ctx, pod, taskRunId, dagRunId, exitcode)
		return
	}

	taskIdStr, ok := pod.Annotations["kontroler/task-id"]
	if !ok {
		log.Log.Error(fmt.Errorf("missing annotation"), "annotation", "kontroler/task-id", "pod", pod.Name)
		return
	}

	taskId, err := strconv.Atoi(taskIdStr)
	if err != nil {
		log.Log.Error(fmt.Errorf("failed to convert task id string: %s", taskIdStr), "annotation", "kontroler/task-id", "value", taskIdStr, "pod", pod.Name)
		return
	}

	script, injector, err := t.dbManager.GetTaskScriptAndInjectorImage(ctx, taskId)
	if err != nil {
		log.Log.Error(err, "GetTaskScriptAndInjectorImage failed", "pod", pod.Name)
		return
	}

	dbTask := &db.Task{
		Id:      taskId,
		Name:    container.Name,
		Args:    container.Args,
		Command: container.Command,
		Image:   container.Image,
		PodTemplate: &v1alpha1.PodTemplateSpec{
			Volumes:                      pod.Spec.Volumes,
			VolumeMounts:                 container.VolumeMounts,
			ImagePullSecrets:             pod.Spec.ImagePullSecrets,
			SecurityContext:              pod.Spec.SecurityContext,
			NodeSelector:                 pod.Spec.NodeSelector,
			Tolerations:                  pod.Spec.Tolerations,
			Affinity:                     pod.Spec.Affinity,
			ServiceAccountName:           pod.Spec.ServiceAccountName,
			AutomountServiceAccountToken: pod.Spec.AutomountServiceAccountToken,
			ActiveDeadlineSeconds:        pod.Spec.ActiveDeadlineSeconds,
		},
	}

	if script != nil {
		dbTask.Script = *script
	}

	if injector != nil {
		dbTask.ScriptInjectorImage = *injector
	}

	taskUUID, err := t.taskAllocator.AllocateTaskWithEnv(ctx, dbTask, dagRunId, taskRunId, pod.Namespace, container.Env, &container.Resources)
	if err != nil {
		log.Log.Error(err, "failed to allocate new pod")
		return
	}

	if err := t.dbManager.IncrementAttempts(ctx, taskRunId); err != nil {
		log.Log.Error(err, "failed to increment attempts", "taskRunId", taskRunId)
	}

	log.Log.Info("new task allocated allocated with env", "taskUUID", taskUUID)
}

func (w *worker) writeStatusToDB(pod *v1.Pod, stamp *time.Time) error {
	taskRunId, err := w.getTaskRunID(pod)
	if err != nil {
		return err
	}

	dagRunId, err := w.getDagRunID(pod)
	if err != nil {
		return err
	}

	var exitCode *int32 = nil
	var duration int64 = 0

	if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].State.Terminated != nil {
		exitCode = &pod.Status.ContainerStatuses[0].State.Terminated.ExitCode

		// Use the termination time for final pod status
		stamp = &pod.Status.ContainerStatuses[0].State.Terminated.FinishedAt.Time
		startTime := pod.Status.ContainerStatuses[0].State.Terminated.StartedAt.Time
		duration = int64(stamp.Sub(startTime).Seconds())

		if err := w.dbManager.AddPodDuration(context.Background(), taskRunId, duration); err != nil {
			log.Log.Error(err, "failed to add pod duration", "podUID", pod.UID, "name", pod.Name, "taskRunId", taskRunId)
		}
	} else if pod.Status.Phase == v1.PodFailed {
		// Handle case where pod failed before container started
		defaultExitCode := int32(-1)
		exitCode = &defaultExitCode
	}

	if err := w.dbManager.MarkPodStatus(context.Background(), pod.UID, pod.Name,
		taskRunId, pod.Status.Phase, *stamp, exitCode, pod.Namespace); err != nil {
		log.Log.Error(err, "failed to mark pod status", "podUID", pod.UID, "name", pod.Name, "taskRunId", taskRunId, "status", pod.Status.Phase)
		return fmt.Errorf("failed to mark pod status: %w", err)
	}

	webhook, err := w.dbManager.GetWebhookDetails(context.Background(), dagRunId)
	if err != nil {
		log.Log.Error(err, "failed to get webhook details", "runId", dagRunId)
	} else if webhook.URL != "" {
		go w.webhookNotifier.NotifyPodEvent(
			pod.Spec.Containers[0].Name,
			string(pod.Status.Phase),
			dagRunId,
			taskRunId,
			webhook.URL,
			webhook.VerifySSL,
			int(duration),
		)
	}

	log.Log.Info("pod status written to db", "podUID", pod.UID, "name", pod.Name, "taskRunId", taskRunId, "status", pod.Status.Phase)
	return nil
}

func (w *worker) handleUnretryablePod(ctx context.Context, pod *v1.Pod, taskRunId, dagRunId int, exitCode int32) {
	log.Log.Info("pod has reached it max backoffLimit or exit code not recoverable", "podUID", pod.UID, "name", pod.Name, "exitCode", exitCode)

	if err := w.dbManager.MarkTaskAsFailed(ctx, taskRunId); err != nil {
		log.Log.Error(err, "failed to mark task as failed", "podUID", pod.UID, "name", pod.Name, "event", "add/update")
	}

	webhook, err := w.dbManager.GetWebhookDetails(ctx, dagRunId)
	if err != nil {
		log.Log.Error(err, "failed to get webhook details", "runId", dagRunId)
	} else if webhook.URL != "" {
		go w.webhookNotifier.NotifyTaskRun(pod.Spec.Containers[0].Name, "failed", dagRunId, taskRunId, webhook.URL, webhook.VerifySSL)
	}

	taskNames, err := w.dbManager.MarkConnectingTasksAsSuspended(ctx, dagRunId, taskRunId)
	if err == nil {
		if webhook != nil && webhook.URL != "" {
			for _, taskName := range taskNames {
				log.Log.Info("task marked as suspended", "taskName", taskName)
				go w.webhookNotifier.NotifyTaskRun(taskName, "suspended", dagRunId, taskRunId, webhook.URL, webhook.VerifySSL)
			}
		}
	} else {
		log.Log.Error(err, "failed to mark connecting tasks as suspended", "taskRunId", taskRunId)
		return
	}

	complete, err := w.checkIfDagRunIsComplete(ctx, dagRunId)
	if err != nil {
		log.Log.Error(err, "failed to check if dag run is complete", "runId", dagRunId)
		return
	}

	if !complete {
		return
	}

	if err := w.deletePVC(ctx, pod); err != nil {
		log.Log.Error(err, "failed to delete PVC", "pod", pod.Name, "namespace", pod.Namespace, "dagRunId", dagRunId, "status", pod.Status.Phase)
	}
}

func (t *worker) deletePod(ctx context.Context, pod *v1.Pod, removeFinaliser bool) error {
	if removeFinaliser {
		if err := object.RemoveFinalizer(t.clientSet, pod.Name, pod.Namespace, "kontroler/logcollection"); err != nil {
			log.Log.Error(err, "error removing finalizer", "pod", pod.Name, "namespace", pod.Namespace)
		}
	}

	backgroundDeletion := metav1.DeletePropagationBackground
	return t.clientSet.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{
		PropagationPolicy: &backgroundDeletion,
	})
}

func (t *worker) handleStartedTaskRun(ctx context.Context, pod *v1.Pod, taskRunId int) {
	log.Log.Info("task started", "podUID", pod.UID, "name", pod.Name, "taskRunId", taskRunId)

	dagRunId, err := t.getDagRunID(pod)
	if err != nil {
		log.Log.Error(err, "failed to get dag run ID", "pod", pod.Name)
		return
	}

	webhook, err := t.dbManager.GetWebhookDetails(ctx, dagRunId)
	if err != nil {
		log.Log.Error(err, "failed to get webhook details", "runId", dagRunId)
	} else if webhook.URL != "" {
		t.webhookNotifier.NotifyTaskRun(pod.Spec.Containers[0].Name, "started", dagRunId, taskRunId, webhook.URL, webhook.VerifySSL)
	}
}

func (t *worker) handlePendingTaskRun(ctx context.Context, pod *v1.Pod, taskRunId int) bool {
	dagRunId, err := t.getDagRunID(pod)
	if err != nil {
		log.Log.Error(err, "failed to get dag run ID", "pod", pod.Name)
		return true
	}

	if hasCreateContainerConfigError(pod) {
		log.Log.Info("detected CreateContainerConfigError, treating as failure", "podUID", pod.UID, "name", pod.Name)
		// Treat config error as a special kind of failure
		t.handleConfigError(ctx, pod, taskRunId, dagRunId)
		return false
	}

	log.Log.Info("task pending", "podUID", pod.UID, "name", pod.Name, "taskRunId", taskRunId)

	webhook, err := t.dbManager.GetWebhookDetails(ctx, dagRunId)
	if err != nil {
		log.Log.Error(err, "failed to get webhook details", "runId", dagRunId)
	} else if webhook.URL != "" {
		go t.webhookNotifier.NotifyTaskRun(pod.Spec.Containers[0].Name, "pending", dagRunId, taskRunId, webhook.URL, webhook.VerifySSL)
	}

	return true
}

func (t *worker) handleConfigError(ctx context.Context, pod *v1.Pod, taskRunId, dagRunId int) {
	// Config errors are typically unrecoverable, so mark as failed immediately
	if err := t.dbManager.MarkTaskAsFailed(ctx, taskRunId); err != nil {
		log.Log.Error(err, "failed to mark config error task as failed", "podUID", pod.UID, "name", pod.Name)
	}

	// remove finalizer as no logs will be collected
	if err := t.deletePod(ctx, pod, true); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			log.Log.Error(err, "failed to delete pod with config error", "podUId", pod.UID)
		}
	}

	webhook, err := t.dbManager.GetWebhookDetails(ctx, dagRunId)
	if err != nil {
		log.Log.Error(err, "failed to get webhook details", "runId", dagRunId)
	} else if webhook.URL != "" {
		go t.webhookNotifier.NotifyTaskRun(pod.Spec.Containers[0].Name, "failed", dagRunId, taskRunId, webhook.URL, webhook.VerifySSL)
	}

	// Handle downstream tasks
	t.handleUnretryablePod(ctx, pod, taskRunId, dagRunId, -2) // -2 for config error
}

func (t *worker) deletePVC(ctx context.Context, pod *v1.Pod) error {
	for _, volumes := range pod.Spec.Volumes {
		if volumes.PersistentVolumeClaim != nil && volumes.Name == "workspace" {
			// Fetch the PVC
			pvc, err := t.clientSet.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(ctx, volumes.PersistentVolumeClaim.ClaimName, metav1.GetOptions{})
			if err != nil {
				return err
			}

			// Remove finalizers
			pvc.Finalizers = []string{}

			// Update the PVC
			_, err = t.clientSet.CoreV1().PersistentVolumeClaims(pod.Namespace).Update(ctx, pvc, metav1.UpdateOptions{})
			if err != nil {
				return err
			}

			return t.clientSet.CoreV1().PersistentVolumeClaims(pod.Namespace).Delete(ctx, volumes.PersistentVolumeClaim.ClaimName, metav1.DeleteOptions{})
		}
	}

	return nil
}

func (t *worker) checkIfDagRunIsComplete(ctx context.Context, runId int) (bool, error) {
	// check if all tasks are done
	allTasksDone, err := t.dbManager.CheckIfAllTasksDone(ctx, runId)
	if err != nil {
		log.Log.Error(err, "failed to check if all tasks are done", "runId", runId)
		return false, err
	}

	return allTasksDone, nil
}

func (t *worker) getTaskRunID(pod *v1.Pod) (int, error) {
	taskRunIdStr, ok := pod.Annotations[kontrolerTaskRunID]
	if !ok {
		return 0, fmt.Errorf("%w: %s", ErrMissingAnnotation, kontrolerTaskRunID)
	}

	taskRunId, err := strconv.Atoi(taskRunIdStr)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrInvalidTaskRunID, taskRunIdStr)
	}

	return taskRunId, nil
}

func (t *worker) getDagRunID(pod *v1.Pod) (int, error) {
	dagRunStr, ok := pod.Annotations["kontroler/dagRun-id"]
	if !ok {
		return 0, fmt.Errorf("%w: kontroler/dagRun-id", ErrMissingAnnotation)
	}

	return strconv.Atoi(dagRunStr)
}

func (w *worker) handleLogCollection(ctx context.Context, pod *v1.Pod) {
	// log collection
	readyForLogCollection := false
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Running != nil || containerStatus.State.Terminated != nil {
			readyForLogCollection = true
			break
		}
	}

	if w.logStore != nil && readyForLogCollection {
		// Attempt to get logs, but we don't stop if we can't get them
		go func() {
			dagRunId, err := w.getDagRunID(pod)
			if err != nil {
				log.Log.Error(err, "failed to get dag run ID", "pod", pod.Name)
				return
			}

			if ok := w.logStore.IsFetching(dagRunId, pod); ok {
				log.Log.Info("already fetching", "podUID", pod.UID, "name", pod.Name)
				return
			}

			if err := w.logStore.MarkAsFetching(dagRunId, pod); err != nil {
				log.Log.Info("already fetching", "podUID", pod.UID, "name", pod.Name)
				return
			}

			defer w.logStore.UnlistFetching(dagRunId, pod)

			log.Log.Info("started collecting logs", "pod", pod.Name)
			if err := w.logStore.UploadLogs(context.Background(), dagRunId, w.clientSet, pod); err != nil {
				log.Log.Error(err, "failed to uploadLogs")
			}
		}()
	}
}

func hasCreateContainerConfigError(pod *v1.Pod) bool {
	for _, status := range pod.Status.ContainerStatuses {
		if status.State.Waiting != nil {
			fmt.Println("Waiting reason:", status.State.Waiting.Reason)
		}

		if status.State.Waiting != nil && status.State.Waiting.Reason == "CreateContainerConfigError" {
			return true
		}
	}
	// Also check init containers
	for _, status := range pod.Status.InitContainerStatuses {
		if status.State.Waiting != nil && status.State.Waiting.Reason == "CreateContainerConfigError" {
			return true
		}
	}
	return false
}
