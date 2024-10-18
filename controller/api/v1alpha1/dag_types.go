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
	"encoding/json"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DagParameterSpec struct {
	Name string `json:"name"`
	// +optional
	DefaultValue string `json:"defaultValue,omitempty"`
	// +optional
	DefaultFromSecret string `json:"defaultFromSecret,omitempty"`
}

// PodTemplateSpec defines the template for the pod of a task
type PodTemplateSpec struct {
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	// +optional
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
	// +optional
	AutomountServiceAccountToken *bool `json:"automountServiceAccountToken,omitempty"`
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

func (p PodTemplateSpec) Serialize() (string, error) {
	jsonData, err := json.Marshal(p)
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

// TaskSpec defines the structure of a task in the DAG
type TaskSpec struct {
	Name string `json:"name"`
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
}

// Backoff defines the backoff strategy for a task
type Backoff struct {
	Limit int `json:"limit"`
}

// Conditional defines the conditional execution parameters
type Conditional struct {
	Enabled    bool  `json:"enabled"`
	RetryCodes []int `json:"retryCodes"`
}

// DAGSpec defines the desired state of DAG
type DAGSpec struct {
	// +optional
	Schedule string     `json:"schedule"`
	Task     []TaskSpec `json:"task"`
	// +optional
	Parameters []DagParameterSpec `json:"parameters,omitempty"`
}

// DAGStatus defines the observed state of DAG
type DAGStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DAG is the Schema for the dags API
type DAG struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DAGSpec   `json:"spec,omitempty"`
	Status DAGStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DAGList contains a list of DAG
type DAGList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DAG `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DAG{}, &DAGList{})
}

// ValidateDAG checks if the DAG is valid.
func (dag *DAG) ValidateDAG() error {
	if err := dag.checkFieldsFilled(); err != nil {
		return err
	}

	if err := dag.checkParameters(); err != nil {
		return err
	}
	if err := dag.checkNoCycles(); err != nil {
		return err
	}
	if err := dag.checkRunAfterTasksExist(); err != nil {
		return err
	}
	if err := dag.checkAllTasksConnected(); err != nil {
		return err
	}
	if err := dag.checkStartingTask(); err != nil {
		return err
	}

	return nil
}

// checkFieldsFilled ensures all necessary fields in the DAG are filled.
func (dag *DAG) checkFieldsFilled() error {
	taskNames := make(map[string]bool)

	for _, task := range dag.Spec.Task {
		if task.Name == "" {
			return errors.New("task name must be specified")
		}

		// Either have a script or command
		if len(task.Script) == 0 && len(task.Command) == 0 {
			return errors.New("must provide a script or a command")
		}

		if task.Image == "" {
			return errors.New("task image must be specified")
		}

		if _, exists := taskNames[task.Name]; exists {
			return errors.New("duplicate task name: " + task.Name)
		}

		taskNames[task.Name] = true
	}

	return nil
}

// checkNoCycles ensures there are no cyclic dependencies in the tasks.
func (dag *DAG) checkNoCycles() error {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var visit func(string) bool
	visit = func(name string) bool {
		if recStack[name] {
			return true // cycle detected
		}
		if visited[name] {
			return false
		}
		visited[name] = true
		recStack[name] = true

		for _, task := range dag.Spec.Task {
			if task.Name == name {
				for _, dep := range task.RunAfter {
					if visit(dep) {
						return true
					}
				}
			}
		}
		recStack[name] = false
		return false
	}

	for _, task := range dag.Spec.Task {
		if visit(task.Name) {
			return errors.New("cyclic dependency detected")
		}
	}
	return nil
}

// checkRunAfterTasksExist ensures that all runAfter references point to existing tasks.
func (dag *DAG) checkRunAfterTasksExist() error {
	taskMap := make(map[string]bool)
	for _, task := range dag.Spec.Task {
		taskMap[task.Name] = true
	}
	for _, task := range dag.Spec.Task {
		for _, dep := range task.RunAfter {
			if !taskMap[dep] {
				return fmt.Errorf("task %s has runAfter dependency on non-existent task %s", task.Name, dep)
			}
		}
	}
	return nil
}

// checkAllTasksConnected ensures that all tasks are part of a single connected component.
func (dag *DAG) checkAllTasksConnected() error {
	adjList := make(map[string][]string)
	taskMap := make(map[string]bool)
	for _, task := range dag.Spec.Task {
		taskMap[task.Name] = true
		adjList[task.Name] = append(adjList[task.Name], task.RunAfter...)
		for _, dep := range task.RunAfter {
			adjList[dep] = append(adjList[dep], task.Name)
		}
	}

	visited := make(map[string]bool)
	var dfs func(string)
	dfs = func(name string) {
		visited[name] = true
		for _, neighbor := range adjList[name] {
			if !visited[neighbor] {
				dfs(neighbor)
			}
		}
	}

	var startNode string
	// Get first node
	for name := range taskMap {
		startNode = name
		break
	}

	dfs(startNode)

	for task := range taskMap {
		if !visited[task] {
			return errors.New("not all tasks are connected")
		}
	}
	return nil
}

// checkStartingTask ensures there is at least one task that has no runAfter dependencies.
func (dag *DAG) checkStartingTask() error {
	for _, task := range dag.Spec.Task {
		if len(task.RunAfter) == 0 {
			return nil
		}
	}
	return errors.New("no starting task found (a task with no runAfter dependencies)")
}

// checkParameters ensures there is at least one task that has no runAfter dependencies.
func (dag *DAG) checkParameters() error {
	paramsMap := map[string]bool{}
	for _, value := range dag.Spec.Parameters {
		if value.Name == "" {
			return fmt.Errorf("parameter has an empty name")
		}

		if value.DefaultValue == "" && value.DefaultFromSecret == "" {
			return fmt.Errorf("parameter does not provide defaultValue or defaultFromSecret")
		}

		if value.DefaultValue != "" && value.DefaultFromSecret != "" {
			return fmt.Errorf("parameter does not provide defaultValue or defaultFromSecret")
		}

		paramsMap[value.Name] = true
	}

	for _, task := range dag.Spec.Task {
		for _, value := range task.Parameters {
			if _, ok := paramsMap[value]; !ok {
				return fmt.Errorf("parameter selected in task does not exist")
			}
		}
	}

	return nil
}
