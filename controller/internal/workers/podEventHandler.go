package workers

import (
	"fmt"
	"kontroler-controller/internal/queue"
	"time"

	"github.com/cespare/xxhash/v2"
	v1 "k8s.io/api/core/v1"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

type podEventHandler struct {
	queues []queue.Queue
}

func NewPodEventHandler(queues []queue.Queue) ResourceEventHandler {
	return &podEventHandler{
		queues: queues,
	}
}

func (p *podEventHandler) HandleAdd(obj interface{}) {
	eventTime := time.Now()

	pod, ok := obj.(*v1.Pod)
	if !ok {
		log.Log.Error(fmt.Errorf("invalid object"), "failed to parse pod object")
		return
	}

	log.Log.Info("pod was added", "podUID", pod.UID, "name", pod.Name)

	if err := p.queues[p.getQueueIndex(pod)].Push(&queue.PodEvent{
		Pod:       pod,
		Event:     "add",
		EventTime: &eventTime,
	}); err != nil {
		log.Log.Error(err, "failed to push pod event to queue", "podUID", pod.UID)
		return
	}
}

func (p *podEventHandler) HandleUpdate(old, obj interface{}) {
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

	// TODO: See if this is an issue
	if oldPod.Status.Phase == pod.Status.Phase {
		return
	}

	log.Log.Info("pod was updated", "podUID", pod.UID, "name", pod.Name, "newPhase", pod.Status.Phase)

	if err := p.queues[p.getQueueIndex(pod)].Push(&queue.PodEvent{
		Pod:       pod,
		Event:     "update",
		EventTime: &eventTime,
	}); err != nil {
		log.Log.Error(err, "failed to push pod event to queue", "podUID", pod.UID)
		return
	}
}

func (p *podEventHandler) HandleDelete(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		log.Log.Error(fmt.Errorf("invalid object"), "failed to parse pod object")
		return
	}

	log.Log.Info("pod was deleted", "podUid", pod.UID)
}

func (p *podEventHandler) getQueueIndex(pod *v1.Pod) int {
	hasher := xxhash.NewWithSeed(0xABC)
	hasher.Write([]byte(pod.Name))
	hasher.Write([]byte(pod.Namespace))
	index := int(hasher.Sum64() % uint64(len(p.queues)))
	return index
}
