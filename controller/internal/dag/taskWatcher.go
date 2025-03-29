package dag

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
	"kontroler-controller/internal/webhook"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	ErrMissingAnnotation = errors.New("missing required annotation")
	ErrInvalidTaskRunID  = errors.New("invalid task run ID")
	kontrolerTaskRunID   = "kontroler/task-rid"
)

// Purpose of TaskWatcher is to listen for pods to finish and record results/trigger the next pods
// Will also allocate new pods
type TaskWatcher interface {
	StartWatching(stopCh <-chan struct{})
}

type taskWatcher struct {
	namespace       string
	dbManager       db.DBDAGManager
	informer        cache.SharedIndexInformer
	clientSet       *kubernetes.Clientset
	taskAllocator   TaskAllocator
	logStore        object.LogStore
	webhookNotifier webhook.WebhookNotifier
}

func NewTaskWatcher(namespace string, clientSet *kubernetes.Clientset, taskAllocator TaskAllocator, dbManager db.DBDAGManager, id string, logStore object.LogStore, webhookChan chan webhook.WebhookPayload) (TaskWatcher, error) {
	labelSelector := labels.Set(map[string]string{
		"managed-by":     "kontroler",
		"kontroler/type": "task",
		"kontroler/id":   id,
	}).AsSelector().String()

	factory := informers.NewSharedInformerFactoryWithOptions(
		clientSet,
		30*time.Second,
		informers.WithNamespace(namespace),
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = labelSelector
		}),
	)

	// Create an informer that watches pods with the specified label selector
	informer := factory.Core().V1().Pods().Informer()

	watcher := &taskWatcher{
		namespace:       namespace,
		dbManager:       dbManager,
		informer:        informer,
		clientSet:       clientSet,
		taskAllocator:   taskAllocator,
		logStore:        logStore,
		webhookNotifier: webhook.NewWebhookNotifier(webhookChan),
	}

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    watcher.handleAdd,
		UpdateFunc: func(oldObj, newObj interface{}) { watcher.handleUpdate(oldObj, newObj) },
		DeleteFunc: watcher.handlePodDelete,
	})

	return watcher, nil
}

func (t *taskWatcher) StartWatching(stopCh <-chan struct{}) {
	t.informer.Run(stopCh)
}

func (t *taskWatcher) handleAdd(obj interface{}) {
	eventTime := time.Now()

	pod, ok := obj.(*v1.Pod)
	if !ok {
		log.Log.Error(fmt.Errorf("invalid object"), "failed to parse pod object")
		return
	}

	t.handleOutcome(pod, "add", eventTime)

	if err := t.writeStatusToDB(pod, eventTime); err != nil {
		log.Log.Error(err, "failed to writeStatusToDB")
	}
}

func (t *taskWatcher) handleUpdate(old, obj interface{}) {
	eventTime := time.Now()
	oldPod, ok := old.(*v1.Pod)
	if !ok {
		log.Log.Error(fmt.Errorf("invalid object"), "failed to parse old pod object in handleUpdate")
		return
	}

	pod, ok := obj.(*v1.Pod)
	if !ok {
		log.Log.Error(fmt.Errorf("invalid object"), "failed to parse pod object")
		return
	}

	if oldPod.Status.Phase == pod.Status.Phase {
		return
	}

	t.handleOutcome(pod, "update", eventTime)

	log.Log.Info("pod event", "podUID", pod.UID, "name", pod.Name, "event", "update", "status", pod.Status.Phase)

	if err := t.writeStatusToDB(pod, eventTime); err != nil {
		log.Log.Error(err, "failed to writeStatusToDB")
	}
}

