package queue

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func setupMemoryTestQueue(t *testing.T) Queue {
	return NewMemoryQueue(t.Context())
}

func TestNewMemoryQueue(t *testing.T) {
	q := NewMemoryQueue(context.Background())
	require.NotNil(t, q)
	defer q.Close()

	size, err := q.Size()
	require.NoError(t, err)
	require.Equal(t, uint64(0), size)
}

func TestMemoryPushPop(t *testing.T) {
	q := setupMemoryTestQueue(t)
	defer q.Close()

	testCases := []string{
		"test1",
		"test2",
		"test3",
	}

	// Test push
	for _, tc := range testCases {
		require.NoError(t, q.Push(tc))
	}

	// Test size
	size, err := q.Size()
	require.NoError(t, err)
	require.Equal(t, uint64(len(testCases)), size)

	// Test pop
	for _, expected := range testCases {
		got, err := q.Pop()
		require.NoError(t, err)
		require.Equal(t, expected, got)
	}

	// Test size after pop
	size, err = q.Size()
	require.NoError(t, err)
	require.Equal(t, uint64(0), size)

	// Test pop on empty queue
	_, err = q.Pop()
	require.Error(t, err)
}

func TestMemoryEmptyQueue(t *testing.T) {
	q := setupMemoryTestQueue(t)
	defer q.Close()

	// Test pop on empty queue
	_, err := q.Pop()
	require.Error(t, err)

	// Test size on empty queue
	size, err := q.Size()
	require.NoError(t, err)
	require.Equal(t, uint64(0), size)
}

func TestMemoryBatchOperations(t *testing.T) {
	q := setupMemoryTestQueue(t)
	defer q.Close()

	// Test PushBatch
	values := []string{"batch1", "batch2", "batch3"}
	require.NoError(t, q.PushBatch(values))

	size, err := q.Size()
	require.NoError(t, err)
	require.Equal(t, uint64(len(values)), size)

	// Test PopBatch
	results, err := q.PopBatch(2)
	require.NoError(t, err)
	require.Equal(t, values[:2], results)

	// Verify remaining size
	size, err = q.Size()
	require.NoError(t, err)
	require.Equal(t, uint64(1), size)

	// Pop remaining item
	result, err := q.Pop()
	require.NoError(t, err)
	require.Equal(t, values[2], result)
}

func TestMemoryLargeQueue(t *testing.T) {
	q := setupMemoryTestQueue(t)
	defer q.Close()

	// Push many items
	itemCount := 1000
	for i := 0; i < itemCount; i++ {
		require.NoError(t, q.Push(strconv.Itoa(i)))
	}

	size, err := q.Size()
	require.NoError(t, err)
	require.Equal(t, uint64(itemCount), size)

	// Pop all items in batches
	batchSize := 100
	for i := 0; i < itemCount/batchSize; i++ {
		results, err := q.PopBatch(batchSize)
		require.NoError(t, err)
		require.Len(t, results, batchSize)

		// Verify values
		for j, val := range results {
			expected := strconv.Itoa(i*batchSize + j)
			require.Equal(t, expected, val)
		}
	}

	// Verify empty
	size, err = q.Size()
	require.NoError(t, err)
	require.Equal(t, uint64(0), size)
}

func TestMemoryQueueConcurrency(t *testing.T) {
	q := setupMemoryTestQueue(t)
	defer q.Close()

	// Test concurrent pushes and pops
	itemCount := 100
	done := make(chan bool)

	// Start producer
	go func() {
		for i := 0; i < itemCount; i++ {
			require.NoError(t, q.Push(strconv.Itoa(i)))
		}
		done <- true
	}()

	// Start consumer
	go func() {
		count := 0
		for count < itemCount {
			if val, err := q.Pop(); err == nil {
				require.NotEmpty(t, val)
				count++
			}
		}
		done <- true
	}()

	// Wait for both goroutines to complete
	<-done
	<-done

	// Verify queue is empty
	size, err := q.Size()
	require.NoError(t, err)
	require.Equal(t, uint64(0), size)
}
