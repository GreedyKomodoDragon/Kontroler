package pods

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

type PodAllocator interface {
	AllocatePod(ctx context.Context, uid types.UID, name string, imageName string, command []string, namespace string) (types.UID, error)
}

type podAllocator struct {
	clientset *kubernetes.Clientset
}

func NewPodAllocator(clientset *kubernetes.Clientset) PodAllocator {
	return &podAllocator{
		clientset: clientset,
	}
}

func (p *podAllocator) AllocatePod(ctx context.Context, uid types.UID, name string, imageName string, command []string, namespace string) (types.UID, error) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				"managed-by":                 "kubeconductor",
				"kubeconductor/schedule-uid": string(uid),
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:    name,
					Image:   imageName,
					Command: command,
				},
			},
		},
	}

	// Create the pod
	createdPod, err := p.clientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}

	return createdPod.UID, nil
}
