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
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type ParameterSpec struct {
	Name string `json:"name"`
	// +optional
	Value string `json:"value,omitempty"`
	// +optional
	FromSecret string `json:"fromSecret,omitempty"`
}

// DagRunSpec defines the desired state of DagRun
type DagRunSpec struct {
	DagName string `json:"dagName"`
	// +optional
	Parameters []ParameterSpec `json:"parameters"`
}

// DagRunStatus defines the observed state of DagRun
type DagRunStatus struct {
	DagRunId int `json:"dagRunId"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DagRun is the Schema for the dagruns API
type DagRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DagRunSpec   `json:"spec,omitempty"`
	Status DagRunStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DagRunList contains a list of DagRun
type DagRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DagRun `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DagRun{}, &DagRunList{})
}
