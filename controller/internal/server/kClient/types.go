package kclient

import (
	corev1 "k8s.io/api/core/v1"
	v1 "kontroler-controller/api/v1alpha1"
)

// Keep a small set of client-only helper types here and alias shared API types

type Metadata struct {
	Labels map[string]string `json:"labels"`
	Name   string            `json:"name"`
}

// The UI/form representation of a DAG parameter (contains ID/Value/IsSecret used by the form)
// Renamed to FormDagParameterSpec to avoid collision with the API DagParameterSpec type.
type FormDagParameterSpec struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IsSecret bool   `json:"isSecret"`
	Value    string `json:"value"`
}

// The UI/form representation of a task used by the DAG form.
// Note: this intentionally differs from the CRD TaskSpec — keep it local.
// Renamed to FormTaskSpec to avoid collision with the API TaskSpec type.
type FormTaskSpec struct {
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

// Reuse API types from controller/api/v1alpha1 where they are semantically identical.
type Parameter = v1.DagParameterSpec
type Backoff = v1.Backoff
type Conditional = v1.Conditional
type PodTemplate = v1.PodTemplateSpec
type TaskRef = v1.TaskRef
type PVC = v1.PVC
type Workspace = v1.Workspace
type Webhook = v1.Webhook
type DAGSpec = v1.DAGSpec
type DAG = v1.DAG

// Use core Kubernetes types for volume structures
type Volume = corev1.Volume
type VolumeMount = corev1.VolumeMount
type PersistentVolumeClaim = corev1.PersistentVolumeClaim

// DagFormObj represents the overall DAG form object used by the UI/client layer.
// Uses the form-specific types (FormTaskSpec/FormDagParameterSpec) which differ from the CRD types.
type DagFormObj struct {
	Name       string                 `json:"name"`
	Schedule   string                 `json:"schedule,omitempty"`
	Tasks      []FormTaskSpec         `json:"tasks"`
	Parameters []FormDagParameterSpec `json:"parameters,omitempty"`
	Namespace  string                 `json:"namespace"`
	Webhook    Webhook                `json:"webhook"`
	Workspace  *Workspace             `json:"workspace,omitempty"`
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

type DagSuspendForm struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Suspend   bool   `json:"suspend"`
}
