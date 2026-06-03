/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// WorkerPoolSpec defines the desired state of WorkerPool
type WorkerPoolSpec struct {
	// Replicas is the desired number of worker pods
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Image is the container image to run for the worker
	// +optional
	Image string `json:"image,omitempty"`

	// Concurrency settings
	// +optional
	Concurrency *struct {
		// MaxConcurrentClaims is the max concurrent claim-processing goroutines per pod
		// +optional
		MaxConcurrentClaims *int32 `json:"maxConcurrentClaims,omitempty"`
		// ClaimBatchSize controls how many tasks to claim per DB call
		// +optional
		ClaimBatchSize *int32 `json:"claimBatchSize,omitempty"`
	} `json:"concurrency,omitempty"`

	// Lease settings
	// +optional
	Lease *struct {
		// TTLSeconds is the lease TTL in seconds
		// +optional
		TTLSeconds *int32 `json:"ttlSeconds,omitempty"`
	} `json:"lease,omitempty"`

	// PodTemplate allows customizing pod-level values like nodeSelector, tolerations, resources and serviceAccountName
	// Re-uses the local CRD-safe PodTemplateSpec defined in DAG types
	// +optional
	PodTemplate *PodTemplateSpec `json:"podTemplate,omitempty"`

	// DB secret name to mount into worker pods as env (operator will mount)
	// +optional
	DBSecretRef string `json:"dbSecretRef,omitempty"`

	// Metrics related options
	// +optional
	Metrics *struct {
		Enabled    bool   `json:"enabled,omitempty"`
		ScrapePort *int32 `json:"scrapePort,omitempty"`
	} `json:"metrics,omitempty"`

	// Graceful shutdown period in seconds
	// +optional
	GracefulShutdownSeconds *int32 `json:"gracefulShutdownSeconds,omitempty"`
}

// WorkerPoolStatus defines the observed state of WorkerPool
type WorkerPoolStatus struct {
	// Replicas is the number of desired replicas. Mirrors Deployment.spec.replicas
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// ReadyReplicas is the number of ready replicas. Mirrors Deployment.status.readyReplicas
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// LastReconcileTime is the last time the controller reconciled the resource
	// +optional
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// WorkerPool is the Schema for the workerpools API
type WorkerPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkerPoolSpec   `json:"spec,omitempty"`
	Status WorkerPoolStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// WorkerPoolList contains a list of WorkerPool
type WorkerPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkerPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WorkerPool{}, &WorkerPoolList{})
}

// DeepCopyObject implements runtime.Object for WorkerPool
func (in *WorkerPool) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

// DeepCopyObject implements runtime.Object for WorkerPoolList
func (in *WorkerPoolList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}
