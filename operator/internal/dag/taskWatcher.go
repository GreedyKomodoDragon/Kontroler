package dag

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/GreedyKomodoDragon/KubeConductor/operator/internal/db"
	batchv1 "k8s.io/api/batch/v1"
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

// Purpose of TaskWatcher is to listen for pods within a job to finish and record results/trigger the next pods
type TaskWatcher interface {
	StartWatching()
}

type taskWatcher struct {
	dbManager     db.DBDAGManager
	informer      cache.SharedIndexInformer
	clientSet     *kubernetes.Clientset
	taskAllocator TaskAllocator
	startTime     time.Time
}

func NewTaskWatcher(clientSet *kubernetes.Clientset, taskAllocator TaskAllocator, dbManager db.DBDAGManager) (TaskWatcher, error) {
	// Define label selector
	labelSelector := labels.Set(map[string]string{
		"managed-by":         "kubeconductor",
		"kubeconductor/type": "task",
	}).AsSelector().String()

	// Create a factory that watches all namespaces
	factory := informers.NewSharedInformerFactoryWithOptions(clientSet, 0, informers.WithTweakListOptions(func(options *metav1.ListOptions) {
		options.LabelSelector = labelSelector
	}))

	// Create an informer that watches jobs with the specified label selector
	informer := factory.Batch().V1().Jobs().Informer()

	watcher := &taskWatcher{
		dbManager:     dbManager,
		informer:      informer,
		clientSet:     clientSet,
		taskAllocator: taskAllocator,
		startTime:     time.Now(), // Track the start time of the watcher
	}

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    watcher.handleJobAddOrUpdate,
		UpdateFunc: func(oldObj, newObj interface{}) { watcher.handleJobAddOrUpdate(newObj) },
		DeleteFunc: watcher.handleJobDelete,
	})

	return watcher, nil
}

func (t *taskWatcher) StartWatching() {
	stopCh := make(chan struct{})
	defer close(stopCh)
	t.informer.Run(stopCh)
}

func (t *taskWatcher) handleJobAddOrUpdate(obj interface{}) {
	job, ok := obj.(*batchv1.Job)
	if !ok {
		log.Log.Error(fmt.Errorf("invalid object"), "failed to parse job object")
		return
	}

	// Filter out jobs created before the watcher started
	if job.CreationTimestamp.Time.Before(t.startTime) {
		return
	}

	ctx := context.Background()
	log.Log.Info("job event", "jobUid", job.UID, "event", "add/update")

	taskRunIdStr, ok := job.Annotations["kubeconductor/task-rid"]
	if !ok {
		log.Log.Error(fmt.Errorf("missing annotation"), "annotation", "kubeconductor/task-rid", "job", job.Name)
		return
	}

	taskRunId, err := strconv.Atoi(taskRunIdStr)
	if err != nil {
		log.Log.Error(err, "failed to convert task run string", "taskRunIdStr", taskRunIdStr)
		return
	}

	if job.Status.Succeeded > 0 {
		t.handleSuccessfulJob(ctx, job, taskRunId)
	} else if job.Status.Failed > 0 {
		t.handleFailedJob(ctx, job, taskRunId)
	}
}

func (t *taskWatcher) handleJobDelete(obj interface{}) {
	job, ok := obj.(*batchv1.Job)
	if !ok {
		log.Log.Error(fmt.Errorf("invalid object"), "failed to parse job object")
		return
	}

	log.Log.Info("job was deleted", "jobUid", job.UID)
}

