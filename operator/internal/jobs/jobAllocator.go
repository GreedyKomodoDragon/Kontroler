package jobs

import (
	"context"
	"fmt"
	"strings"

	"github.com/GreedyKomodoDragon/KubeConductor/operator/internal/utils"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

type JobAllocator interface {
	AllocateJob(ctx context.Context, uid types.UID, name string, imageName string, command, args []string, namespace string) (types.UID, error)
}

type jobAllocator struct {
	clientset *kubernetes.Clientset
}

func NewJobAllocator(clientset *kubernetes.Clientset) JobAllocator {
	return &jobAllocator{
		clientset: clientset,
	}
}

func (p *jobAllocator) AllocateJob(ctx context.Context, uid types.UID, name string, imageName string, command, args []string, namespace string) (types.UID, error) {
	backoff := int32(0)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"managed-by": "kubeconductor",
			},
			Annotations: map[string]string{
				"kubeconductor/schedule-uid": string(uid),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoff,
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:    name,
							Image:   imageName,
							Command: command,
							Args:    args,
						},
					},
					RestartPolicy: "Never",
				},
			},
		},
	}

	// TODO: make this dynamic
	for i := 0; i < 5; i++ {
		job.ObjectMeta.Name = utils.GenerateRandomName()

		// Create the Job
		createdJob, err := p.clientset.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				// just try again with a new name
				continue
			}

			return "", err
		}

		return createdJob.UID, nil
	}

	return "", fmt.Errorf("failed to create pod due to naming collisions")
}
