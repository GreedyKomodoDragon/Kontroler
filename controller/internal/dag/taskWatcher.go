package dag

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/GreedyKomodoDragon/Kontroler/operator/api/v1alpha1"
	"github.com/GreedyKomodoDragon/Kontroler/operator/internal/db"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ALL_NAMESPACES string = ""
)

// Purpose of TaskWatcher is to listen for pods to finish and record results/trigger the next pods
// Will also allocate new pods
type TaskWatcher interface {
	StartWatching(stopCh <-chan struct{})
}

type taskWatcher struct {
	namespace     string
	dbManager     db.DBDAGManager
	informer      cache.SharedIndexInformer
	clientSet     *kubernetes.Clientset
	taskAllocator TaskAllocator
	lock          *sync.Mutex
}

func NewTaskWatcher(namespace string, clientSet *kubernetes.Clientset, taskAllocator TaskAllocator, dbManager db.DBDAGManager, id string) (TaskWatcher, error) {
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
		namespace:     namespace,
		dbManager:     dbManager,
		informer:      informer,
		clientSet:     clientSet,
		taskAllocator: taskAllocator,
		lock:          &sync.Mutex{},
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

	taskRunIdStr, ok := pod.Annotations["kontroler/task-rid"]
	if !ok {
		log.Log.Error(fmt.Errorf("missing annotation"), "annotation", "kontroler/task-rid", "pod", pod.Name)
		return
	}

	taskRunId, err := strconv.Atoi(taskRunIdStr)
	if err != nil {
		log.Log.Error(err, "failed to convert task run string", "taskRunIdStr", taskRunIdStr)
		return
	}

	// Ensure container status exists
	if len(pod.Status.ContainerStatuses) == 0 {
		log.Log.Info("no container status available", "podUID", pod.UID, "name", pod.Name, "event", event)
		return
	}

	containerStatus := pod.Status.ContainerStatuses[0]
	state := containerStatus.State

	// Check if container is still running or waiting
	if state.Running != nil {
		log.Log.Info("pod is still running", "podUID", pod.UID, "name", pod.Name, "event", event, "startedAt", state.Running.StartedAt)
		return
	}

	if state.Waiting != nil {
		log.Log.Info("pod is waiting", "podUID", pod.UID, "name", pod.Name, "event", event, "reason", state.Waiting.Reason, "message", state.Waiting.Message)
		return
	}

	// Check if the container has terminated
	if state.Terminated == nil {
		return
	}

	terminatedState := state.Terminated
	log.Log.Info("pod terminated", "podUID", pod.UID, "name", pod.Name, "event", event, "exitCode", terminatedState.ExitCode, "reason", terminatedState.Reason, "message", terminatedState.Message, "finishedAt", terminatedState.FinishedAt)

	// Log specific success/failure
	if terminatedState.ExitCode == 0 {
		log.Log.Info("pod completed successfully", "podUID", pod.UID, "name", pod.Name, "event", event, "exitCode", terminatedState.ExitCode)
		t.handleSuccessfulTaskRun(ctx, pod, taskRunId)
		return
	}

	log.Log.Info("pod failed", "podUID", pod.UID, "name", pod.Name, "exitCode", terminatedState.ExitCode, "reason", terminatedState.Reason)
	t.handleFailedTaskRun(ctx, pod, taskRunId)
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

	// Get the run ID from the Pod
	dagRunIdStr, ok := pod.Annotations["kontroler/dagRun-id"]
	if !ok {
		log.Log.Error(fmt.Errorf("missing annotation"), "annotation", "kontroler/dagRun-id", "pod", pod.Name)
		return
	}

	runId, err := strconv.Atoi(dagRunIdStr)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Log.Info("pod has already been deleted/handled, skipping", "podUId", pod.UID)
		} else {
			log.Log.Error(err, "failed to delete pod in successful,", "podUId", pod.UID)
		}
		return
	}

	if err := t.deletePod(ctx, pod); err != nil {
		log.Log.Info("pod has already been deleted/handled, skipping", "podUId", pod.UID)
		return
	}

	// Mark this as success in db - Have to use lock to avoid getting already started task
	t.lock.Lock()
	tasks, err := t.dbManager.MarkSuccessAndGetNextTasks(ctx, taskRunId)
	if err != nil {
		log.Log.Error(err, "failed to mark outcome and get next task", "podUID", pod.UID, "name", pod.Name, "event", "add/update")
		t.lock.Unlock()
		return
	}
	t.lock.Unlock()

	log.Log.Info("number of tasks", "tasks", len(tasks))

	// TODO: Using a channel + Goroutines Workers for scaling out pods quicker
	for _, task := range tasks {
		taskRunId, err := t.dbManager.MarkTaskAsStarted(ctx, runId, task.Id)
		if err != nil {
			log.Log.Error(err, "failed to mark task as started", "dagRun_id", runId, "task_id", task.Id)
			continue
		}

		newPod, err := t.taskAllocator.AllocateTask(ctx, task, runId, taskRunId, pod.Namespace)
		if err != nil {
			log.Log.Error(err, "failed to allocate task", "task.Id", task.Id, "task.Name", task.Name)
			continue
		}

		log.Log.Info("allocated task", "newPodUID", newPod, "task.Id", task.Id, "task.Name", task.Name)
	}
}

