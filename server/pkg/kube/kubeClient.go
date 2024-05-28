package kube

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type ScheduleCrds struct {
	APIVersion string      `json:"apiVersion"`
	Kind       string      `json:"kind"`
	Name       string      `json:"name"`
	Spec       interface{} `json:"spec"`
}

type KubeClient interface {
	// GetAllCronJobCrds uses []map[string]interface{} to avoid refactoring on spec changes
	GetAllCronJobCrds() ([]*ScheduleCrds, error)
}

func NewKubeClient(dynamicClient *dynamic.DynamicClient) KubeClient {
	return &kubeClient{
		dynamicClient: dynamicClient,
	}
}

type kubeClient struct {
	dynamicClient *dynamic.DynamicClient
}

func (k *kubeClient) GetAllCronJobCrds() ([]*ScheduleCrds, error) {
	// Define the CRD group and version
	grv := schema.GroupVersionResource{
		Group:    "kubeconductor.greedykomodo",
		Version:  "v1alpha1",
		Resource: "schedules",
	}

	// Get the list of instances of the CRD
	instances, err := k.dynamicClient.Resource(grv).Namespace("default").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	items := make([]*ScheduleCrds, len(instances.Items))
	for i, instance := range instances.Items {
		instance.GetKind()
		// Access the spec field from the map
		spec, found, err := unstructured.NestedMap(instance.Object, "spec")
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("spec field not found in the object")
		}

		items[i] = &ScheduleCrds{
			APIVersion: instance.GetAPIVersion(),
			Kind:       instance.GetKind(),
			Name:       instance.GetName(),
			Spec:       spec,
		}
	}

	return items, nil
}