func (t *taskWatcher) handleOutcome(pod *v1.Pod, event string, eventTime time.Time) {
	ctx := context.Background()
	log.Log.Info("pod event", "podUID", pod.UID, "name", pod.Name, "event", event, "eventTime", eventTime)

	taskRunId, err := t.getTaskRunID(pod)
	if err != nil {
		log.Log.Error(err, "failed to get task run ID", "pod", pod.Name)
		return
	}

	// log collection
	readyForLogCollection := false
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Running != nil || containerStatus.State.Terminated != nil {
			readyForLogCollection = true
			break
		}
	}

	if t.logStore != nil && readyForLogCollection {
		// Attempt to get logs, but we don't stop if we can't get them
		go func() {
			dagRunId, err := t.getDagRunID(pod)
			if err != nil {
				log.Log.Error(err, "failed to get dag run ID", "pod", pod.Name)
				return
			}

			if ok := t.logStore.IsFetching(dagRunId, pod); ok {
				log.Log.Info("already fetching", "podUID", pod.UID, "name", pod.Name, "event", event)
				return
			}

			if err := t.logStore.MarkAsFetching(dagRunId, pod); err != nil {
				log.Log.Info("already fetching", "podUID", pod.UID, "name", pod.Name, "event", event)
				return
			}

			defer t.logStore.UnlistFetching(dagRunId, pod)

			log.Log.Info("started collecting logs", "pod", pod.Name)
			if err := t.logStore.UploadLogs(context.Background(), dagRunId, t.clientSet, pod); err != nil {
				log.Log.Error(err, "failed to uploadLogs")
			}
		}()
	}

	switch pod.Status.Phase {
	case v1.PodSucceeded:
		t.handleSuccessfulTaskRun(ctx, pod, taskRunId)
	case v1.PodFailed:
		t.handleFailedTaskRun(ctx, pod, taskRunId)
	case v1.PodRunning:
		t.handleStartedTaskRun(ctx, pod, taskRunId)
	case v1.PodPending:
		t.handlePendingTaskRun(ctx, pod, taskRunId)
	case v1.PodUnknown:
		log.Log.Info("pod status unknown", "podUID", pod.UID, "name", pod.Name, "event", event)
	}
}

func (t *taskWatcher) handlePodDelete(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		log.Log.Error(fmt.Errorf("invalid object"), "failed to parse pod object")
		return
	}

	log.Log.Info("pod was deleted", "podUid", pod.UID)
}

func (t *taskWatcher) handleSuccessfulTaskRun(ctx context.Context, pod *v1.Pod, taskRunId int) {
	log.Log.Info("task succeeded", "podUID", pod.UID, "name", pod.Name, "taskRunId", taskRunId)

	dagRunId, err := t.getDagRunID(pod)
	if err != nil {
		log.Log.Error(err, "failed to get dag run ID", "pod", pod.Name)
		return
	}

	if err := t.deletePod(ctx, pod); err != nil {
		log.Log.Info("pod has already been deleted/handled, skipping", "podUId", pod.UID)
		return
	}

	tasks, err := t.dbManager.MarkSuccessAndGetNextTasks(ctx, taskRunId)
	if err != nil {
		log.Log.Error(err, "failed to mark outcome and get next task", "podUID", pod.UID, "name", pod.Name, "event", "add/update")
		return
	}

	webhook, err := t.dbManager.GetWebhookDetails(ctx, dagRunId)
	if err != nil {
		log.Log.Error(err, "failed to get webhook details", "runId", dagRunId)
	} else if webhook.URL != "" {
		go t.webhookNotifier.NotifyTaskRun(pod.Spec.Containers[0].Name, "success", dagRunId, taskRunId, webhook.URL, webhook.VerifySSL)
	}

	log.Log.Info("number of tasks", "tasks", len(tasks))

	if len(tasks) == 0 {
		complete, err := t.checkIfDagRunIsComplete(ctx, dagRunId)
		if err != nil {
			log.Log.Error(err, "failed to check if dag run is complete", "runId", dagRunId)
			return
		}

		if !complete {
			return
		}

		if err := t.deletePVC(ctx, pod); err != nil {
			log.Log.Error(err, "failed to delete PVC", "pod", pod.Name, "namespace", pod.Namespace, "dagRunId", dagRunId, "status", pod.Status.Phase)
		}
		return
	}

	for _, task := range tasks {
		taskRunId, err := t.dbManager.MarkTaskAsStarted(ctx, dagRunId, task.Id)
		if err != nil {
			log.Log.Error(err, "failed to mark task as started", "dagRun_id", dagRunId, "task_id", task.Id)
			continue
		}

		newPod, err := t.taskAllocator.AllocateTask(ctx, task, dagRunId, taskRunId, pod.Namespace)
		if err != nil {
			log.Log.Error(err, "failed to allocate task", "task.Id", task.Id, "task.Name", task.Name)
			continue
		}

		log.Log.Info("allocated task", "newPodUID", newPod, "task.Id", task.Id, "task.Name", task.Name)
	}
}

