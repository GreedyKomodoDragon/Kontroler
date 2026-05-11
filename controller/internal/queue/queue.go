package queue

import (
	"context"
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
	Push(value *PodEvent) error
	PushBatch(values []*PodEvent) error
	// Context-aware pop operations. Implementations should support context
	// cancellation/timeout. Backwards-compatible Pop/PopBatch wrappers are
	// provided and call these with context.Background().
	PopWithContext(ctx context.Context) (*PodEvent, error)
	PopBatchWithContext(ctx context.Context, count int) ([]*PodEvent, error)
	Pop() (*PodEvent, error)
	PopBatch(count int) ([]*PodEvent, error)
	Size() (uint64, error)
	Close() error
}