func (t *taskWatcher) handleSuccessfulJob(ctx context.Context, job *batchv1.Job, taskRunId int) {
	log.Log.Info("task succeeded", "jobUid", job.UID, "taskRunId", taskRunId)

	// Get the run ID from the job
	dagRunIdStr, ok := job.Annotations["kubeconductor/dagRun-id"]
	if !ok {
		log.Log.Error(fmt.Errorf("missing annotation"), "annotation", "kubeconductor/dagRun-id", "job", job.Name)
		return
	}

	runId, err := strconv.Atoi(dagRunIdStr)
	if err != nil {
		log.Log.Error(err, "failed to get dag run", "dagRun", dagRunIdStr)
		return
	}

	// Mark this as success in db
	tasks, err := t.dbManager.MarkSuccessAndGetNextTasks(ctx, taskRunId)
	if err != nil {
		log.Log.Error(err, "failed to mark outcome and get next task", "jobUid", job.UID, "event", "add/update")
		return
	}

	log.Log.Info("number of tasks", "tasks", len(tasks))

	for _, task := range tasks {
		taskRunId, err := t.dbManager.MarkTaskAsStarted(ctx, runId, task.Id)
		if err != nil {
			log.Log.Error(err, "failed to mark task as started", "dagRun_id", runId, "task_id", task.Id)
			continue
		}

		job, err := t.taskAllocator.AllocateTask(ctx, task, runId, taskRunId, job.Namespace)
		if err != nil {
			log.Log.Error(err, "failed to allocate task", "task.Id", task.Id, "task.Name", task.Name)
			continue
		}

		log.Log.Info("allocated task", "jobId", job, "task.Id", task.Id, "task.Name", task.Name)
	}
}

func (t *taskWatcher) handleFailedJob(ctx context.Context, job *batchv1.Job, taskRunId int) {
	log.Log.Info("task failed", "jobUid", job.UID, "taskRunId", taskRunId)

	// Extract failure details
	pods, err := t.clientSet.CoreV1().Pods(job.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", job.Name),
	})

	if err != nil {
		log.Log.Error(err, "failed to list pods for job", "job", job.Name)
		return
	}

	log.Log.Info("number of pods in job", "jobUid", job.UID, "count", len(pods.Items))

	// Get Exitable codes to restart with and backoff limit
	for _, pod := range pods.Items {

		dagRunStr, ok := job.Annotations["kubeconductor/dagRun-id"]
		if !ok {
			log.Log.Error(err, "found pod missing kubeconductor/dagRun-id", "job", job.Name)
			continue
		}

		if len(pod.Status.ContainerStatuses) == 0 {
			log.Log.Error(err, "does not seem to be any containers?", "job", job.Name)
			continue
		}

		if pod.Status.ContainerStatuses[0].State.Terminated == nil {
			log.Log.Error(err, "missing status informationt", "job", job.Name)
			continue
		}

		ok, err := t.dbManager.ShouldRerun(ctx, taskRunId, pod.Status.ContainerStatuses[0].State.Terminated.ExitCode)
		if err != nil {
			log.Log.Error(err, "failed to determine if pod should be re-ran", "job", job.Name)
			continue
		}

		if !ok {
			log.Log.Info("job has reached it max backoffLimit or exit code not recoverable", "jobUid", job.UID, "exitCode", pod.Status.ContainerStatuses[0].State.Terminated.ExitCode)

			if err := t.dbManager.MarkTaskAsFailed(ctx, taskRunId); err != nil {
				log.Log.Error(err, "failed to mark task as failed", "jobUid", job.UID, "event", "add/update")
			}

			continue
		}

		dagRunId, err := strconv.Atoi(dagRunStr)
		if err != nil {
			log.Log.Error(err, "failed to parse dagRunStr", "dagRunStr", dagRunStr)
			continue
		}

		container := pod.Spec.Containers[0]
		// Re-use envs instead of going to database again
		taskId, err := t.taskAllocator.AllocateTaskWithEnv(ctx,
			db.Task{
				Name:    container.Name,
				Args:    container.Args,
				Command: container.Command,
				Image:   container.Image,
			}, dagRunId, taskRunId, pod.Namespace, container.Env)

		if err != nil {
			log.Log.Error(err, "failed to allocate new pod")
			continue
		}

		if err := t.dbManager.IncrementAttempts(ctx, taskRunId); err != nil {
			log.Log.Error(err, "failed to increment attempts", "taskRunId", taskRunId)
		}

		log.Log.Info("new task allocated allocated", "taskId", taskId)

	}
}
