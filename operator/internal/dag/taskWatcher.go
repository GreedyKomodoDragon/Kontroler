package dag

import (
	"context"
	"fmt"
	"strconv"

	"github.com/GreedyKomodoDragon/KubeConductor/operator/internal/db"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
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
	kubeWatcher   watch.Interface
	clientSet     *kubernetes.Clientset
	taskAllocator TaskAllocator
}

func NewTaskWatcher(clientSet *kubernetes.Clientset, taskAllocator TaskAllocator, dbManager db.DBDAGManager) (TaskWatcher, error) {
	// Set up job watcher
	watcher, err := clientSet.BatchV1().Jobs(ALL_NAMESPACES).Watch(context.TODO(), metav1.ListOptions{
		LabelSelector: labels.Set(map[string]string{
			"managed-by":         "kubeconductor",
			"kubeconductor/type": "task",
		}).AsSelector().String(),
	})
	if err != nil {
		log.Log.Error(err, "failed to watch task")
		return nil, err
	}

	return &taskWatcher{
		dbManager:     dbManager,
		kubeWatcher:   watcher,
		clientSet:     clientSet,
		taskAllocator: taskAllocator,
	}, nil
}

func (t *taskWatcher) StartWatching() {
	// Handle job events
	for event := range t.kubeWatcher.ResultChan() {
		job, ok := event.Object.(*batchv1.Job)
		if !ok {
			log.Log.Error(fmt.Errorf("invalid object"), "failed to parse job object")
			continue
		}

		ctx := context.Background()

		switch event.Type {
		case watch.Added, watch.Modified:
			log.Log.Info("job event", "jobUid", job.UID, "event.type", event.Type)

			taskRunIdStr, ok := job.Annotations["kubeconductor/task-rid"]
			if !ok {
				log.Log.Error(fmt.Errorf("missing annotation"), "annotation", "kubeconductor/task-rid", "job", job.Name)
				continue
			}

			taskRunId, err := strconv.Atoi(taskRunIdStr)
			if err != nil {
				log.Log.Info("failed to get dag run", "dagRun", taskRunIdStr)
				continue
			}

			if job.Status.Succeeded > 0 {
				log.Log.Info("task succeeded", "jobUid", job.UID, "taskRunId", taskRunId)

				// Get the run ID from the job
				dagRunIdStr, ok := job.Annotations["kubeconductor/dagRun-id"]
				if !ok {
					log.Log.Error(fmt.Errorf("missing annotation"), "annotation", "kubeconductor/dagRun-id", "job", job.Name)
					continue
				}

				runId, err := strconv.Atoi(dagRunIdStr)
				if err != nil {
					log.Log.Info("failed to get dag run", "dagRun", dagRunIdStr)
					continue
				}

				// Mark this as success in db
				task, err := t.dbManager.MarkOutcomeAndGetNextTasks(ctx, taskRunId, "success")
				if err != nil {
					log.Log.Info("failed to mark outcome and get next task", "jobUid", job.UID, "event.type", event.Type)
					continue
				}

				for _, task := range task {
					taskRunId, err := t.dbManager.MarkTaskAsStarted(ctx, runId, task.Id)
					if err != nil {
						log.Log.Error(err, "failed to mask task as stated", "dagRun_id", runId, "task_id", task.Id)
						continue
					}

					if _, err := t.taskAllocator.AllocateTask(ctx, task, runId, taskRunId); err != nil {
						log.Log.Info("failed to allocate task", "task.Id", task.Id, "task.Name", task.Name)
					}
				}

				continue
			}

			if job.Status.Failed > 0 {
				log.Log.Info("task failed", "jobUid", job.UID, "taskRunId", taskRunId)

				if err := t.dbManager.MarkOutcome(ctx, taskRunId, "failed"); err != nil {
					log.Log.Info("failed to mark outcome as failed", "jobUid", job.UID, "event.type", event.Type)
				}
			}
		case watch.Deleted:
			// Job was deleted, do something we this in the future
			log.Log.Info("jobs was deleted", "jobUid", job.UID)
		}
	}
}
