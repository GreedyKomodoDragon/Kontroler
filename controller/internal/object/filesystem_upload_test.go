package object

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
)

// fake streamer and getter implementations
type fakeStreamer struct {
	data []byte
	err  error
}

func (f *fakeStreamer) Stream(ctx context.Context) (io.ReadCloser, error) {
	if f.err != nil {
		return nil, f.err
	}
	return io.NopCloser(bytes.NewReader(f.data)), nil
}

type fakeGetter struct {
	stream podLogStreamer
	calls  int32
}

func (f *fakeGetter) GetLogs(podName string, opts *v1.PodLogOptions) podLogStreamer {
	atomic.AddInt32(&f.calls, 1)
	return f.stream
}

// retryStreamer is a helper that fails the first `failUntil` Stream calls then succeeds
type retryStreamer struct {
	count     *int32
	failUntil int32
	data      []byte
}

func (r *retryStreamer) Stream(ctx context.Context) (io.ReadCloser, error) {
	if atomic.AddInt32(r.count, 1) <= r.failUntil {
		return nil, fmt.Errorf("transient error")
	}
	return io.NopCloser(bytes.NewReader(r.data)), nil
}

func TestUploadLogsWithGetter_Success(t *testing.T) {
	base := t.TempDir()
	store, err := NewFileSystemLogStore(base)
	require.NoError(t, err)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			UID:       types.UID("uid-1"),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{Name: "main"}},
		},
	}

	getter := &fakeGetter{stream: &fakeStreamer{data: []byte("hello world\n")}}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, store.(*fileSystemLogStore).uploadLogsWithGetter(ctx, 42, getter, pod, nil))

	// verify file written
	path := filepath.Join(base, "42", "uid-1-log.txt")
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", string(b))
}

func TestUploadLogsWithGetter_PodDeleted(t *testing.T) {
	base := t.TempDir()
	store, err := NewFileSystemLogStore(base)
	require.NoError(t, err)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			UID:       types.UID("uid-2"),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{Name: "main"}},
		},
	}

	getter := &fakeGetter{stream: &fakeStreamer{err: fmt.Errorf("not found")}}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Should not return error if pod is already deleted
	require.NoError(t, store.(*fileSystemLogStore).uploadLogsWithGetter(ctx, 43, getter, pod, nil))

	// file is created before stream open; for deleted pod it should be present and empty
	path := filepath.Join(base, "43", "uid-2-log.txt")
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, 0, len(b))
}

func TestUploadLogsWithGetter_RetryThenSuccess(t *testing.T) {
	base := t.TempDir()
	store, err := NewFileSystemLogStore(base)
	require.NoError(t, err)
	fs := store.(*fileSystemLogStore)
	fs.streamRetryCount = 4
	fs.streamOpenTimeout = 250 * time.Millisecond

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			UID:       types.UID("uid-3"),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{Name: "main"}},
		},
	}

	// stream that returns error first two times then valid data
	var attempts int32
	rstream := &retryStreamer{count: &attempts, failUntil: 2, data: []byte("retry success\n")}
	getter := &fakeGetter{stream: rstream}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	require.NoError(t, store.(*fileSystemLogStore).uploadLogsWithGetter(ctx, 44, getter, pod, nil))
	require.EqualValues(t, 3, atomic.LoadInt32(&getter.calls))

	path := filepath.Join(base, "44", "uid-3-log.txt")
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "retry success\n", string(b))
}
