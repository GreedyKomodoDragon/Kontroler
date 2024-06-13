package dag

import (
	"fmt"

	"github.com/GreedyKomodoDragon/KubeConductor/operator/internal/db"
	batchv1 "k8s.io/api/batch/v1"
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
	dbManager   db.DBDAGManager
	kubeWatcher watch.Interface
	clientSet   *kubernetes.Clientset
	namespace   string
}

func NewJobWatcher(clientSet *kubernetes.Clientset, kubeWatcher watch.Interface, dbManager db.DBDAGManager) TaskWatcher {
	return &taskWatcher{
		dbManager:   dbManager,
		kubeWatcher: kubeWatcher,
		clientSet:   clientSet,
	}
}

func (t *taskWatcher) StartWatching() {
	// Handle job events
	for event := range t.kubeWatcher.ResultChan() {
		job, ok := event.Object.(*batchv1.Job)
		if !ok {
			log.Log.Error(fmt.Errorf("invalid object"), "failed to parse job object")
			continue
		}

		switch event.Type {
		case watch.Added, watch.Modified:
			log.Log.Info("job event", "jobUid", job.UID, "event.type", event.Type)

			if job.Status.Succeeded > 0 {
				// Mark this as success in db

				// Check if any more task to execute now
				continue
			}

			if job.Status.Failed > 0 {
				// TODO: Come back and add in conditional check here
			}
		case watch.Deleted:
			// Job was deleted, do something we this in the future
			log.Log.Info("jobs was deleted", "jobUid", job.UID)
		}
	}
}
