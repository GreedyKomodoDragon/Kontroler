package object

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
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
}

type s3LogStore struct {
	client     *s3.Client
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
	defer removeFinalizer(clientSet, pod.Name, pod.Namespace, "kontroler/logcollection")

	req := clientSet.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{
		Follow: true,
	})

	logStream, err := req.Stream(context.TODO())
	if err != nil {
		return fmt.Errorf("error in opening stream: %v", err)
	}
	defer logStream.Close()

	objectKey := fmt.Sprintf("/%v/%s-log.txt", dagrunId, pod.UID)
	buffer := bytes.NewBuffer(nil)
	reader := bufio.NewReader(logStream)

	createOutput, err := s.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: s.bucketName,
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("error initiating multipart upload: %v", err)
	}

	uploadID := createOutput.UploadId
	var completedParts []types.CompletedPart
	var partNumber int32 = 1
	hasUploadedParts := false

	for {
		chunk := make([]byte, 1024*1024) // 1 MB read buffer
		n, readErr := reader.Read(chunk)
		if n > 0 {
			buffer.Write(chunk[:n])
		}

		// Check if buffer has enough data to upload a part
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
		if readErr != nil {
			return fmt.Errorf("error reading logs: %v", readErr)
		}

		// Wait to avoid burning CPU
		// Waiting also helps avoids the stream closing in the case when there are no logs being generated for a bit
		time.Sleep(time.Second)
	}

	// If no parts were uploaded (logs < 5 MB), abort multipart and use PutObject for small data
	if !hasUploadedParts {
		_, _ = s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   s.bucketName,
			Key:      aws.String(objectKey),
			UploadId: uploadID,
		})

		// Upload the log data as a single object instead of using a multi-part upload
		_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: s.bucketName,
			Key:    aws.String(objectKey),
			Body:   bytes.NewReader(buffer.Bytes()),
		})
		if err != nil {
			return fmt.Errorf("error uploading small log file: %v", err)
		}
		log.Log.Info("Logs successfully uploaded to S3 bucket with PutObject", "bucket", *s.bucketName, "key", objectKey)
		return nil
	}

	// Upload remaining data as the final part if multipart was used
	if buffer.Len() > 0 {
		if err := s.uploadPart(ctx, buffer, uploadID, &completedParts, partNumber, objectKey); err != nil {
			return err
		}
	}

	// Complete multipart upload
	if _, err = s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   s.bucketName,
		Key:      aws.String(objectKey),
		UploadId: uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	}); err != nil {
		return fmt.Errorf("error completing multipart upload: %v", err)
	}

	log.Log.Info("Logs successfully uploaded to S3 bucket with multipart upload", "bucket", *s.bucketName, "key", objectKey)
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
		return fmt.Errorf("error uploading part %d: %v", partNumber, err)
	}

	// Record the completed part and reset buffer
	*completedParts = append(*completedParts, types.CompletedPart{
		ETag:       uploadPartOutput.ETag,
		PartNumber: aws.Int32(partNumber),
	})
	buffer.Reset() // Clear buffer after each successful upload
	return nil
}

func (s *s3LogStore) IsFetching(dagRunId int, pod *v1.Pod) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	_, ok := s.fetching[fmt.Sprintf("%v-%s", dagRunId, pod.Name)]
	return ok
}

func (s *s3LogStore) MarkAsFetching(dagRunId int, pod *v1.Pod) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if _, ok := s.fetching[fmt.Sprintf("%v-%s", dagRunId, pod.Name)]; ok {
		return fmt.Errorf("already fetching")
	}

	s.fetching[fmt.Sprintf("%v-%s", dagRunId, pod.Name)] = true
	return nil
}

func (s *s3LogStore) UnlistFetching(dagRunId int, pod *v1.Pod) {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.fetching, fmt.Sprintf("%v-%s", dagRunId, pod.Name))
}
