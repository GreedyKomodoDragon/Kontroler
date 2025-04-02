package workers

import (
	"errors"
	"fmt"
	"time"

	"kontroler-controller/internal/queue"

	v1 "k8s.io/api/core/v1"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	ErrMissingAnnotation = errors.New("missing required annotation")
	ErrInvalidTaskRunID  = errors.New("invalid task run ID")
	kontrolerTaskRunID   = "kontroler/task-rid"
)

type ResourceEventHandler interface {
	HandleAdd(obj interface{})
	HandleUpdate(old, obj interface{})
	HandleDelete(obj interface{})
}

type podEventHandler struct {
	queue queue.Queue
}

func NewPodEventHandler(queue queue.Queue) ResourceEventHandler {
	return &podEventHandler{
		queue: queue,
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
	p.queue.Push(&queue.PodEvent{
		Pod:       pod,
		Event:     "add",
		EventTime: &eventTime,
	})
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
	p.queue.Push(&queue.PodEvent{
		Pod:       pod,
		Event:     "update",
		EventTime: &eventTime,
	})
}

func (p *podEventHandler) HandleDelete(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		log.Log.Error(fmt.Errorf("invalid object"), "failed to parse pod object")
		return
	}

	log.Log.Info("pod was deleted", "podUid", pod.UID)
}
