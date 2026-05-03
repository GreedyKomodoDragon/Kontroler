package object

import (
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	"sync"
)

func TestS3FetchingStatus(t *testing.T) {
	store := &s3LogStore{
		fetching: map[string]bool{},
		lock:     &sync.RWMutex{},
	}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			UID:       types.UID("test-uid-xyz"),
		},
	}
	dagRunId := 999

	// Initial state should be not fetching
	require.False(t, store.IsFetching(dagRunId, pod), "Should not be fetching initially")

	// Mark as fetching
	require.NoError(t, store.MarkAsFetching(dagRunId, pod), "Should be able to mark as fetching")

	// Should be fetching now
	require.True(t, store.IsFetching(dagRunId, pod), "Should be fetching after marking")

	// Try marking again - should fail
	require.Error(t, store.MarkAsFetching(dagRunId, pod), "Should not be able to mark as fetching twice")

	// Unlist fetching
	store.UnlistFetching(dagRunId, pod)

	// Should not be fetching anymore
	require.False(t, store.IsFetching(dagRunId, pod), "Should not be fetching after unlisting")
}
