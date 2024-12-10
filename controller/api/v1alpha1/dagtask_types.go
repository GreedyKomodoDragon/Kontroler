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

/*

Purpose of this API is to allow a user to define some tasks that can then
be used across multiple DAGs. Allowing re-use.

Note, a in-line task cannot be used across DAGs even if the correct ID is used.
This is done to avoid having to de-taggle DAGs as they are deleted

*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DagTaskSpec defines the desired state of DagTask
type DagTaskSpec struct {
	// +optional
	Command []string `json:"command,omitempty"`
	// +optional
	Args        []string    `json:"args,omitempty"`
	Image       string      `json:"image"`
	RunAfter    []string    `json:"runAfter,omitempty"`
	Backoff     Backoff     `json:"backoff"`
	Conditional Conditional `json:"conditional"`
	// +optional
	Parameters []string `json:"parameters,omitempty"`
	// +optional
	PodTemplate *PodTemplateSpec `json:"podTemplate,omitempty"`
	// +optional
	Script string `json:"script,omitempty"`
	// Used to select the image that is used to push to script into the pod
	// +optional
	ScriptInjectorImage string `json:"scriptInjectorImage,omitempty"`
	// Using reference to existing pre-created task - cannot reference another in-line task
	// +optional
}

// DagTaskStatus defines the observed state of DagTask
type DagTaskStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DagTask is the Schema for the dagtasks API
type DagTask struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DagTaskSpec   `json:"spec,omitempty"`
	Status DagTaskStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DagTaskList contains a list of DagTask
type DagTaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DagTask `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DagTask{}, &DagTaskList{})
}
