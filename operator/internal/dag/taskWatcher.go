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

		return
	}

	if job.Status.Failed > 0 {
		log.Log.Info("task failed", "jobUid", job.UID, "taskRunId", taskRunId)

		if err := t.dbManager.MarkOutcomeAsFailed(ctx, taskRunId); err != nil {
			log.Log.Error(err, "failed to mark outcome as failed", "jobUid", job.UID, "event", "add/update")
		}
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
