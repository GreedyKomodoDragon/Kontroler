package object

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	partSize int = 5 * 1024 * 1024
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

	if bucketName == "" {
		return nil, fmt.Errorf("S3_BUCKETNAME must be set to a non-empty value")
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

	req := clientSet.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{})

	logStream, err := req.Stream(context.TODO())
	if err != nil {
		return fmt.Errorf("error in opening stream: %v", err)
	}
	defer logStream.Close()

	objectKey := fmt.Sprintf("/%v/%s-log.txt", dagrunId, pod.Name)
	reader := bufio.NewReader(logStream)

	createOutput, err := s.client.CreateMultipartUpload(context.TODO(), &s3.CreateMultipartUploadInput{
		Bucket: s.bucketName,
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("error initiating multipart upload: %v", err)
	}

	uploadID := createOutput.UploadId
	var completedParts []types.CompletedPart
	var partNumber int32 = 1

	buf := make([]byte, partSize)

	// Upload in chunks
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			uploadPartOutput, err := s.client.UploadPart(context.TODO(), &s3.UploadPartInput{
				Bucket:     s.bucketName,
				Key:        aws.String(objectKey),
				PartNumber: aws.Int32(partNumber),
				UploadId:   uploadID,
				Body:       bytes.NewReader(buf[:n]),
			})
			if err != nil {
				_, _errAbort := s.client.AbortMultipartUpload(context.TODO(), &s3.AbortMultipartUploadInput{
					Bucket:   s.bucketName,
					Key:      aws.String(objectKey),
					UploadId: uploadID,
				})
				return fmt.Errorf("error uploading part %d: %v, abort err: %v", partNumber, err, _errAbort)
			}

			// Record the completed part
			completedParts = append(completedParts, types.CompletedPart{
				ETag:       uploadPartOutput.ETag,
				PartNumber: aws.Int32(partNumber),
			})

			partNumber++
		}

		// Check if we reached the end of the stream
		if readErr == io.EOF {
			break
		} else if readErr != nil {
			return fmt.Errorf("error reading logs: %v", readErr)
		}
	}

	// Gracefully abort if nothing has been uploaded
	if len(completedParts) == 0 {
		if _, err := s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   s.bucketName,
			Key:      aws.String(objectKey),
			UploadId: uploadID,
		}); err != nil {
			return fmt.Errorf("no parts uploaded, abort error: %v", err)
		}

		return nil
	}

	if _, err = s.client.CompleteMultipartUpload(context.TODO(), &s3.CompleteMultipartUploadInput{
		Bucket:   s.bucketName,
		Key:      aws.String(objectKey),
		UploadId: uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	}); err != nil {
		return fmt.Errorf("error completing multipart upload: %v", err)
	}

	log.Log.Info("Logs successfully uploaded to S3 bucket", "bucket", *s.bucketName, "key", objectKey)
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
