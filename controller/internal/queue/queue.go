package queue

import (
	"errors"
	"time"

	v1 "k8s.io/api/core/v1"
)

var (
	ErrQueueIsEmpty = errors.New("queue is empty")
)

type PodEvent struct {
	Pod       *v1.Pod
	Event     string
	EventTime *time.Time
}

type Queue interface {
	Start() error
	Push(value *PodEvent) error
	PushBatch(values []*PodEvent) error
	Pop() (*PodEvent, error)
	PopBatch(count int) ([]*PodEvent, error)
	Size() (uint64, error)
	Close() error
}
