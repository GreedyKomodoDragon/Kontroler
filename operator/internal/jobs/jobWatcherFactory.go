package jobs

import (
	"context"

	"github.com/GreedyKomodoDragon/KubeConductor/operator/internal/db"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

type JobWatcherFactory interface {
	StartWatcher(namespace string) error
	IsWatching(namespace string) bool
}

type jobWatcherFactory struct {
	clientSet    *kubernetes.Clientset
	watcherMap   map[string]JobWatcher
	dbManager    db.DBSchedulerManager
	jobAllocator JobAllocator
}

func NewJobWatcherFactory(clientSet *kubernetes.Clientset, jobAllocator JobAllocator, dbManager db.DBSchedulerManager) JobWatcherFactory {
	return &jobWatcherFactory{
		clientSet:    clientSet,
		watcherMap:   map[string]JobWatcher{},
		jobAllocator: jobAllocator,
		dbManager:    dbManager,
	}
}

func (p *jobWatcherFactory) StartWatcher(namespace string) error {
	labelSelector := labels.Set(map[string]string{
		"managed-by": "kubeconductor",
	}).AsSelector()

	// Set up job watcher
	watcher, err := p.clientSet.BatchV1().Jobs(namespace).Watch(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	})
	if err != nil {
		log.Log.Error(err, "failed to watch jobs", "namespace", namespace)
		return err
	}

	newJobWatcher := NewJobWatcher(namespace, p.clientSet, watcher, p.jobAllocator, p.dbManager)
	p.watcherMap[namespace] = newJobWatcher

	go newJobWatcher.StartWatching()

	return nil
}

func (p *jobWatcherFactory) IsWatching(namespace string) bool {
	_, ok := p.watcherMap[namespace]
	return ok
}
