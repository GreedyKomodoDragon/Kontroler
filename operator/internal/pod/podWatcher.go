package pod

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/GreedyKomodoDragon/KubeConductor/operator/internal/db"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

type PodWatcher interface {
	StartWatching()
}

type podWatcher struct {
	dbManager db.DBDAGManager
	informer  cache.SharedIndexInformer
	clientSet *kubernetes.Clientset
	startTime time.Time
}

func NewPodWatcher(clientSet *kubernetes.Clientset, dbManager db.DBDAGManager) (PodWatcher, error) {

	// Define label selector
	labelSelector := labels.Set(map[string]string{
		"managed-by":         "kubeconductor",
		"kubeconductor/type": "taskPod",
	}).AsSelector().String()

	// Create a factory that watches all namespaces
	factory := informers.NewSharedInformerFactoryWithOptions(clientSet, 0, informers.WithTweakListOptions(func(options *metav1.ListOptions) {
		options.LabelSelector = labelSelector
	}))

	// Create an informer that watches pods with the specified label selector
	informer := factory.Core().V1().Pods().Informer()

	watcher := &podWatcher{
		dbManager: dbManager,
		informer:  informer,
		clientSet: clientSet,
		startTime: time.Now(), // Track the start time of the watcher
	}

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    watcher.handleAdd,
		UpdateFunc: func(oldObj, newObj interface{}) { watcher.handleUpdate(oldObj, newObj) },
		DeleteFunc: watcher.handleDelete,
	})

	return watcher, nil
}
func (p *podWatcher) StartWatching() {
	stopCh := make(chan struct{})
	defer close(stopCh)
	p.informer.Run(stopCh)
}

func (p *podWatcher) handleAdd(obj interface{}) {
	eventTime := time.Now()
	pod, ok := obj.(*v1.Pod)
	if !ok {
		log.Log.Error(fmt.Errorf("invalid object"), "failed to parse pod object in handleAdd")
		return
	}

	// Filter out pods created before the watcher started
	if pod.CreationTimestamp.Time.Before(p.startTime) {
		return
	}

	log.Log.Info("pod event", "podUID", pod.UID, "event", "add", "status", pod.Status.Phase)

	if err := p.writeStatusToDB(pod, eventTime); err != nil {
		log.Log.Error(err, "failed to writeStatusToDB")
	}
}

func (p *podWatcher) handleUpdate(old, new interface{}) {
	eventTime := time.Now()
	oldPod, ok := old.(*v1.Pod)
	if !ok {
		log.Log.Error(fmt.Errorf("invalid object"), "failed to parse old pod object in handleUpdate")
		return
	}

	newPod, ok := new.(*v1.Pod)
	if !ok {
		log.Log.Error(fmt.Errorf("invalid object"), "failed to parse new pod object in handleUpdate")
		return
	}

	// Filter out pods created before the watcher started
	if oldPod.CreationTimestamp.Time.Before(p.startTime) {
		return
	}

	// nothing has changed so don't write it down
	if oldPod.Status.Phase == newPod.Status.Phase {
		return
	}

	log.Log.Info("pod event", "podUID", newPod.UID, "event", "update", "status", newPod.Status.Phase)

	if err := p.writeStatusToDB(newPod, eventTime); err != nil {
		log.Log.Error(err, "failed to writeStatusToDB")
	}

}

func (p *podWatcher) handleDelete(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		log.Log.Error(fmt.Errorf("invalid object"), "failed to parse pod object")
		return
	}

	log.Log.Info("pod was deleted", "podUID", pod.UID)
}

func (p *podWatcher) writeStatusToDB(pod *v1.Pod, stamp time.Time) error {
	taskRunIDStr, ok := pod.Annotations["kubeconductor/task-rid"]
	if !ok {
		return fmt.Errorf("missing annotation kubeconductor/task-rid")
	}

	taskRunId, err := strconv.Atoi(taskRunIDStr)
	if err != nil {
		return fmt.Errorf("failed to convert task run string: %s", taskRunIDStr)
	}

	var exitCode *int32 = nil
	if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].State.Terminated != nil {
		exitCode = &pod.Status.ContainerStatuses[0].State.Terminated.ExitCode
	}

	if err := p.dbManager.MarkPodStatus(context.Background(), pod.UID, pod.Name, taskRunId, pod.Status.Phase, stamp, exitCode); err != nil {
		return fmt.Errorf("failed to mark pods status: %v", pod.Status.Phase)
	}

	return nil
}
