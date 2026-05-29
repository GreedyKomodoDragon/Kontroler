package object

import (
	"context"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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

// RemoveFinalizer attempts to remove the provided finalizer from the named Pod.
// It performs a small retry loop on conflicts to reduce noisy "object has been
// modified" errors that can occur when the Pod is being updated concurrently by
// other controllers.
func RemoveFinalizer(clientset *kubernetes.Clientset, podName, namespace, finalizer string) error {
	var lastErr error
	for i := 0; i < 5; i++ {
		pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, v1.GetOptions{})
		if err != nil {
			// If the pod is already gone, nothing to do
			if k8serrors.IsNotFound(err) {
				return nil
			}
			return err
		}

		// Build new finalizer slice without the target finalizer
		newFinalizers := make([]string, 0, len(pod.ObjectMeta.Finalizers))
		for _, f := range pod.ObjectMeta.Finalizers {
			if f != finalizer {
				newFinalizers = append(newFinalizers, f)
			}
		}

		// If there is no change, we're done
		if len(newFinalizers) == len(pod.ObjectMeta.Finalizers) {
			// ensure the specific finalizer is absent
			found := false
			for _, f := range pod.ObjectMeta.Finalizers {
				if f == finalizer {
					found = true
					break
				}
			}
			if !found {
				return nil
			}
		}

		pod.ObjectMeta.Finalizers = newFinalizers
		_, err = clientset.CoreV1().Pods(namespace).Update(context.TODO(), pod, v1.UpdateOptions{})
		if err == nil {
			return nil
		}

		// If pod was deleted between GET and UPDATE, treat as success
		if k8serrors.IsNotFound(err) {
			return nil
		}

		// On conflict, retry with backoff
		if k8serrors.IsConflict(err) {
			lastErr = err
			backoff := time.Duration(100*(1<<i)) * time.Millisecond
			log.Log.V(1).Info("finalizer update conflict, retrying", "pod", podName, "attempt", i+1, "backoff", backoff)
			time.Sleep(backoff)
			continue
		}

		// For other errors, log and return
		log.Log.Error(err, "failed to remove finalizer", "pod", podName, "namespace", namespace)
		return err
	}

	// Return last observed error if we exhausted retries
	return lastErr
}
