package kube

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type KubeClient interface {
	// GetAllCronJobCrds uses []map[string]interface{} to avoid refactoring on spec changes
	GetAllCronJobCrds() ([]map[string]interface{}, error)
}

func NewKubeClient(dynamicClient *dynamic.DynamicClient) KubeClient {
	return &kubeClient{
		dynamicClient: dynamicClient,
	}
}

type kubeClient struct {
	dynamicClient *dynamic.DynamicClient
}

func (k *kubeClient) GetAllCronJobCrds() ([]map[string]interface{}, error) {
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

	specs := []map[string]interface{}{}
	for _, instance := range instances.Items {
		// Access the spec field from the map
		spec, found, err := unstructured.NestedMap(instance.Object, "spec")
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("spec field not found in the object")
		}

		specs = append(specs, spec)
	}

	return specs, nil
}
