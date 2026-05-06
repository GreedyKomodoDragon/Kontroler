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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
// Note: use CRD-safe local types to avoid controller-gen emitting $ref to core/v1
// and causing forbidden $ref entries in the CRD schema.
type PodTemplateSpec struct {
	// +optional
	Volumes []Volume `json:"volumes,omitempty"`
	// +optional
	VolumeMounts []VolumeMount `json:"volumeMounts,omitempty"`
	// +optional
	ImagePullSecrets []LocalObjectReference `json:"imagePullSecrets,omitempty"`
	// +optional
	SecurityContext *PodSecurityContext `json:"securityContext,omitempty"`
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// +optional
	Tolerations []Toleration `json:"tolerations,omitempty"`
	// +optional
	Affinity *Affinity `json:"affinity,omitempty"`
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
	// ActiveDeadlineSeconds is how long the pod will last for (basically a time-limit)
	// Will Start the moment the pod is scheduled for a node, will count down even if in pending state
	// +optional
	ActiveDeadlineSeconds *int64 `json:"activeDeadlineSeconds,omitempty"`
	// +optional
	AutomountServiceAccountToken *bool `json:"automountServiceAccountToken,omitempty"`
	// +optional
	Resources *ResourceRequirements `json:"resources,omitempty"`
}

// Local CRD-safe types mirroring the fields we need from core/v1
type EmptyDirVolumeSource struct{}

type PersistentVolumeClaimVolumeSource struct {
	ClaimName string `json:"claimName"`
}

type Volume struct {
	Name string `json:"name"`
	// Only support EmptyDir and PersistentVolumeClaim for now
	EmptyDir              *EmptyDirVolumeSource              `json:"emptyDir,omitempty"`
	PersistentVolumeClaim *PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty"`
}

type VolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
	ReadOnly  bool   `json:"readOnly,omitempty"`
}

type LocalObjectReference struct {
	Name string `json:"name"`
}

type PodSecurityContext struct {
	// minimal mapping — expand if needed
	FSGroup *int64 `json:"fsGroup,omitempty"`
}

type Toleration struct {
	Key      string `json:"key,omitempty"`
	Operator string `json:"operator,omitempty"`
	Value    string `json:"value,omitempty"`
	Effect   string `json:"effect,omitempty"`
	// +optional
	TolerationSeconds *int64 `json:"tolerationSeconds,omitempty"`
}

type Affinity struct {
	// keep opaque for brevity; map to corev1.Affinity in conversion
	NodeAffinity    *apiextensionsv1.JSON `json:"nodeAffinity,omitempty"`
	PodAffinity     *apiextensionsv1.JSON `json:"podAffinity,omitempty"`
	PodAntiAffinity *apiextensionsv1.JSON `json:"podAntiAffinity,omitempty"`
}

type ResourceRequirements struct {
	// simplified: requests/limits maps
	Limits   map[string]string `json:"limits,omitempty"`
	Requests map[string]string `json:"requests,omitempty"`
}

// PVC and Workspace local types
type PVC struct {
	AccessModes []string              `json:"accessModes,omitempty"`
	Selector    *metav1.LabelSelector `json:"selector,omitempty"`
	// simplified resources
	Resources        *ResourceRequirements `json:"resources,omitempty"`
	StorageClassName *string               `json:"storageClassName,omitempty"`
	VolumeMode       *string               `json:"volumeMode,omitempty"`
}

type Workspace struct {
	Enabled bool `json:"enable"`
	PvcSpec PVC  `json:"pvc"`
}

