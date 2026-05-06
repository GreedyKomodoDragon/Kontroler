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

func TestS3Upload_Multipart_UploadsMultipleParts_Extra(t *testing.T) {
	fake := &fakeS3Client{}
	s := &s3LogStore{client: fake, bucketName: aws.String("my-bucket"), fetching: map[string]bool{}, lock: &sync.RWMutex{}, minPartSize: 32 * 1024}

	sz := s.minPartSize*3 + 10
	data := bytes.Repeat([]byte("B"), int(sz))
	stream := &s3FakeStreamer{data: data}
	getter := &s3FakeGetter{stream: stream}

	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns", UID: typesk8s.UID("uid-extra")}, Spec: v1.PodSpec{Containers: []v1.Container{{Name: "main"}}}}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	require.NoError(t, s.uploadLogsWithGetter(ctx, 201, getter, pod, nil))

	fake.mu.Lock()
	uploads := fake.uploadCount
	etags := append([]string(nil), fake.uploadedETags...)
	fake.mu.Unlock()

	expectedParts := (int(sz) + s.minPartSize - 1) / s.minPartSize
	require.Equal(t, expectedParts, uploads)
	require.Equal(t, expectedParts, len(etags))
	require.True(t, fake.completeCalled)
}

func TestFetchingMapConcurrency_Extra(t *testing.T) {
	dagRunId := 777
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns", UID: typesk8s.UID("uid-conc-extra")}}

	fs, err := NewFileSystemLogStore(t.TempDir())
	require.NoError(t, err)

	concurrency := 30
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

	success := 0
	for i := 0; i < concurrency; i++ {
		if <-results {
			success++
		}
	}

	require.Equal(t, 1, success)
	fs.UnlistFetching(dagRunId, pod)
	require.NoError(t, fs.MarkAsFetching(dagRunId, pod))
	fs.UnlistFetching(dagRunId, pod)

	// s3 store
	fake := &fakeS3Client{}
	s := &s3LogStore{client: fake, bucketName: aws.String("my-bucket"), fetching: map[string]bool{}, lock: &sync.RWMutex{}, minPartSize: 32 * 1024}

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

	success = 0
	for i := 0; i < concurrency; i++ {
		if <-results {
			success++
		}
	}

	require.Equal(t, 1, success)
	s.UnlistFetching(dagRunId, pod)
	require.NoError(t, s.MarkAsFetching(dagRunId, pod))
	s.UnlistFetching(dagRunId, pod)
}
