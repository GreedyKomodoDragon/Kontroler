package dag

import (
	"kontroler-controller/internal/workers"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type EventWatcher interface {
	StartWatching(stopCh <-chan struct{})
}

type eventWatcher struct {
	informer cache.SharedIndexInformer
}

func NewEventWatcher(id, namespace string, clientSet *kubernetes.Clientset, resourceEventHandler workers.ResourceEventHandler) (EventWatcher, error) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		clientSet,
		30*time.Second,
		informers.WithNamespace(namespace),
	)

	informer := factory.Core().V1().Events().Informer()

	watcher := &eventWatcher{
		informer: informer,
	}

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    resourceEventHandler.HandleAdd,
		UpdateFunc: resourceEventHandler.HandleUpdate,
		DeleteFunc: resourceEventHandler.HandleDelete,
	})

	return watcher, nil
}

func (e *eventWatcher) StartWatching(stopCh <-chan struct{}) {
	e.informer.Run(stopCh)
}