func (p PodTemplateSpec) Serialize() (string, error) {
	jsonData, err := json.Marshal(p)
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

// Conversion helper: convert this CRD-safe PodTemplateSpec into k8s core types
func (p *PodTemplateSpec) ToK8sParts() (volumes []corev1.Volume, volumeMounts []corev1.VolumeMount, imagePullSecrets []corev1.LocalObjectReference, securityContext *corev1.PodSecurityContext, tolerations []corev1.Toleration, affinity *corev1.Affinity, resources *corev1.ResourceRequirements) {
	// volumes
	for _, v := range p.Volumes {
		var vol corev1.Volume
		vol.Name = v.Name
		if v.EmptyDir != nil {
			vol.VolumeSource = corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}
		} else if v.PersistentVolumeClaim != nil {
			vol.VolumeSource = corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: v.PersistentVolumeClaim.ClaimName}}
		}
		volumes = append(volumes, vol)
	}

	// volumeMounts
	for _, vm := range p.VolumeMounts {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{Name: vm.Name, MountPath: vm.MountPath, ReadOnly: vm.ReadOnly})
	}

	// imagePullSecrets
	for _, s := range p.ImagePullSecrets {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: s.Name})
	}

	// securityContext (limited)
	if p.SecurityContext != nil {
		securityContext = &corev1.PodSecurityContext{}
		if p.SecurityContext.FSGroup != nil {
			securityContext.FSGroup = p.SecurityContext.FSGroup
		}
	}

	// tolerations
	for _, t := range p.Tolerations {
		tolerations = append(tolerations, corev1.Toleration{Key: t.Key, Operator: corev1.TolerationOperator(t.Operator), Value: t.Value, Effect: corev1.TaintEffect(t.Effect), TolerationSeconds: t.TolerationSeconds})
	}

	// affinity: best-effort (not converting complex structures)
	if p.Affinity != nil {
		// This is an approximation: we marshal/unmarshal via JSON to/from core type
		b, _ := json.Marshal(p.Affinity)
		var a corev1.Affinity
		_ = json.Unmarshal(b, &a)
		affinity = &a
	}

	// resources
	if p.Resources != nil {
		r := corev1.ResourceList{}
		l := corev1.ResourceList{}
		for k, v := range p.Resources.Limits {
			q, err := resource.ParseQuantity(v)
			if err == nil {
				l[corev1.ResourceName(k)] = q
			}
		}
		for k, v := range p.Resources.Requests {
			q, err := resource.ParseQuantity(v)
			if err == nil {
				r[corev1.ResourceName(k)] = q
			}
		}
		resources = &corev1.ResourceRequirements{Limits: l, Requests: r}
	}

	return
}

func (p PVC) ToK8sPersistentVolumeClaimSpec() corev1.PersistentVolumeClaimSpec {
	accessModes := make([]corev1.PersistentVolumeAccessMode, 0, len(p.AccessModes))
	for _, mode := range p.AccessModes {
		accessModes = append(accessModes, corev1.PersistentVolumeAccessMode(mode))
	}

	var volumeMode *corev1.PersistentVolumeMode
	if p.VolumeMode != nil {
		vm := corev1.PersistentVolumeMode(*p.VolumeMode)
		volumeMode = &vm
	}

	resources := corev1.VolumeResourceRequirements{}
	if p.Resources != nil {
		if len(p.Resources.Limits) > 0 {
			resources.Limits = corev1.ResourceList{}
			for k, v := range p.Resources.Limits {
				q, err := resource.ParseQuantity(v)
				if err == nil {
					resources.Limits[corev1.ResourceName(k)] = q
				}
			}
		}
		if len(p.Resources.Requests) > 0 {
			resources.Requests = corev1.ResourceList{}
			for k, v := range p.Resources.Requests {
				q, err := resource.ParseQuantity(v)
				if err == nil {
					resources.Requests[corev1.ResourceName(k)] = q
				}
			}
		}
	}

	return corev1.PersistentVolumeClaimSpec{
		AccessModes:      accessModes,
		Resources:        resources,
		Selector:         p.Selector,
		StorageClassName: p.StorageClassName,
		VolumeMode:       volumeMode,
	}
}

