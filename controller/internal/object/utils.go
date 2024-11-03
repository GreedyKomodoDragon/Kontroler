package object

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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

func removeFinalizer(clientset *kubernetes.Clientset, podName, namespace, finalizer string) error {
	// Fetch the Pod
	pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, v1.GetOptions{})
	if err != nil {
		return err
	}

	// Check if the finalizer exists, and remove it
	var finalizers []string
	for _, f := range pod.ObjectMeta.Finalizers {
		if f != finalizer {
			finalizers = append(finalizers, f)
		}
	}
	pod.ObjectMeta.Finalizers = finalizers

	// Update the Pod to save the changes
	_, err = clientset.CoreV1().Pods(namespace).Update(context.TODO(), pod, v1.UpdateOptions{})
	if err != nil {
		log.Log.Error(err, "finaliser not removed")
	}

	return err
}
