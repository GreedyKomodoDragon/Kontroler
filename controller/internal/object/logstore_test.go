package object

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
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
	pod2 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			UID:       types.UID("test-uid-abc"),
		},
	}
	dagRunId := 999

	require.False(t, store.IsFetching(dagRunId, pod), "Should not be fetching initially")
	require.False(t, store.IsFetching(dagRunId, pod2), "Second pod should not be fetching initially")

	require.NoError(t, store.MarkAsFetching(dagRunId, pod), "Should be able to mark as fetching")
	require.True(t, store.IsFetching(dagRunId, pod), "Should be fetching after marking")
	require.False(t, store.IsFetching(dagRunId, pod2), "Second pod should remain independent")
	require.Error(t, store.MarkAsFetching(dagRunId, pod), "Should not be able to mark as fetching twice")

	require.NoError(t, store.MarkAsFetching(dagRunId, pod2), "Second pod should be markable independently")
	require.True(t, store.IsFetching(dagRunId, pod2), "Second pod should be fetching after marking")

	store.UnlistFetching(dagRunId, pod)
	store.UnlistFetching(dagRunId, pod2)
	require.False(t, store.IsFetching(dagRunId, pod), "Should not be fetching after unlisting")
	require.False(t, store.IsFetching(dagRunId, pod2), "Second pod should not be fetching after unlisting")
}
