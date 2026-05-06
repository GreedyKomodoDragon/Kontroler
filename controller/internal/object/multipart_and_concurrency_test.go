package object

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typesk8s "k8s.io/apimachinery/pkg/types"
)

func TestS3Upload_Multipart_UploadsMultipleParts(t *testing.T) {
	fake := &fakeS3Client{}
	s := &s3LogStore{client: fake, bucketName: aws.String("my-bucket"), fetching: map[string]bool{}, lock: &sync.RWMutex{}, minPartSize: 64 * 1024}

	sz := s.minPartSize*2 + 100
	data := bytes.Repeat([]byte("A"), int(sz))
	stream := &s3FakeStreamer{data: data}
	getter := &s3FakeGetter{stream: stream}

	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns", UID: typesk8s.UID("uid-multi")}, Spec: v1.PodSpec{Containers: []v1.Container{{Name: "main"}}}}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	require.NoError(t, s.uploadLogsWithGetter(ctx, 200, getter, pod, nil))

	fake.mu.Lock()
	uploads := fake.uploadCount
	fake.mu.Unlock()
	require.GreaterOrEqual(t, uploads, 2, "expected >=2 upload parts, got %d", uploads)
	require.True(t, fake.completeCalled, "expected CompleteMultipartUpload to be called")
}

func TestFetchingMapConcurrency(t *testing.T) {
	dagRunId := 555
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns", UID: typesk8s.UID("uid-concurrency")}}

	fs, err := NewFileSystemLogStore(t.TempDir())
	require.NoError(t, err)

	concurrency := 20
	results := make(chan bool, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			if err := fs.MarkAsFetching(dagRunId, pod); err == nil {
				results <- true
			} else {
				results <- false
			}
		}()
	}

	successCount := 0
	for i := 0; i < concurrency; i++ {
		if <-results {
			successCount++
		}
	}

	require.Equal(t, 1, successCount, "expected exactly one successful MarkAsFetching, got %d", successCount)

	fs.UnlistFetching(dagRunId, pod)
	require.NoError(t, fs.MarkAsFetching(dagRunId, pod))
	fs.UnlistFetching(dagRunId, pod)

	fake := &fakeS3Client{}
	s := &s3LogStore{client: fake, bucketName: aws.String("my-bucket"), fetching: map[string]bool{}, lock: &sync.RWMutex{}, minPartSize: 64 * 1024}

	results = make(chan bool, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			if err := s.MarkAsFetching(dagRunId, pod); err == nil {
				results <- true
			} else {
				results <- false
			}
		}()
	}

	successCount = 0
	for i := 0; i < concurrency; i++ {
		if <-results {
			successCount++
		}
	}

	require.Equal(t, 1, successCount, "expected exactly one successful MarkAsFetching for s3 store, got %d", successCount)

	s.UnlistFetching(dagRunId, pod)
	require.NoError(t, s.MarkAsFetching(dagRunId, pod))
	s.UnlistFetching(dagRunId, pod)
}
