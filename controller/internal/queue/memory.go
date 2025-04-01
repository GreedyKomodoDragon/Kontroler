package queue

import (
	"context"
	"fmt"
	"sync"
)

type MemoryQueue struct {
	data   []string
	mutex  sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
}

func NewMemoryQueue(ctx context.Context) *MemoryQueue {
	ctx, cancel := context.WithCancel(ctx)
	return &MemoryQueue{
		data:   make([]string, 0),
		mutex:  sync.Mutex{},
		ctx:    ctx,
		cancel: cancel,
	}
}

func (q *MemoryQueue) Push(value string) error {
	return q.PushBatch([]string{value})
}

func (q *MemoryQueue) PushBatch(values []string) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	q.data = append(q.data, values...)
	return nil
}

func (q *MemoryQueue) Pop() (string, error) {
	values, err := q.PopBatch(1)
	if err != nil || len(values) == 0 {
		return "", err
	}
	return values[0], nil
}

func (q *MemoryQueue) PopBatch(count int) ([]string, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if len(q.data) == 0 {
		return nil, fmt.Errorf("queue is empty")
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
