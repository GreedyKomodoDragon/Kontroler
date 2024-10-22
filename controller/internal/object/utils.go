package object

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

func loadS3Config() (aws.Config, error) {
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	sessionToken := os.Getenv("AWS_SESSION_TOKEN")

	if accessKey != "" && secretKey != "" {
		log.Log.Info("Using explicit S3 credentials from environment variables.")
		return config.LoadDefaultConfig(context.TODO(),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, sessionToken)),
		)
	}

	log.Log.Info("Using default S3 credentials provider chain (IRSA, Instance Profile, etc.).")
	return config.LoadDefaultConfig(context.TODO())
}
