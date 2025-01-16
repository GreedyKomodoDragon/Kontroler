package logs

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Currently only fitted to AWS/S3 so inteface will likely change
type LogFetcher interface {
	// Fetch only a certain byte range
	RangeFetchLogs(logKey *string, start int64, end int64) (io.ReadCloser, error)
	// Fetch the whole file
	FetchLogs(logKey *string) (io.ReadCloser, error)
	// Checks if the file we want actually exists, also return the filesize back
	LogFileExists(logFileKey *string) (bool, int64, error)
}

// NewLogFetcher will currently create an S3 API log fetcher
// We return nil as we assume it has not been enabled so you must check for nil
func NewLogFetcher(bucketName string) (LogFetcher, error) {
	if bucketName == "" {
		return nil, nil
	}

	s3Endpoint := os.Getenv("S3_ENDPOINT")

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

	return &s3LogFetcher{
		bucketName: &bucketName,
		client:     client,
	}, nil
}

type s3LogFetcher struct {
	client     *s3.Client
	bucketName *string
}

func (s *s3LogFetcher) RangeFetchLogs(logKey *string, start int64, end int64) (io.ReadCloser, error) {
	// Fetch the requested byte range from S3
	getObjectInput := &s3.GetObjectInput{
		Bucket: s.bucketName,
		Key:    logKey,
		Range:  aws.String(fmt.Sprintf("bytes=%d-%d", start, end)),
	}

	getObjectOutput, err := s.client.GetObject(context.TODO(), getObjectInput)
	if err != nil {
		return nil, err
	}

	return getObjectOutput.Body, nil
}

func (s *s3LogFetcher) FetchLogs(logKey *string) (io.ReadCloser, error) {
	// If no range, fetch and serve the full file
	getObjectInput := &s3.GetObjectInput{
		Bucket: s.bucketName,
		Key:    logKey,
	}

	getObjectOutput, err := s.client.GetObject(context.TODO(), getObjectInput)
	if err != nil {
		return nil, err
	}

	return getObjectOutput.Body, nil
}

func (s *s3LogFetcher) LogFileExists(logFileKey *string) (bool, int64, error) {
	headObjectInput := &s3.HeadObjectInput{
		Bucket: s.bucketName,
		Key:    logFileKey,
	}

	headObjectOutput, err := s.client.HeadObject(context.TODO(), headObjectInput)
	if err != nil {
		// TODO: Make this more fine-grained
		return false, 0, nil
	}

	fileSize := headObjectOutput.ContentLength
	if fileSize == nil || *fileSize == 0 {
		// No useable file, its empty
		return false, 0, nil
	}

	return true, *fileSize, nil
}
