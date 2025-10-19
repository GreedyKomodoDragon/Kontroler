package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/cockroachdb/pebble"
)

const keyFormat = "%s:%08d"

type PebbleQueue struct {
	db                *pebble.DB
	dbPath            string
	topic             string
	headKey           string
	tailKey           string
	mutex             sync.Mutex
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	lastCommittedHead uint64
}

func NewPebbleQueue(ctx context.Context, dbPath, topic string) (*PebbleQueue, error) {
	ctx, cancel := context.WithCancel(ctx)

	// Open the database immediately during construction
	db, err := pebble.Open(dbPath, &pebble.Options{})
	if err != nil {
		cancel()
		return nil, err
	}

	q := &PebbleQueue{
		db:      db,
		dbPath:  dbPath,
		topic:   topic,
		headKey: topic + ":head",
		tailKey: topic + ":tail",
		mutex:   sync.Mutex{},
		ctx:     ctx,
		cancel:  cancel,
		wg:      sync.WaitGroup{},
	}

	// Initialize counters
	head, _ := q.getCounter(q.headKey)
	tail, _ := q.getCounter(q.tailKey)

	if tail < head {
		tail = head
		q.updateCounter(q.tailKey, tail)
	}

	q.lastCommittedHead = head
	return q, nil
}

func (q *PebbleQueue) Push(value *PodEvent) error {
	return q.PushBatch([]*PodEvent{value})
}

func (q *PebbleQueue) PushBatch(values []*PodEvent) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	tail, _ := q.getCounter(q.tailKey)
	batch := q.db.NewBatch()

	for _, value := range values {
		tail++
		key := fmt.Sprintf(keyFormat, q.topic, tail)
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		batch.Set([]byte(key), data, nil)
	}

	batch.Set([]byte(q.tailKey), []byte(strconv.FormatUint(tail, 10)), nil)
	return batch.Commit(pebble.Sync)
}

func (q *PebbleQueue) Pop() (*PodEvent, error) {
	values, err := q.PopBatch(1)
	if err != nil {
		return nil, err
	}
	return values[0], nil
}

func (q *PebbleQueue) PopBatch(count int) ([]*PodEvent, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	head, _ := q.getCounter(q.headKey)
	tail, _ := q.getCounter(q.tailKey)

	available := int(tail - head)
	if available <= 0 {
		return nil, ErrQueueIsEmpty
	}

	if count > available {
		count = available
	}

	results := make([]*PodEvent, 0, count)
	batch := q.db.NewBatch()

	for i := 0; i < count; i++ {
		head++
		key := fmt.Sprintf(keyFormat, q.topic, head)
		value, closer, err := q.db.Get([]byte(key))
		if err != nil {
			return results, err
		}
		var event PodEvent
		if err := json.Unmarshal(value, &event); err != nil {
			closer.Close()
			return results, err
		}
		results = append(results, &event)
		closer.Close()
		batch.Delete([]byte(key), nil)
	}

	batch.Set([]byte(q.headKey), []byte(strconv.FormatUint(head, 10)), nil)
	if err := batch.Commit(pebble.Sync); err != nil {
		return results, err
	}

	q.lastCommittedHead = head
	return results, nil
}

func (q *PebbleQueue) getCounter(key string) (uint64, error) {
	value, closer, err := q.db.Get([]byte(key))
	if err == pebble.ErrNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer closer.Close()

	return strconv.ParseUint(string(value), 10, 64)
}

func (q *PebbleQueue) updateCounter(key string, value uint64) error {
	return q.db.Set([]byte(key), []byte(strconv.FormatUint(value, 10)), pebble.Sync)
}

func (q *PebbleQueue) Size() (uint64, error) {
	head, err := q.getCounter(q.headKey)
	if err != nil {
		return 0, err
	}

	tail, err := q.getCounter(q.tailKey)
	if err != nil {
		return 0, err
	}

	return tail - head, nil
}

func (q *PebbleQueue) Close() error {
	q.cancel()
	q.wg.Wait()
	return q.db.Close()
}
