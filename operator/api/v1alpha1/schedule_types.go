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

// Important: Run "make" to regenerate code after modifying this file

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ScheduleSpec defines the desired state of Schedule
type ScheduleSpec struct {
	// Cron schedule for the job
	CronSchedule string `json:"cronSchedule,omitempty"`

	// Name of the Docker image to use for the job
	ImageName string `json:"imageName,omitempty"`

	// Command to run on the container
	Command []string `json:"command,omitempty"`

	// Args to pass to container
	Args []string `json:"args,omitempty"`

	// BackoffLimit determines how many times to retry
	BackoffLimit uint64 `json:"backoffLimit,omitempty"`

	// RetryCodes are used to determine if job should be restarted
	RetryCodes []int32 `json:"retryCodes,omitempty"`
}

// ScheduleStatus defines the observed state of Schedule
type ScheduleStatus struct {
	DeploymentStatus metav1.Condition `json:"deploymentStatus,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Schedule is the Schema for the schedules API
type Schedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScheduleSpec   `json:"spec,omitempty"`
	Status ScheduleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ScheduleList contains a list of Schedule
type ScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Schedule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Schedule{}, &ScheduleList{})
}
