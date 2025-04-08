package dag

import (
	"errors"
	"kontroler-controller/internal/workers"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
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
	informer cache.SharedIndexInformer
}

func NewTaskWatcher(id, namespace string, clientSet *kubernetes.Clientset, resourceEventHandler workers.ResourceEventHandler) (TaskWatcher, error) {
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
		informer: informer,
	}

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    resourceEventHandler.HandleAdd,
		UpdateFunc: resourceEventHandler.HandleUpdate,
		DeleteFunc: resourceEventHandler.HandleDelete,
	})

	return watcher, nil
}

func (t *taskWatcher) StartWatching(stopCh <-chan struct{}) {
	t.informer.Run(stopCh)
}
