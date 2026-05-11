package queue

import (
	"context"
	"sync"
)

// MemoryQueue is a simple in-memory queue. It used to re-slice the backing
// array on Pop which caused the backing array to retain references to
// popped elements leading to unbounded memory retention. This implementation
// uses a head index and periodically compacts the underlying slice to avoid
// retaining popped elements while keeping low allocation overhead.

type MemoryQueue struct {
	data   []*PodEvent
	head   int
	mutex  sync.Mutex
	notify chan struct{}
	ctx    context.Context
	cancel context.CancelFunc
}

func NewMemoryQueue(ctx context.Context) *MemoryQueue {
	ctx, cancel := context.WithCancel(ctx)
	q := &MemoryQueue{
		data:   make([]*PodEvent, 0),
		head:   0,
		mutex:  sync.Mutex{},
		notify: make(chan struct{}, 1),
		ctx:    ctx,
		cancel: cancel,
	}
	return q
}

func (q *MemoryQueue) Push(value *PodEvent) error {
	return q.PushBatch([]*PodEvent{value})
}

func (q *MemoryQueue) PushBatch(values []*PodEvent) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	q.data = append(q.data, values...)
	// notify a waiter if any (non-blocking)
	select {
	case q.notify <- struct{}{}:
	default:
	}
	return nil
}

func (q *MemoryQueue) Pop() (*PodEvent, error) {
	return q.PopWithContext(context.Background())
}

func (q *MemoryQueue) PopBatch(count int) ([]*PodEvent, error) {
	return q.PopBatchWithContext(context.Background(), count)
}

func (q *MemoryQueue) PopWithContext(ctx context.Context) (*PodEvent, error) {
	vals, err := q.PopBatchWithContext(ctx, 1)
	if err != nil {
		return nil, err
	}
	return vals[0], nil
}

func (q *MemoryQueue) PopBatchWithContext(ctx context.Context, count int) ([]*PodEvent, error) {
	for {
		q.mutex.Lock()
		available := len(q.data) - q.head
		if available == 0 {
			q.mutex.Unlock()
			// Wait for either ctx done or a notify from Push
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-q.ctx.Done():
				return nil, ErrQueueIsEmpty
			case <-q.notify:
				// notified, loop and re-check under lock
			}
			continue
		}

		if count > available {
			count = available
		}

		// Copy out the results so we don't return references into the backing
		// array (which we'll nil-out and possibly compact). This avoids keeping
		// popped elements alive.
		res := make([]*PodEvent, count)
		copy(res, q.data[q.head:q.head+count])

		// Nil out references in the backing array to allow GC of popped items.
		for i := 0; i < count; i++ {
			q.data[q.head+i] = nil
		}

		q.head += count

		// Periodic compaction: if head is large we shift remaining items to a
		// fresh slice to prevent the backing array from growing indefinitely.
		if q.head > 1024 && q.head*2 > len(q.data) {
			// Copy remaining to a new slice (fresh backing array)
			remaining := q.data[q.head:]
			newData := append([]*PodEvent(nil), remaining...)
			q.data = newData
			q.head = 0
		}

		q.mutex.Unlock()
		return res, nil
	}
}

func (q *MemoryQueue) Size() (uint64, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	return uint64(len(q.data) - q.head), nil
}

func (q *MemoryQueue) Close() error {
	q.cancel()
	// Wake up any blocked waiters by notifying the channel (non-blocking)
	select {
	case q.notify <- struct{}{}:
	default:
	}
	return nil
}
