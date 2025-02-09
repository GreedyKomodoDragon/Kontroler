package kclient

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Metadata struct {
	Labels map[string]string `json:"labels"`
	Name   string            `json:"name"`
}

type Parameter struct {
	Name              string `json:"name"`
	DefaultFromSecret string `json:"defaultFromSecret,omitempty"`
	DefaultValue      string `json:"defaultValue,omitempty"`
}

type Backoff struct {
	Limit int `json:"limit"`
}

type Conditional struct {
	Enabled    bool  `json:"enabled"`
	RetryCodes []int `json:"retryCodes"`
}

type VolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
}

type PersistentVolumeClaim struct {
	ClaimName string `json:"claimName"`
}

type Volume struct {
	Name                  string                `json:"name"`
	PersistentVolumeClaim PersistentVolumeClaim `json:"persistentVolumeClaim"`
}

type PodTemplate struct {
	Volumes      []Volume      `json:"volumes"`
	VolumeMounts []VolumeMount `json:"volumeMounts"`
}

type Task struct {
	Name        string      `json:"name"`
	Command     []string    `json:"command"`
	Args        []string    `json:"args"`
	Image       string      `json:"image"`
	RunAfter    []string    `json:"runAfter,omitempty"`
	Backoff     Backoff     `json:"backoff"`
	Parameters  []string    `json:"parameters"`
	Conditional Conditional `json:"conditional"`
	PodTemplate PodTemplate `json:"podTemplate,omitempty"`
}

type DAGSpec struct {
	Parameters []Parameter `json:"parameters"`
	Task       []Task      `json:"task"`
}

type DAG struct {
	ApiVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Metadata   Metadata `json:"metadata"`
	Spec       DAGSpec  `json:"spec"`
}

// DagParameterSpec represents a parameter for the DAG.
type DagParameterSpec struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IsSecret bool   `json:"isSecret"`
	Value    string `json:"value"`
}

// TaskSpec represents a task within the DAG.
type TaskSpec struct {
	Name         string   `json:"name"`
	Command      []string `json:"command,omitempty"`
	Args         []string `json:"args,omitempty"`
	Script       string   `json:"script,omitempty"`
	Image        string   `json:"image"`
	RunAfter     []string `json:"runAfter,omitempty"`
	BackoffLimit int      `json:"backoffLimit"`
	RetryCodes   []int    `json:"retryCodes,omitempty"`
	Parameters   []string `json:"parameters,omitempty"`
	PodTemplate  string   `json:"podTemplate,omitempty"`
	TaskRef      *TaskRef `json:"taskRef,omitempty"`
}

type TaskRef struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
}

type PVC struct {
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes"`
	// +optional
	Selector         *metav1.LabelSelector        `json:"selector,omitempty"`
	Resources        corev1.ResourceRequirements  `json:"resources,omitempty"`
	StorageClassName *string                      `json:"storageClassName,omitempty"`
	VolumeMode       *corev1.PersistentVolumeMode `json:"volumeMode,omitempty"`
}

type Workspace struct {
	Enabled bool `json:"enable"`
	PvcSpec PVC  `json:"pvc"`
}

// DagFormObj represents the overall DAG form object.
type DagFormObj struct {
	Name       string             `json:"name"`
	Schedule   string             `json:"schedule,omitempty"`
	Tasks      []TaskSpec         `json:"tasks"`
	Parameters []DagParameterSpec `json:"parameters,omitempty"`
	Namespace  string             `json:"namespace"`
	Webhook    Webhook            `json:"webhook"`
	Workspace  *Workspace         `json:"workspace,omitempty"`
}

type Webhook struct {
	URL       string `json:"url"`
	VerifySSL bool   `json:"verifySSL"`
}

type DagRunForm struct {
	Name       string            `json:"name"`
	RunName    string            `json:"runName"`
	Parameters map[string]string `json:"parameters"`
	Namespace  string            `json:"namespace"`
}

type ValParameter struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type SecretParameter struct {
	Name       string `json:"name"`
	FromSecret string `json:"fromSecret"`
}
