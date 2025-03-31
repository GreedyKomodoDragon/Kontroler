package queue

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/cockroachdb/pebble"
)

const keyFormat = "%s:%08d"

// QueueOptions contains configuration for the queue
type QueueOptions struct {
	BatchSize    int
	MemTableSize uint64
	BytesPerSync int
	Timeout      time.Duration
}

// DefaultOptions returns the default queue options
func DefaultOptions() *QueueOptions {
	return &QueueOptions{
		BatchSize:    10,
		MemTableSize: 64 << 20,  // 64MB
		BytesPerSync: 256 << 10, // 256KB
		Timeout:      5 * time.Second,
	}
}

// Queue represents a FIFO queue backed by PebbleDB
type Queue struct {
	db      *pebble.DB
	topic   string
	headKey string
	tailKey string
	opts    *QueueOptions
	batch   *pebble.Batch
	count   int
	mutex   *sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewQueue initializes a new queue in PebbleDB
func NewQueue(ctx context.Context, dbPath, topic string, opts *QueueOptions) (*Queue, error) {
	if opts == nil {
		opts = DefaultOptions()
	}

	dbOpts := &pebble.Options{}

	if opts.MemTableSize > 0 {
		dbOpts.MemTableSize = opts.MemTableSize
	}

	if opts.BytesPerSync > 0 {
		dbOpts.BytesPerSync = opts.BytesPerSync
	}

	db, err := pebble.Open(dbPath, dbOpts)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)
	q := &Queue{
		db:      db,
		topic:   topic,
		headKey: topic + ":head",
		tailKey: topic + ":tail",
		opts:    opts,
		batch:   db.NewBatch(),
		mutex:   &sync.Mutex{},
		ctx:     ctx,
		cancel:  cancel,
		wg:      sync.WaitGroup{},
	}

	q.wg.Add(1)
	go q.startTimer()
	return q, nil
}

func (q *Queue) startTimer() {
	defer q.wg.Done()
	ticker := time.NewTicker(q.opts.Timeout)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			q.mutex.Lock()
			if q.count > 0 {
				q.Flush()
			}
			q.mutex.Unlock()
		case <-q.ctx.Done():
			return
		}
	}
}

// Push adds an item to the queue
func (q *Queue) Push(value string) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	tail, _ := q.getCounter(q.tailKey)
	tail++
	key := fmt.Sprintf(keyFormat, q.topic, tail)
	q.batch.Set([]byte(key), []byte(value), nil)
	q.batch.Set([]byte(q.tailKey), []byte(strconv.FormatUint(tail, 10)), nil)
	q.count++

	if q.count >= q.opts.BatchSize {
		return q.Flush()
	}

	return nil
}

// Flush commits any pending writes
func (q *Queue) Flush() error {
	if q.count > 0 {
		if err := q.batch.Commit(nil); err != nil {
			return err
		}
		q.batch = q.db.NewBatch()
		q.count = 0
	}
	return nil
}

// Close flushes pending writes and closes the database
func (q *Queue) Close() error {
	q.cancel()
	q.wg.Wait() // Wait for timer goroutine to finish
	if err := q.Flush(); err != nil {
		return err
	}
	return q.db.Close()
}

// PushBatch adds multiple items to the queue efficiently
func (q *Queue) PushBatch(values []string) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	tail, _ := q.getCounter(q.tailKey)

	batch := q.db.NewBatch()
	for i, value := range values {
		key := fmt.Sprintf(keyFormat, q.topic, tail+uint64(i)+1)
		batch.Set([]byte(key), []byte(value), nil)
	}

	batch.Set([]byte(q.tailKey), []byte(strconv.FormatUint(tail+uint64(len(values)), 10)), nil)
	return batch.Commit(nil)
}

// Pop removes and returns the oldest item from the queue
func (q *Queue) Pop() (string, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	head, err := q.getCounter(q.headKey)
	if err != nil {
		return "", fmt.Errorf("failed to get head counter: %w", err)
	}

	tail, err := q.getCounter(q.tailKey)
	if err != nil {
		return "", fmt.Errorf("failed to get tail counter: %w", err)
	}

	if head >= tail {
		return "", fmt.Errorf("queue is empty")
	}

	head++ // Move head forward
	key := fmt.Sprintf(keyFormat, q.topic, head)

	// First read the value
	value, closer, err := q.db.Get([]byte(key))
	if err != nil {
		return "", fmt.Errorf("failed to get value: %w", err)
	}
	valueStr := string(value)
	closer.Close()

	// Then update head pointer and delete the entry
	batch := q.db.NewBatch()
	batch.Set([]byte(q.headKey), []byte(strconv.FormatUint(head, 10)), nil)
	batch.Delete([]byte(key), nil)
	if err := batch.Commit(pebble.Sync); err != nil {
		return "", fmt.Errorf("failed to commit batch: %w", err)
	}

	return valueStr, nil
}

// PopBatch removes and returns multiple items from the queue
func (q *Queue) PopBatch(count int) ([]string, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	head, err := q.getCounter(q.headKey)
	if err != nil {
		return nil, err
	}

	tail, err := q.getCounter(q.tailKey)
	if err != nil {
		return nil, err
	}

	available := tail - head
	if available == 0 {
		return nil, fmt.Errorf("queue is empty")
	}

	if uint64(count) > available {
		count = int(available)
	}

	results := make([]string, 0, count)
	batch := q.db.NewBatch()

	for i := 0; i < count; i++ {
		key := fmt.Sprintf(keyFormat, q.topic, head+uint64(i)+1)
		value, closer, err := q.db.Get([]byte(key))
		if err != nil {
			return results, err
		}
		results = append(results, string(value))
		closer.Close()
		batch.Delete([]byte(key), nil)
	}

	batch.Set([]byte(q.headKey), []byte(strconv.FormatUint(head+uint64(count), 10)), nil)
	if err := batch.Commit(nil); err != nil {
		return results, err
	}

	return results, nil
}

// Peek returns the next item without removing it
func (q *Queue) Peek() (string, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	head, _ := q.getCounter(q.headKey)
	tail, _ := q.getCounter(q.tailKey)

	if head >= tail {
		return "", fmt.Errorf("queue is empty")
	}

	head++
	key := fmt.Sprintf("%s:%08d", q.topic, head)

	value, closer, err := q.db.Get([]byte(key))
	if err != nil {
		return "", err
	}
	defer closer.Close()

	return string(value), nil
}

// getCounter retrieves a stored counter (head or tail pointer)
func (q *Queue) getCounter(key string) (uint64, error) {
	value, closer, err := q.db.Get([]byte(key))
	if err == pebble.ErrNotFound {
		return 0, nil // Default to 0 if not found
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get counter: %w", err)
	}
	defer closer.Close()

	count, err := strconv.ParseUint(string(value), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse counter: %w", err)
	}
	return count, nil
}

// Size returns the current number of items in the queue
func (q *Queue) Size() (uint64, error) {
	head, err := q.getCounter(q.headKey)
	if err != nil {
		return 0, fmt.Errorf("failed to get head counter: %w", err)
	}

	tail, err := q.getCounter(q.tailKey)
	if err != nil {
		return 0, fmt.Errorf("failed to get tail counter: %w", err)
	}

	return tail - head, nil
}

// Compact manually compacts the database to free space
func (q *Queue) Compact() {
	q.db.Compact([]byte(q.topic+":"), []byte(q.topic+":\xff"), true)
}
