package queue

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func setupTestQueue(t *testing.T) (*Queue, string, func()) {
	tmpDir, err := os.MkdirTemp("", "queue-test-*")
	require.NoError(t, err)

	q, err := NewQueue(t.Context(), tmpDir, "test-topic", DefaultOptions())
	if err != nil {
		os.RemoveAll(tmpDir)
		require.NoError(t, err)
	}

	cleanup := func() {
		q.Close()
		os.RemoveAll(tmpDir)
	}

	return q, tmpDir, cleanup
}

func TestNewQueue(t *testing.T) {

	// Test invalid path - use a different directory
	nonexistentDir, err := os.MkdirTemp("", "queue-test-nonexistent-*")
	require.NoError(t, err)
	defer os.RemoveAll(nonexistentDir)

	q1, err := NewQueue(t.Context(), filepath.Join(nonexistentDir, "subdir"), "test-topic", nil)
	require.NoError(t, err)
	defer q1.Close()

	// Test custom options - use another different directory
	optsDir, err := os.MkdirTemp("", "queue-test-opts-*")
	require.NoError(t, err)
	defer os.RemoveAll(optsDir)

	opts := &QueueOptions{
		BatchSize:    100,
		MemTableSize: 32 << 20,
		Timeout:      1 * time.Second,
	}

	q2, err := NewQueue(context.Background(), optsDir, "test-topic-opts", opts)
	require.NoError(t, err)
	defer q2.Close()
}

func TestPushPop(t *testing.T) {
	q, _, cleanup := setupTestQueue(t)
	defer cleanup()

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

func TestEmptyQueue(t *testing.T) {
	q, _, cleanup := setupTestQueue(t)
	defer cleanup()

	// Test pop on empty queue
	_, err := q.Pop()
	require.Error(t, err)

	// Test peek on empty queue
	_, err = q.Peek()
	require.Error(t, err)

	// Test size on empty queue
	size, err := q.Size()
	require.NoError(t, err)
	require.Equal(t, uint64(0), size)
}

func TestPeek(t *testing.T) {
	q, _, cleanup := setupTestQueue(t)
	defer cleanup()

	testValue := "peek-test"
	require.NoError(t, q.Push(testValue))

	// Test multiple peeks
	for i := 0; i < 3; i++ {
		val, err := q.Peek()
		require.NoError(t, err)
		require.Equal(t, testValue, val)
	}

	// Verify value is still there after peek
	val, err := q.Pop()
	require.NoError(t, err)
	require.Equal(t, testValue, val)
}

func TestQueuePersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "queue-persist-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testValue := "persistence-test"

	// Create queue and push value
	q1, err := NewQueue(t.Context(), tmpDir, "test-topic", DefaultOptions())
	require.NoError(t, err)
	require.NoError(t, q1.Push(testValue))
	q1.Close()

	// Create new queue instance and verify value
	q2, err := NewQueue(t.Context(), tmpDir, "test-topic", DefaultOptions())
	require.NoError(t, err)
	defer q2.Close()

	val, err := q2.Pop()
	require.NoError(t, err)
	require.Equal(t, testValue, val)
}

func TestLargeQueue(t *testing.T) {
	q, _, cleanup := setupTestQueue(t)
	defer cleanup()

	// Push many items
	itemCount := 1000
	for i := 0; i < itemCount; i++ {
		require.NoError(t, q.Push(strconv.Itoa(i)))
	}

	size, err := q.Size()
	require.NoError(t, err)
	require.Equal(t, uint64(itemCount), size)

	// Pop all items
	for i := 0; i < itemCount; i++ {
		_, err := q.Pop()
		require.NoError(t, err)
	}

	// Verify empty
	size, err = q.Size()
	require.NoError(t, err)
	require.Equal(t, uint64(0), size)
}
