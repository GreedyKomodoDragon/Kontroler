package object

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	v1 "k8s.io/api/core/v1"
)

// fake S3 client used by tests
type fakeS3Client struct {
	putBody          []byte
	putKey           string
	uploadShouldFail bool
	abortCalled      bool
	completeCalled   bool
	uploadCount      int
	uploadedETags    []string
	uploadedBodies   [][]byte
	listObjectsPages [][]types.Object
	mu               sync.Mutex
}

func (f *fakeS3Client) CreateMultipartUpload(ctx context.Context, params *s3.CreateMultipartUploadInput, opts ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error) {
	return &s3.CreateMultipartUploadOutput{UploadId: aws.String("fake-upload-id")}, nil
}

func (f *fakeS3Client) UploadPart(ctx context.Context, params *s3.UploadPartInput, opts ...func(*s3.Options)) (*s3.UploadPartOutput, error) {
	b, _ := io.ReadAll(params.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.uploadCount++
	f.uploadedBodies = append(f.uploadedBodies, b)
	etag := fmt.Sprintf("etag-%d", *params.PartNumber)
	f.uploadedETags = append(f.uploadedETags, etag)
	if f.uploadShouldFail {
		return nil, io.ErrUnexpectedEOF
	}
	return &s3.UploadPartOutput{ETag: aws.String(etag)}, nil
}

func (f *fakeS3Client) CompleteMultipartUpload(ctx context.Context, params *s3.CompleteMultipartUploadInput, opts ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error) {
	f.mu.Lock()
	f.completeCalled = true
	f.mu.Unlock()
	return &s3.CompleteMultipartUploadOutput{}, nil
}

func (f *fakeS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	b, _ := io.ReadAll(params.Body)
	f.mu.Lock()
	f.putBody = b
	f.putKey = *params.Key
	f.mu.Unlock()
	return &s3.PutObjectOutput{}, nil
}

func (f *fakeS3Client) AbortMultipartUpload(ctx context.Context, params *s3.AbortMultipartUploadInput, opts ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error) {
	f.mu.Lock()
	f.abortCalled = true
	f.mu.Unlock()
	return &s3.AbortMultipartUploadOutput{}, nil
}

func (f *fakeS3Client) DeleteObjects(ctx context.Context, params *s3.DeleteObjectsInput, opts ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
	return &s3.DeleteObjectsOutput{}, nil
}

func (f *fakeS3Client) ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, opts ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	if len(f.listObjectsPages) == 0 {
		return &s3.ListObjectsV2Output{Contents: []types.Object{}}, nil
	}

	page := 0
	if params.ContinuationToken != nil {
		_, _ = fmt.Sscanf(*params.ContinuationToken, "page-%d", &page)
	}
	if page < 0 || page >= len(f.listObjectsPages) {
		return &s3.ListObjectsV2Output{Contents: []types.Object{}}, nil
	}

	isTruncated := page < len(f.listObjectsPages)-1
	output := &s3.ListObjectsV2Output{
		Contents:    f.listObjectsPages[page],
		IsTruncated: aws.Bool(isTruncated),
	}
	if isTruncated {
		output.NextContinuationToken = aws.String(fmt.Sprintf("page-%d", page+1))
	}
	return output, nil
}

// test stream/getter types used by multiple tests
type s3FakeStreamer struct {
	data []byte
	err  error
}

func (f *s3FakeStreamer) Stream(ctx context.Context) (io.ReadCloser, error) {
	if f.err != nil {
		return nil, f.err
	}
	return io.NopCloser(bytes.NewReader(f.data)), nil
}

type s3FakeGetter struct{ stream podLogStreamer }

func (f *s3FakeGetter) GetLogs(name string, opts *v1.PodLogOptions) podLogStreamer { return f.stream }
