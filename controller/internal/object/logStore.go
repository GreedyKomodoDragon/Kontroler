package object

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"k8s.io/client-go/rest"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	partSize int = 5 * 1024 * 1024
)

type LogStore interface {
	UploadLogs(ctx context.Context, podName string, req *rest.Request) error
}

type s3LogStore struct {
	client     *s3.Client
	bucketName *string
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
	}, nil
}

func (s *s3LogStore) UploadLogs(ctx context.Context, podName string, req *rest.Request) error {
	logStream, err := req.Stream(context.TODO())
	if err != nil {
		return fmt.Errorf("error in opening stream: %v", err)
	}
	defer logStream.Close()

	objectKey := fmt.Sprintf("%s-log.txt", podName)
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
