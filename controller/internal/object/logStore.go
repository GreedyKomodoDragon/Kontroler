package object

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	minPartSize int = 5 * 1024 * 1024
)

type LogStore interface {
	IsFetching(dagRunId int, pod *v1.Pod) bool
	MarkAsFetching(dagRunId int, pod *v1.Pod) error
	UnlistFetching(dagRunId int, pod *v1.Pod)
	UploadLogs(ctx context.Context, dagrunId int, clientSet *kubernetes.Clientset, pod *v1.Pod) error
	DeleteLogs(ctx context.Context, dagrunId int) error
}

type s3Client interface {
	CreateMultipartUpload(ctx context.Context, params *s3.CreateMultipartUploadInput, opts ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error)
	UploadPart(ctx context.Context, params *s3.UploadPartInput, opts ...func(*s3.Options)) (*s3.UploadPartOutput, error)
	CompleteMultipartUpload(ctx context.Context, params *s3.CompleteMultipartUploadInput, opts ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	AbortMultipartUpload(ctx context.Context, params *s3.AbortMultipartUploadInput, opts ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error)
	DeleteObjects(ctx context.Context, params *s3.DeleteObjectsInput, opts ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error)
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, opts ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

type s3LogStore struct {
	client     s3Client
	bucketName *string
	fetching   map[string]bool
	lock       *sync.RWMutex
}

func NewLogStore() (LogStore, error) {
	s3Endpoint := os.Getenv("S3_ENDPOINT")
	bucketName := os.Getenv("S3_BUCKETNAME")

	// If there is no bucket we do not send logs
	if bucketName == "" {
		return nil, nil
	}

	s3Config, err := loadS3Config()
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(s3Config, func(o *s3.Options) {
		if s3Endpoint != "" {
			o.BaseEndpoint = aws.String(s3Endpoint)
		}

		// Better handles Minio with Kubernetes DNS
		o.UsePathStyle = true
	})

	// HeadBucket to check if the bucket exists and the connection is valid
	if _, err = client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: &bucketName,
	}); err != nil {
		return nil, err
	}

	return &s3LogStore{
		client:     client,
		bucketName: &bucketName,
		lock:       &sync.RWMutex{},
		fetching:   map[string]bool{},
	}, nil
}

func (s *s3LogStore) UploadLogs(ctx context.Context, dagrunId int, clientSet *kubernetes.Clientset, pod *v1.Pod) error {
	getter := &coreV1PodLogsGetter{client: clientSet, ns: pod.Namespace}
	return s.uploadLogsWithGetter(ctx, dagrunId, getter, pod, func() {
		if err := RemoveFinalizer(clientSet, pod.Name, pod.Namespace, "kontroler/logcollection"); err != nil {
			log.Log.Error(err, "error removing finalizer", "pod", pod.Name, "namespace", pod.Namespace)
		}
	})
}

func (s *s3LogStore) uploadLogsWithGetter(ctx context.Context, dagrunId int, getter podLogsGetter, pod *v1.Pod, finaliserCleanup func()) error {
	if finaliserCleanup != nil {
		defer finaliserCleanup()
	}

	objectKey := fmt.Sprintf("%v/%s-log.txt", dagrunId, pod.UID)
	buffer := bytes.NewBuffer(nil)

	// Attempt to open the log stream with a short timeout and a couple of retries.
	var logStream io.ReadCloser
	var cancelStream context.CancelFunc
	var lastErr error
	for i := 0; i < 3; i++ {
		openCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		req := getter.GetLogs(pod.Name, &v1.PodLogOptions{
			Follow:    true,
			Container: pod.Spec.Containers[0].Name,
		})

		logStream, lastErr = req.Stream(openCtx)
		if lastErr == nil {
			cancelStream = cancel
			break
		}

		cancel()

		if strings.Contains(lastErr.Error(), "not found") {
			log.Log.Info("pod already deleted, cannot fetch logs", "pod", pod.Name)
			return nil
		}

		if i < 2 {
			time.Sleep(time.Duration(1<<i) * time.Second)
			continue
		}

		return fmt.Errorf("error in opening stream: %w", lastErr)
	}

	if logStream == nil {
		return fmt.Errorf("failed to open log stream: %v", lastErr)
	}
	defer func() {
		if cancelStream != nil {
			cancelStream()
		}
	}()
	defer logStream.Close()

	reader := bufio.NewReader(logStream)

	createOutput, err := s.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: s.bucketName,
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("error initiating multipart upload: %w", err)
	}

	uploadID := createOutput.UploadId
	var completedParts []types.CompletedPart
	var partNumber int32 = 1
	hasUploadedParts := false

	// Ensure cleanup of partial upload on any error
	defer func() {
		if !hasUploadedParts || err != nil {
			s.cleanupPartialUpload(ctx, objectKey, uploadID)
		}
	}()

	for {
		chunk := make([]byte, 1024*1024) // 1 MB read buffer
		n, readErr := reader.Read(chunk)

		if readErr != nil && readErr != io.EOF {
			isPodDeleted := strings.Contains(readErr.Error(), "not found") ||
				strings.Contains(readErr.Error(), "connection refused") ||
				strings.Contains(readErr.Error(), "has been terminated")

			if isPodDeleted {
				log.Log.Info("pod deleted while reading logs", "pod", pod.Name)
				s.cleanupPartialUpload(ctx, objectKey, uploadID)
				return nil
			}
			return fmt.Errorf("error reading logs: %w", readErr)
		}

		if n > 0 {
			buffer.Write(chunk[:n])
		}

		if buffer.Len() >= minPartSize {
			hasUploadedParts = true
			if err := s.uploadPart(ctx, buffer, uploadID, &completedParts, partNumber, objectKey); err != nil {
				return err
			}
			partNumber++
		}

		if readErr == io.EOF {
			break
		}

		time.Sleep(time.Second)
	}

	// Handle remaining data
	if buffer.Len() > 0 {
		if buffer.Len() < minPartSize && !hasUploadedParts {
			// Upload small file as single object
			_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
				Bucket: s.bucketName,
				Key:    aws.String(objectKey),
				Body:   bytes.NewReader(buffer.Bytes()),
			})
			if err != nil {
				return fmt.Errorf("error uploading small log file: %w", err)
			}
			log.Log.Info("Logs successfully uploaded to S3 bucket with PutObject", "bucket", *s.bucketName, "key", objectKey)
			return nil
		}

		// Upload final part for multipart upload
		if err := s.uploadPart(ctx, buffer, uploadID, &completedParts, partNumber, objectKey); err != nil {
			return err
		}
	}

	// Complete multipart upload if we have parts
	if hasUploadedParts {
		if _, err = s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
			Bucket:   s.bucketName,
			Key:      aws.String(objectKey),
			UploadId: uploadID,
			MultipartUpload: &types.CompletedMultipartUpload{
				Parts: completedParts,
			},
		}); err != nil {
			return fmt.Errorf("error completing multipart upload: %w", err)
		}
		log.Log.Info("Logs successfully uploaded to S3 bucket with multipart upload", "bucket", *s.bucketName, "key", objectKey)
	}

	return nil
}