func PodTemplateSpecFromK8s(podSpec *corev1.PodSpec, container *corev1.Container) *PodTemplateSpec {
	if podSpec == nil || container == nil {
		return &PodTemplateSpec{}
	}

	pt := &PodTemplateSpec{
		NodeSelector:                 podSpec.NodeSelector,
		ServiceAccountName:           podSpec.ServiceAccountName,
		AutomountServiceAccountToken: podSpec.AutomountServiceAccountToken,
		ActiveDeadlineSeconds:        podSpec.ActiveDeadlineSeconds,
	}

	for _, v := range podSpec.Volumes {
		lv := Volume{Name: v.Name}
		if v.EmptyDir != nil {
			lv.EmptyDir = &EmptyDirVolumeSource{}
		}
		if v.PersistentVolumeClaim != nil {
			lv.PersistentVolumeClaim = &PersistentVolumeClaimVolumeSource{ClaimName: v.PersistentVolumeClaim.ClaimName}
		}
		pt.Volumes = append(pt.Volumes, lv)
	}

	for _, vm := range container.VolumeMounts {
		pt.VolumeMounts = append(pt.VolumeMounts, VolumeMount{Name: vm.Name, MountPath: vm.MountPath, ReadOnly: vm.ReadOnly})
	}

	for _, s := range podSpec.ImagePullSecrets {
		pt.ImagePullSecrets = append(pt.ImagePullSecrets, LocalObjectReference{Name: s.Name})
	}

	if podSpec.SecurityContext != nil {
		pt.SecurityContext = &PodSecurityContext{FSGroup: podSpec.SecurityContext.FSGroup}
	}

	for _, t := range podSpec.Tolerations {
		pt.Tolerations = append(pt.Tolerations, Toleration{
			Key:               t.Key,
			Operator:          string(t.Operator),
			Value:             t.Value,
			Effect:            string(t.Effect),
			TolerationSeconds: t.TolerationSeconds,
		})
	}

	if podSpec.Affinity != nil {
		b, _ := json.Marshal(podSpec.Affinity)
		var a Affinity
		_ = json.Unmarshal(b, &a)
		pt.Affinity = &a
	}

	if len(container.Resources.Limits) > 0 || len(container.Resources.Requests) > 0 {
		r := &ResourceRequirements{Limits: map[string]string{}, Requests: map[string]string{}}
		for k, v := range container.Resources.Limits {
			r.Limits[string(k)] = v.String()
		}
		for k, v := range container.Resources.Requests {
			r.Requests[string(k)] = v.String()
		}
		pt.Resources = r
	}

	return pt
}

// TaskSpec defines the structure of a task in the DAG
type TaskSpec struct {
	Name string `json:"name"`
	// +optional
	Command []string `json:"command,omitempty"`
	// +optional
	Args []string `json:"args,omitempty"`
	// +optional
	Image string `json:"image"`
	// +optional
	RunAfter []string `json:"runAfter,omitempty"`
	// +optional
	Backoff Backoff `json:"backoff"`
	// +optional
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
	TaskRef *TaskRef `json:"taskRef,omitempty"`
}

type TaskRef struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
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

type Webhook struct {
	URL       string `json:"url"`
	VerifySSL bool   `json:"verifySSL"`
}

// DAGSpec defines the desired state of DAG
type DAGSpec struct {
	// +optional
	Schedule string `json:"schedule"`
	// +optional
	Task []TaskSpec `json:"task,omitempty"`
	// +optional
	Parameters []DagParameterSpec `json:"parameters,omitempty"`
	// +optional
	Webhook Webhook `json:"webhook,omitempty"`
	// +optional
	Workspace Workspace `json:"workspace,omitempty"`
	// +optional
	Suspended bool `json:"suspended,omitempty"`
	// DSL string to define the DAG using the DSL syntax
	// When provided, this takes precedence over the individual fields above
	// +optional
	DSL string `json:"dsl,omitempty"`
}

// DAGStatus defines the observed state of DAG
type DAGStatus struct {
	// shows the current phase of the DAG
	Phase string `json:"phase,omitempty"`
	// reason for the current phase
	Message string `json:"message,omitempty"`
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
func (dag *DAG) ValidateDAG(refParams map[TaskRef][]string) error {
	if err := dag.checkFieldsFilled(); err != nil {
		return err
	}

	if err := dag.checkParameters(refParams); err != nil {
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

		if task.TaskRef != nil {
			// We ignore any further checks as they will not be used
			// Previous check will have confirmed if this task exists or not
			continue
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
func (dag *DAG) checkParameters(refParams map[TaskRef][]string) error {
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
		if task.TaskRef != nil {
			values, ok := refParams[*task.TaskRef]
			if !ok {
				continue
			}

			for _, value := range values {
				if _, ok := paramsMap[value]; !ok {
					return fmt.Errorf("parameter selected in task does not exist")
				}
			}
		}

		for _, value := range task.Parameters {
			if _, ok := paramsMap[value]; !ok {
				return fmt.Errorf("parameter selected in task does not exist")
			}
		}
	}

	return nil
}