func (t *taskWatcher) handleFailedTaskRun(ctx context.Context, pod *v1.Pod, taskRunId int) {
	log.Log.Info("task failed", "podUID", pod.UID, "name", pod.Name, "taskRunId", taskRunId, "exitcode", pod.Status.ContainerStatuses[0].State.Terminated.ExitCode)

	dagRunStr, ok := pod.Annotations["kontroler/dagRun-id"]
	if !ok {
		log.Log.Error(fmt.Errorf("find to find annotation"), "found pod missing kontroler/dagRun-id", "pod", pod.Name)
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

	if !ok {
		log.Log.Info("pod has reached it max backoffLimit or exit code not recoverable", "podUID", pod.UID, "name", pod.Name, "exitCode", pod.Status.ContainerStatuses[0].State.Terminated.ExitCode)

		if err := t.dbManager.MarkTaskAsFailed(ctx, taskRunId); err != nil {
			log.Log.Error(err, "failed to mark task as failed", "podUID", pod.UID, "name", pod.Name, "event", "add/update")
		}

		return
	}

	dagRunId, err := strconv.Atoi(dagRunStr)
	if err != nil {
		log.Log.Error(err, "failed to parse dagRunStr", "dagRunStr", dagRunStr)
		return
	}

	container := pod.Spec.Containers[0]
	// Re-use envs instead of going to database again
	taskId, err := t.taskAllocator.AllocateTaskWithEnv(ctx,
		db.Task{
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
			},
		}, dagRunId, taskRunId, pod.Namespace, container.Env, &container.Resources)

	if err != nil {
		log.Log.Error(err, "failed to allocate new pod")
		return
	}

	if err := t.dbManager.IncrementAttempts(ctx, taskRunId); err != nil {
		log.Log.Error(err, "failed to increment attempts", "taskRunId", taskRunId)
	}

	log.Log.Info("new task allocated allocated with env", "taskId", taskId)

}

func (t *taskWatcher) writeStatusToDB(pod *v1.Pod, stamp time.Time) error {
	taskRunIDStr, ok := pod.Annotations["kontroler/task-rid"]
	if !ok {
		return fmt.Errorf("missing annotation kontroler/task-rid")
	}

	taskRunId, err := strconv.Atoi(taskRunIDStr)
	if err != nil {
		return fmt.Errorf("failed to convert task run string: %s", taskRunIDStr)
	}

	var exitCode *int32 = nil
	if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].State.Terminated != nil {
		exitCode = &pod.Status.ContainerStatuses[0].State.Terminated.ExitCode
	}

	if err := t.dbManager.MarkPodStatus(context.Background(), pod.UID, pod.Name, taskRunId, pod.Status.Phase, stamp, exitCode, pod.Namespace); err != nil {
		return err
	}

	return nil
}

func (t *taskWatcher) deletePod(ctx context.Context, pod *v1.Pod) error {
	backgroundDeletion := metav1.DeletePropagationBackground
	if err := t.clientSet.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{
		PropagationPolicy: &backgroundDeletion,
	}); err != nil {
		return err
	}

	return nil
}
