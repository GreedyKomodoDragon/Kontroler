package workers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"kontroler-controller/internal/queue"

	"github.com/cespare/xxhash/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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

type eventHandler struct {
	queues    []queue.Queue
	clientset kubernetes.Interface
}

func NewEventHandler(queues []queue.Queue, clientset kubernetes.Interface) ResourceEventHandler {
	return &eventHandler{
		queues:    queues,
		clientset: clientset,
	}
}

func (e *eventHandler) HandleAdd(obj interface{}) {
	event, ok := obj.(*v1.Event)
	if !ok {
		log.Log.Error(fmt.Errorf("invalid object"), "failed to parse event object")
		return
	}

	// Check if the involved object has required labels and get the pod
	ok, pod := hasRequiredLabels(&event.InvolvedObject, e.clientset)
	if !ok || pod == nil {
		return
	}

	if event.Type == "Warning" {
		log.Log.Info("warning event detected",
			"reason", event.Reason,
			"message", event.Message,
			"involvedObject", event.InvolvedObject.Name)

		eventTime := time.Now()
		if err := e.queues[e.getQueueIndex(event)].Push(&queue.PodEvent{
			Pod:       pod,
			Event:     "update",
			EventTime: &eventTime,
		}); err != nil {
			log.Log.Error(err, "failed to push event to queue")
			return
		}
	}
}

func (e *eventHandler) HandleUpdate(old, obj interface{}) {
	event, ok := obj.(*v1.Event)
	if !ok {
		log.Log.Error(fmt.Errorf("invalid object"), "failed to parse event object")
		return
	}

	if event.Type != "Warning" {
		return // We only care about warning events
	}

	// Check if the involved object has required labels and get the pod
	ok, pod := hasRequiredLabels(&event.InvolvedObject, e.clientset)
	if !ok || pod == nil {
		return
	}

	eventTime := time.Now()
	if err := e.queues[e.getQueueIndex(event)].Push(&queue.PodEvent{
		Pod:       pod,
		Event:     "update",
		EventTime: &eventTime,
	}); err != nil {
		log.Log.Error(err, "failed to push event to queue")
		return
	}
}

func (e *eventHandler) HandleDelete(obj interface{}) {
	// We  don't need to handle event deletions
	return
}

func (e *eventHandler) getQueueIndex(event *v1.Event) int {
	hasher := xxhash.NewWithSeed(0xABC)
	hasher.Write([]byte(event.InvolvedObject.Name))
	hasher.Write([]byte(event.InvolvedObject.Namespace))
	index := int(hasher.Sum64() % uint64(len(e.queues)))
	return index
}

// Update hasRequiredLabels to return both the result and the pod
func hasRequiredLabels(obj *v1.ObjectReference, clientset kubernetes.Interface) (bool, *v1.Pod) {
	if obj == nil || obj.Kind != "Pod" {
		return false, nil
	}

	pod, err := clientset.CoreV1().Pods(obj.Namespace).Get(context.Background(), obj.Name, metav1.GetOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			// silently ignore if the pod is not found
			return false, nil
		}

		log.Log.Error(err, "failed to get pod from reference",
			"name", obj.Name,
			"namespace", obj.Namespace)
		return false, nil
	}

	if pod.Labels == nil {
		return false, nil
	}

	if managedBy, ok := pod.Labels["managed-by"]; ok && managedBy == "kontroler" {
		if _, ok := pod.Labels["kontroler/id"]; ok {
			return true, pod
		}
	}

	return false, nil
}
