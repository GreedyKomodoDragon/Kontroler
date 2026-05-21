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

func setupTestQueue(t *testing.T) (Queue, string, func()) {
	tmpDir, err := os.MkdirTemp("", "queue-test-*")
	require.NoError(t, err)

	q, err := NewPebbleQueue(t.Context(), tmpDir, "test-topic")
	if err != nil {
		os.RemoveAll(tmpDir)
		require.NoError(t, err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return q, tmpDir, cleanup
}

func TestNewPebbleQueue(t *testing.T) {
	// Test invalid path - use a different directory
	nonexistentDir, err := os.MkdirTemp("", "queue-test-nonexistent-*")
	require.NoError(t, err)
	defer func(){ _ = os.RemoveAll(nonexistentDir) }()

	q1, err := NewPebbleQueue(t.Context(), filepath.Join(nonexistentDir, "subdir"), "test-topic")
	require.NoError(t, err)
	defer func() { _ = q1.Close() }()

	// Test custom options - use another different directory
	optsDir, err := os.MkdirTemp("", "queue-test-opts-*")
	require.NoError(t, err)
	defer func(){ _ = os.RemoveAll(optsDir) }()

	q2, err := NewPebbleQueue(context.Background(), optsDir, "test-topic-opts")
	require.NoError(t, err)
	defer func() { _ = q2.Close() }()
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
		require.NoError(t, q.Push(&PodEvent{
			Pod:   nil,
			Event: tc,
		}))
	}

	// Test size
	size, err := q.Size()
	require.NoError(t, err)

	require.Equal(t, uint64(len(testCases)), size)

	// Test pop
	for _, expected := range testCases {
		got, err := q.PopWithContext(context.Background())
		require.NoError(t, err)
		require.Equal(t, expected, got.Event)
	}

	// Test size after pop
	size, err = q.Size()
	require.NoError(t, err)
	require.Equal(t, uint64(0), size)

	// Test pop on empty queue (should block) - use a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = q.PopWithContext(ctx)
	require.Error(t, err)
	require.Equal(t, context.DeadlineExceeded, err)
}

func TestEmptyQueue(t *testing.T) {
	q, _, cleanup := setupTestQueue(t)
	defer cleanup()

	// Test pop on empty queue (should block) - use a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := q.PopWithContext(ctx)
	require.Error(t, err)
	require.Equal(t, context.DeadlineExceeded, err)

	// Test size on empty queue
	size, err := q.Size()
	require.NoError(t, err)
	require.Equal(t, uint64(0), size)
}

func TestPeek(t *testing.T) {
	q, _, cleanup := setupTestQueue(t)
	defer cleanup()

	testValue := &PodEvent{
		Pod:   nil,
		Event: "peek-test",
	}

	require.NoError(t, q.Push(testValue))

	// Verify value is still there after peek
	val, err := q.PopWithContext(context.Background())
	require.NoError(t, err)
	require.Equal(t, testValue.Event, val.Event)
}

func TestQueuePersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "queue-persist-test-*")
	require.NoError(t, err)
	defer func(){ _ = os.RemoveAll(tmpDir) }()

	testValue := &PodEvent{
		Pod:   nil,
		Event: "persist-test",
	}

	// Create queue and push value
	q1, err := NewPebbleQueue(t.Context(), tmpDir, "test-topic")
	require.NoError(t, err)
	require.NoError(t, q1.Push(testValue))

	// Create new queue instance and verify value
	q2, err := NewPebbleQueue(t.Context(), tmpDir, "test-topic")
	require.NoError(t, err)
	defer func() { _ = q2.Close() }()

	val, err := q2.PopWithContext(context.Background())
	require.NoError(t, err)
	require.Equal(t, testValue.Event, val.Event)
}

func TestLargeQueue(t *testing.T) {
	q, _, cleanup := setupTestQueue(t)
	defer cleanup()

	// Push many items
	itemCount := 1000
	for i := 0; i < itemCount; i++ {
		require.NoError(t, q.Push(&PodEvent{
			Pod:   nil,
			Event: strconv.Itoa(i),
		}))
	}

	size, err := q.Size()
	require.NoError(t, err)
	require.Equal(t, uint64(itemCount), size)

	// Pop all items
	for i := 0; i < itemCount; i++ {
		_, err := q.PopWithContext(context.Background())
		require.NoError(t, err)
	}

	// Verify empty
	size, err = q.Size()
	require.NoError(t, err)
	require.Equal(t, uint64(0), size)
}