// Helper function to upload a buffered part
func (s *s3LogStore) uploadPart(ctx context.Context, buffer *bytes.Buffer, uploadID *string, completedParts *[]types.CompletedPart, partNumber int32, objectKey string) error {
	uploadPartOutput, err := s.client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     s.bucketName,
		Key:        aws.String(objectKey),
		PartNumber: aws.Int32(partNumber),
		UploadId:   uploadID,
		Body:       bytes.NewReader(buffer.Bytes()),
	})
	if err != nil {
		_, _ = s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   s.bucketName,
			Key:      aws.String(objectKey),
			UploadId: uploadID,
		})
		return fmt.Errorf("error uploading part %d: %w", partNumber, err)
	}

	// Record the completed part and reset buffer
	*completedParts = append(*completedParts, types.CompletedPart{
		ETag:       uploadPartOutput.ETag,
		PartNumber: aws.Int32(partNumber),
	})
	buffer.Reset() // Clear buffer after each successful upload
	return nil
}

func (s *s3LogStore) cleanupPartialUpload(ctx context.Context, objectKey string, uploadID *string) {
	if _, err := s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   s.bucketName,
		Key:      aws.String(objectKey),
		UploadId: uploadID,
	}); err != nil {
		log.Log.Error(err, "failed to abort multipart upload", "bucket", *s.bucketName, "key", objectKey)
	}
}

func (s *s3LogStore) IsFetching(dagRunId int, pod *v1.Pod) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	// Use pod UID to avoid collisions when pod names are reused
	_, ok := s.fetching[fmt.Sprintf("%v-%s", dagRunId, pod.UID)]
	return ok
}

func (s *s3LogStore) MarkAsFetching(dagRunId int, pod *v1.Pod) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	key := fmt.Sprintf("%v-%s", dagRunId, pod.UID)
	if _, ok := s.fetching[key]; ok {
		return fmt.Errorf("already fetching")
	}

	s.fetching[key] = true
	return nil
}

func (s *s3LogStore) UnlistFetching(dagRunId int, pod *v1.Pod) {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.fetching, fmt.Sprintf("%v-%s", dagRunId, pod.UID))
}

func (s *s3LogStore) DeleteLogs(ctx context.Context, dagrunId int) error {
	prefix := fmt.Sprintf("%v/", dagrunId)
	var objectIds []types.ObjectIdentifier
	ptrTrue := true

	// List all objects using ListObjectsV2 with continuation
	var cont *string
	for {
		output, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            s.bucketName,
			Prefix:            aws.String(prefix),
			ContinuationToken: cont,
		})
		if err != nil {
			return fmt.Errorf("error listing objects: %w", err)
		}

		for _, object := range output.Contents {
			objectIds = append(objectIds, types.ObjectIdentifier{Key: object.Key})
		}

		if output.IsTruncated == nil || !*output.IsTruncated {
			break
		}
		cont = output.NextContinuationToken
	}

	if len(objectIds) == 0 {
		return nil
	}

	// Delete objects in batches of 1000 (S3's maximum batch size)
	const maxBatchSize = 1000
	for i := 0; i < len(objectIds); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(objectIds) {
			end = len(objectIds)
		}

		batch := objectIds[i:end]
		_, err := s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: s.bucketName,
			Delete: &types.Delete{
				Objects: batch,
				Quiet:   &ptrTrue,
			},
		})
		if err != nil {
			return fmt.Errorf("error deleting objects batch: %w", err)
		}
	}

	log.Log.Info("Successfully deleted all logs", "bucket", *s.bucketName, "prefix", prefix, "count", len(objectIds))
	return nil
}
