package object

import (
	"bytes"
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typesk8s "k8s.io/apimachinery/pkg/types"
)

func TestS3Upload_Multipart_UploadsMultipleParts(t *testing.T) {
	rows := []struct {
		name           string
		minPartSize    int
		sizeMultiplier int
		extraBytes     int
		expectedParts  int
	}{
		{name: "two_parts_plus_tail", minPartSize: 64 * 1024, sizeMultiplier: 2, extraBytes: 100, expectedParts: 3},
		{name: "three_parts_plus_tail", minPartSize: 32 * 1024, sizeMultiplier: 3, extraBytes: 10, expectedParts: 4},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			fake := &fakeS3Client{}
			s := &s3LogStore{
				client:            fake,
				bucketName:        aws.String("my-bucket"),
				fetching:          map[string]bool{},
				lock:              &sync.RWMutex{},
				minPartSize:       row.minPartSize,
				streamRetryCount:  1,
				streamOpenTimeout: 100 * time.Millisecond,
			}

			sz := row.minPartSize*row.sizeMultiplier + row.extraBytes
			data := bytes.Repeat([]byte("A"), sz)
			stream := &s3FakeStreamer{data: data}
			getter := &s3FakeGetter{stream: stream}

			pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns", UID: typesk8s.UID("uid-multi")}, Spec: v1.PodSpec{Containers: []v1.Container{{Name: "main"}}}}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			require.NoError(t, s.uploadLogsWithGetter(ctx, 200, getter, pod, nil))

			fake.mu.Lock()
			uploads := fake.uploadCount
			etags := append([]string(nil), fake.uploadedETags...)
			completeCalled := fake.completeCalled
			fake.mu.Unlock()

			require.Equal(t, row.expectedParts, uploads)
			require.Equal(t, row.expectedParts, len(etags))
			require.True(t, completeCalled, "expected CompleteMultipartUpload to be called")
		})
	}
}

func TestFetchingMapConcurrency(t *testing.T) {
	run := func(t *testing.T, dagRunId int, pod *v1.Pod, concurrency int, mark func() error, unlist func()) {
		var wg sync.WaitGroup
		var successCount int32

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := mark(); err == nil {
					atomic.AddInt32(&successCount, 1)
				}
			}()
		}
		wg.Wait()

		require.Equal(t, int32(1), atomic.LoadInt32(&successCount), "expected exactly one successful MarkAsFetching")

		unlist()
		require.NoError(t, mark())
		unlist()
	}

	dagRunId := 555
	pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns", UID: typesk8s.UID("uid-concurrency")}}
	concurrency := 20

	fs, err := NewFileSystemLogStore(t.TempDir())
	require.NoError(t, err)
	run(t, dagRunId, pod, concurrency, func() error { return fs.MarkAsFetching(dagRunId, pod) }, func() { fs.UnlistFetching(dagRunId, pod) })

	fake := &fakeS3Client{}
	s := &s3LogStore{
		client:            fake,
		bucketName:        aws.String("my-bucket"),
		fetching:          map[string]bool{},
		lock:              &sync.RWMutex{},
		minPartSize:       64 * 1024,
		streamRetryCount:  1,
		streamOpenTimeout: 100 * time.Millisecond,
	}
	run(t, dagRunId, pod, concurrency, func() error { return s.MarkAsFetching(dagRunId, pod) }, func() { s.UnlistFetching(dagRunId, pod) })
}
