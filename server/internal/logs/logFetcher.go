package logs

import (
	"context"
	"fmt"
	"io"
	"kontroler-server/internal/config"
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

// NewLogFetcher creates a new LogFetcher based on the configuration
// If no configuration is provided, returns nil which indicates log fetching is disabled
func NewLogFetcher(config *config.ServerConfig) (LogFetcher, error) {
	if config == nil || config.LogStorage.Type == "" {
		return nil, nil
	}

	switch config.LogStorage.Type {
	case "s3":
		s3Endpoint := os.Getenv("S3_ENDPOINT")
		bucketName := config.LogStorage.S3BucketName

		s3Config, err := loadS3Config()
		if err != nil {
			return nil, fmt.Errorf("failed to load S3 config: %w", err)
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
			return nil, fmt.Errorf("failed to connect to S3 bucket: %w", err)
		}

		return &s3LogFetcher{
			bucketName: &bucketName,
			client:     client,
		}, nil

	case "filesystem":
		return NewFSLogFetcher(config.LogStorage.FileSystemDir)

	default:
		return nil, fmt.Errorf("unsupported log storage type: %s", config.LogStorage.Type)
	}
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
