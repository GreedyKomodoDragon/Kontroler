package queue

import (
	"context"
	"sync"
)

type MemoryQueue struct {
	data   []*PodEvent
	mutex  sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
}

func NewMemoryQueue(ctx context.Context) *MemoryQueue {
	ctx, cancel := context.WithCancel(ctx)
	return &MemoryQueue{
		data:   make([]*PodEvent, 0),
		mutex:  sync.Mutex{},
		ctx:    ctx,
		cancel: cancel,
	}
}

func (q *MemoryQueue) Push(value *PodEvent) error {
	return q.PushBatch([]*PodEvent{value})
}

func (q *MemoryQueue) PushBatch(values []*PodEvent) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	q.data = append(q.data, values...)
	return nil
}

func (q *MemoryQueue) Pop() (*PodEvent, error) {
	values, err := q.PopBatch(1)
	if err != nil {
		return nil, err
	}
	return values[0], nil
}

func (q *MemoryQueue) PopBatch(count int) ([]*PodEvent, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if len(q.data) == 0 {
		return nil, ErrQueueIsEmpty
	}

	if count > len(q.data) {
		count = len(q.data)
	}

	result := q.data[:count]
	q.data = q.data[count:]
	return result, nil
}

func (q *MemoryQueue) Size() (uint64, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	return uint64(len(q.data)), nil
}

func (q *MemoryQueue) Close() error {
	q.cancel()
	return nil
}
