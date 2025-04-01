package queue

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
)

func setupBenchmarkQueue(b *testing.B) (*Queue, func()) {
	tmpDir, err := os.MkdirTemp("", "queue-bench-*")
	if err != nil {
		b.Fatal(err)
	}

	q, err := NewQueue(context.Background(), tmpDir, "bench-topic")
	if err != nil {
		os.RemoveAll(tmpDir)
		b.Fatal(err)
	}

	cleanup := func() {
		q.Close()
		os.RemoveAll(tmpDir)
	}

	return q, cleanup
}

func BenchmarkQueuePush(b *testing.B) {
	q, cleanup := setupBenchmarkQueue(b)
	defer cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := q.Push("test message"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkQueuePop(b *testing.B) {
	q, cleanup := setupBenchmarkQueue(b)
	defer cleanup()

	// Pre-fill queue
	for i := 0; i < b.N; i++ {
		if err := q.Push("test message"); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := q.Pop(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkQueuePushPop(b *testing.B) {
	q, cleanup := setupBenchmarkQueue(b)
	defer cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := q.Push("test message"); err != nil {
			b.Fatal(err)
		}
		if _, err := q.Pop(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkQueueWithDifferentSizes(b *testing.B) {
	sizes := []int{16, 256, 1024, 4096} // bytes
	for _, size := range sizes {
		message := make([]byte, size)
		for i := range message {
			message[i] = 'x'
		}

		b.Run(strconv.Itoa(size), func(b *testing.B) {
			q, cleanup := setupBenchmarkQueue(b)
			defer cleanup()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := q.Push(string(message)); err != nil {
					b.Fatal(err)
				}
				if _, err := q.Pop(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkQueueBatch(b *testing.B) {
	batchSizes := []int{10, 100, 1000}
	for _, batchSize := range batchSizes {
		b.Run(strconv.Itoa(batchSize), func(b *testing.B) {
			q, cleanup := setupBenchmarkQueue(b)
			defer cleanup()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Push batch
				for j := 0; j < batchSize; j++ {
					if err := q.Push("test message"); err != nil {
						b.Fatal(err)
					}
				}
				// Pop batch
				for j := 0; j < batchSize; j++ {
					if _, err := q.Pop(); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}

func BenchmarkQueueBatchOperations(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "queue-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	q, err := NewQueue(context.Background(), tmpDir, "bench-topic")
	if err != nil {
		b.Fatal(err)
	}
	defer q.Close()

	batchSizes := []int{10, 100, 1000}
	messages := make([]string, 1000)
	for i := range messages {
		messages[i] = "test message"
	}

	for _, size := range batchSizes {
		b.Run(fmt.Sprintf("batch-%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if err := q.PushBatch(messages[:size]); err != nil {
					b.Fatal(err)
				}
				if _, err := q.PopBatch(size); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