func (t *taskWatcher) handleFailedTaskRun(ctx context.Context, pod *v1.Pod, taskRunId int) {
	log.Log.Info("task failed", "podUID", pod.UID, "name", pod.Name, "taskRunId", taskRunId, "exitcode", pod.Status.ContainerStatuses[0].State.Terminated.ExitCode)

	dagRunId, err := t.getDagRunID(pod)
	if err != nil {
		log.Log.Error(err, "failed to get dag run ID", "pod", pod.Name)
		return
	}

	if err := t.deletePod(ctx, pod); err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Log.Info("pod has already been deleted/handled, skipping", "podUId", pod.UID)
		} else {
			log.Log.Error(err, "failed to delete pod in failed,", "podUId", pod.UID)
		}
		return
	}

	ok, err := t.dbManager.ShouldRerun(ctx, taskRunId, pod.Status.ContainerStatuses[0].State.Terminated.ExitCode)
	if err != nil {
		log.Log.Error(err, "failed to determine if pod should be re-ran", "pod", pod.Name)
		return
	}

	container := pod.Spec.Containers[0]

	if !ok {
		t.handleUnretryablePod(ctx, pod, taskRunId, dagRunId)
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

	dbTask := db.Task{
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

func (t *taskWatcher) writeStatusToDB(pod *v1.Pod, stamp time.Time) error {
	taskRunId, err := t.getTaskRunID(pod)
	if err != nil {
		return err
	}

	dagRunId, err := t.getDagRunID(pod)
	if err != nil {
		return err
	}

	var exitCode *int32 = nil
	var duration int64 = 0
	if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].State.Terminated != nil {
		exitCode = &pod.Status.ContainerStatuses[0].State.Terminated.ExitCode

		// Use the termination time for final pod status
		stamp = pod.Status.ContainerStatuses[0].State.Terminated.FinishedAt.Time
		startTime := pod.Status.ContainerStatuses[0].State.Terminated.StartedAt.Time
		duration = int64(stamp.Sub(startTime).Seconds())

		if err := t.dbManager.AddPodDuration(context.Background(), taskRunId, duration); err != nil {
			log.Log.Error(err, "failed to add pod duration", "podUID", pod.UID, "name", pod.Name, "taskRunId", taskRunId)
		}
	}

	if err := t.dbManager.MarkPodStatus(context.Background(), pod.UID, pod.Name,
		taskRunId, pod.Status.Phase, stamp, exitCode, pod.Namespace); err != nil {
		log.Log.Error(err, "failed to mark pod status", "podUID", pod.UID, "name", pod.Name, "taskRunId", taskRunId, "status", pod.Status.Phase)
		return fmt.Errorf("failed to mark pod status: %w", err)
	}

	webhook, err := t.dbManager.GetWebhookDetails(context.Background(), dagRunId)
	if err != nil {
		log.Log.Error(err, "failed to get webhook details", "runId", dagRunId)
	} else if webhook.URL != "" {
		go t.webhookNotifier.NotifyPodEvent(
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

func (t *taskWatcher) handleUnretryablePod(ctx context.Context, pod *v1.Pod, taskRunId, dagRunId int) {
	log.Log.Info("pod has reached it max backoffLimit or exit code not recoverable", "podUID", pod.UID, "name", pod.Name, "exitCode", pod.Status.ContainerStatuses[0].State.Terminated.ExitCode)

	if err := t.dbManager.MarkTaskAsFailed(ctx, taskRunId); err != nil {
		log.Log.Error(err, "failed to mark task as failed", "podUID", pod.UID, "name", pod.Name, "event", "add/update")
	}

	webhook, err := t.dbManager.GetWebhookDetails(ctx, dagRunId)
	if err != nil {
		log.Log.Error(err, "failed to get webhook details", "runId", dagRunId)
	} else if webhook.URL != "" {
		go t.webhookNotifier.NotifyTaskRun(pod.Spec.Containers[0].Name, "failed", dagRunId, taskRunId, webhook.URL, webhook.VerifySSL)
	}

	taskNames, err := t.dbManager.MarkConnectingTasksAsSuspended(ctx, dagRunId, taskRunId)
	if err == nil {
		if webhook.URL != "" {
			for _, taskName := range taskNames {
				log.Log.Info("task marked as suspended", "taskName", taskName)
				go t.webhookNotifier.NotifyTaskRun(taskName, "suspended", dagRunId, taskRunId, webhook.URL, webhook.VerifySSL)
			}
		}
	} else {
		log.Log.Error(err, "failed to mark connecting tasks as suspended", "taskRunId", taskRunId)
		return
	}

	complete, err := t.checkIfDagRunIsComplete(ctx, dagRunId)
	if err != nil {
		log.Log.Error(err, "failed to check if dag run is complete", "runId", dagRunId)
		return
	}

	if !complete {
		return
	}

	if err := t.deletePVC(ctx, pod); err != nil {
		log.Log.Error(err, "failed to delete PVC", "pod", pod.Name, "namespace", pod.Namespace, "dagRunId", dagRunId, "status", pod.Status.Phase)
	}
}

func (t *taskWatcher) deletePod(ctx context.Context, pod *v1.Pod) error {
	backgroundDeletion := metav1.DeletePropagationBackground
	return t.clientSet.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{
		PropagationPolicy: &backgroundDeletion,
	})
}

func (t *taskWatcher) handleStartedTaskRun(ctx context.Context, pod *v1.Pod, taskRunId int) {
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

func (t *taskWatcher) handlePendingTaskRun(ctx context.Context, pod *v1.Pod, taskRunId int) {
	log.Log.Info("task pending", "podUID", pod.UID, "name", pod.Name, "taskRunId", taskRunId)

	dagRunId, err := t.getDagRunID(pod)
	if err != nil {
		log.Log.Error(err, "failed to get dag run ID", "pod", pod.Name)
		return
	}

	webhook, err := t.dbManager.GetWebhookDetails(ctx, dagRunId)
	if err != nil {
		log.Log.Error(err, "failed to get webhook details", "runId", dagRunId)
	} else if webhook.URL != "" {
		go t.webhookNotifier.NotifyTaskRun(pod.Spec.Containers[0].Name, "pending", dagRunId, taskRunId, webhook.URL, webhook.VerifySSL)
	}
}

func (t *taskWatcher) deletePVC(ctx context.Context, pod *v1.Pod) error {
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

func (t *taskWatcher) checkIfDagRunIsComplete(ctx context.Context, runId int) (bool, error) {
	// check if all tasks are done
	allTasksDone, err := t.dbManager.CheckIfAllTasksDone(ctx, runId)
	if err != nil {
		log.Log.Error(err, "failed to check if all tasks are done", "runId", runId)
		return false, err
	}

	return allTasksDone, nil
}

func (t *taskWatcher) getTaskRunID(pod *v1.Pod) (int, error) {
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

func (t *taskWatcher) getDagRunID(pod *v1.Pod) (int, error) {
	dagRunStr, ok := pod.Annotations["kontroler/dagRun-id"]
	if !ok {
		return 0, fmt.Errorf("%w: kontroler/dagRun-id", ErrMissingAnnotation)
	}

	return strconv.Atoi(dagRunStr)
}
