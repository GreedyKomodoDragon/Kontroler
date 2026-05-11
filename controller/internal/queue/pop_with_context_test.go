package queue

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMemoryPopWithContextBlocksAndReturns(t *testing.T) {
	q := NewMemoryQueue(context.Background())
	defer q.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// start a goroutine that will pop and should block until we push
	ch := make(chan *PodEvent, 1)
	go func() {
		v, err := q.PopWithContext(ctx)
		if err != nil {
			ch <- nil
			return
		}
		ch <- v
	}()

	// sleep a bit to ensure goroutine is waiting
	time.Sleep(50 * time.Millisecond)

	// now push an item
	re := &PodEvent{Pod: nil, Event: "test-event"}
	require.NoError(t, q.Push(re))

	select {
	case got := <-ch:
		require.NotNil(t, got)
		require.Equal(t, re.Event, got.Event)
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for PopWithContext to return")
	}
}

func TestMemoryPopWithContextTimeout(t *testing.T) {
	q := NewMemoryQueue(context.Background())
	defer q.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := q.PopWithContext(ctx)
	require.Error(t, err)
	// Expect context deadline exceeded
	require.Equal(t, context.DeadlineExceeded, err)
}
