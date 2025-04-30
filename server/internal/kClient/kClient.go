package kclient

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	dagRunsGVR = schema.GroupVersionResource{
		Group:    "kontroler.greedykomodo",
		Version:  "v1alpha1",
		Resource: "dagruns",
	}

	dagsGVR = schema.GroupVersionResource{
		Group:    "kontroler.greedykomodo",
		Version:  "v1alpha1",
		Resource: "dags",
	}
)

func CreateDAG(ctx context.Context, dagForm DagFormObj, client dynamic.Interface) error {

	if dagForm.Namespace == "" {
		return fmt.Errorf("request contains empty namespace")
	}

	// Example metadata generation
	labels := map[string]string{
		"app.kubernetes.io/name":       "dag",
		"app.kubernetes.io/instance":   dagForm.Name,
		"app.kubernetes.io/part-of":    "kontroler",
		"app.kubernetes.io/created-by": "server",
	}

	// Create a map to quickly lookup parameter names by ID
	paramNameByID := make(map[string]string)
	for _, p := range dagForm.Parameters {
		paramNameByID[p.ID] = p.Name
	}

	// Convert DagParameterSpec to Parameters
	var parameters []map[string]interface{}
	for _, p := range dagForm.Parameters {
		param := map[string]interface{}{
			"name": p.Name,
		}
		if p.IsSecret {
			param["defaultFromSecret"] = p.Value
		} else {
			param["defaultValue"] = p.Value
		}
		parameters = append(parameters, param)
	}

	// Convert TaskSpec to Tasks
	var tasks []map[string]interface{}
	for _, t := range dagForm.Tasks {
		paramNames := []string{}

		for _, paramID := range t.Parameters {
			if paramName, exists := paramNameByID[paramID]; exists {
				paramNames = append(paramNames, paramName)
			}
		}

		var task map[string]interface{}
		if t.TaskRef != nil {
			task = map[string]interface{}{
				"name": t.Name,
				"taskRef": map[string]interface{}{
					"name":    t.TaskRef.Name,
					"version": t.TaskRef.Version,
				},
			}
		} else {
			task = map[string]interface{}{
				"name":  t.Name,
				"image": t.Image,
				"backoff": map[string]interface{}{
					"limit": t.BackoffLimit,
				},
				"parameters": paramNames,
				"conditional": map[string]interface{}{
					"enabled":    len(t.RetryCodes) != 0,
					"retryCodes": t.RetryCodes,
				},
			}

			// Only send over the command and args if no script has been provided
			if t.Script == "" {
				task["command"] = t.Command
				task["args"] = t.Args
			} else {
				task["script"] = t.Script
			}

			if t.PodTemplate != "" {
				var result map[string]interface{}
				if err := json.Unmarshal([]byte(t.PodTemplate), &result); err != nil {
					return err
				}

				task["podTemplate"] = result
			}
		}

		if len(t.RunAfter) > 0 {
			task["runAfter"] = t.RunAfter
		}

		tasks = append(tasks, task)
	}

	spec := map[string]interface{}{
		"parameters": parameters,
		"task":       tasks,
	}

	if dagForm.Webhook.URL != "" {
		spec["webhook"] = map[string]interface{}{
			"url":       dagForm.Webhook.URL,
			"verifySSL": dagForm.Webhook.VerifySSL,
		}
	}

	if dagForm.Workspace != nil {
		spec["workspace"] = map[string]interface{}{
			"enable": dagForm.Workspace.Enabled,
			"pvc": map[string]interface{}{
				"accessModes":      dagForm.Workspace.PvcSpec.AccessModes,
				"selector":         dagForm.Workspace.PvcSpec.Selector,
				"resources":        dagForm.Workspace.PvcSpec.Resources,
				"storageClassName": dagForm.Workspace.PvcSpec.StorageClassName,
				"volumeMode":       dagForm.Workspace.PvcSpec.VolumeMode,
			},
		}
	}

	// Create the DAG object
	dag := map[string]interface{}{
		"apiVersion": "kontroler.greedykomodo/v1alpha1",
		"kind":       "DAG",
		"metadata": map[string]interface{}{
			"labels": labels,
			"name":   dagForm.Name,
		},
		"spec": spec,
	}

	// Define the custom resource object using an unstructured object
	customResource := &unstructured.Unstructured{
		Object: dag,
	}

	dagResource, err := client.Resource(dagsGVR).Namespace(dagForm.Namespace).Create(ctx, customResource, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create DAG: %w", err)
	}

	// Wait for reconciliation with timeout
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for DAG reconciliation")
		case <-timeout:
			return fmt.Errorf("timeout waiting for DAG reconciliation")
		case <-ticker.C:
			// Get latest DAG status
			current, err := client.Resource(dagsGVR).Namespace(dagForm.Namespace).Get(ctx, dagResource.GetName(), metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get DAG status: %w", err)
			}

			// Check status conditions
			status, found, err := unstructured.NestedString(current.Object, "status", "phase")
			if err != nil {
				return fmt.Errorf("failed to get status conditions: %w", err)
			}

			if !found {
				continue // Status not yet set
			}

			if status == "Failed" {
				message, found, err := unstructured.NestedString(current.Object, "status", "phase")
				if err != nil {
					return fmt.Errorf("failed to get message/reason for failed: %w", err)
				}

				if !found {
					return fmt.Errorf("DAG failed to reconcile, but no reason given")
				}

				return fmt.Errorf("DAG failed to reconcile, reason: %s", message)
			}

			return nil
		}
	}
}

// CreateDagRunOpts allows for optional configuration of CreateDagRun
type CreateDagRunOpts struct {
	RunIDTimeout time.Duration
	Cleanup      bool
}

// DefaultCreateDagRunOpts provides default options for CreateDagRun
var DefaultCreateDagRunOpts = CreateDagRunOpts{
	RunIDTimeout: 10 * time.Second,
	Cleanup:      true,
}

func CreateDagRun(ctx context.Context, drForm DagRunForm, isSecretMap map[string]bool, namespace string, client dynamic.Interface, opts *CreateDagRunOpts) (int64, error) {
	if opts == nil {
		opts = &DefaultCreateDagRunOpts
	}

	// Validate inputs
	if err := validateDagRunInputs(drForm, namespace); err != nil {
		return 0, fmt.Errorf("validation failed: %w", err)
	}

	// Create DAG run resource
	dagRun, err := createDagRunResource(drForm, isSecretMap, namespace)
	if err != nil {
		return 0, fmt.Errorf("failed to create dag run resource: %w", err)
	}

	// Create the resource in Kubernetes
	created, err := client.Resource(dagRunsGVR).Namespace(namespace).Create(ctx, dagRun, metav1.CreateOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to create dagrun in kubernetes: %w", err)
	}

	// Wait for RunID with cleanup on failure
	runID, err := waitForRunID(ctx, client, namespace, drForm.RunName, opts.RunIDTimeout)
	if err != nil && opts.Cleanup {
		// Attempt cleanup on failure
		deleteCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if cleanupErr := client.Resource(dagRunsGVR).Namespace(namespace).Delete(deleteCtx, created.GetName(), metav1.DeleteOptions{}); cleanupErr != nil {
			return 0, fmt.Errorf("run ID error: %v, cleanup error: %v", err, cleanupErr)
		}
		return 0, fmt.Errorf("failed to get run ID: %w", err)
	}

	return runID, err
}

func validateDagRunInputs(form DagRunForm, namespace string) error {
	if form.Name == "" {
		return fmt.Errorf("dagrun name cannot be empty")
	}
	if form.RunName == "" {
		return fmt.Errorf("run name cannot be empty")
	}
	if namespace == "" {
		return fmt.Errorf("namespace cannot be empty")
	}
	return nil
}

func createDagRunResource(form DagRunForm, isSecretMap map[string]bool, namespace string) (*unstructured.Unstructured, error) {
	parameters := make([]interface{}, 0, len(form.Parameters))

	for paramName, paramValue := range form.Parameters {
		isSecret, exists := isSecretMap[paramName]
		if !exists {
			return nil, fmt.Errorf("parameter %s not found in isSecretMap", paramName)
		}

		param := map[string]interface{}{
			"name": paramName,
		}

		if isSecret {
			param["fromSecret"] = paramValue
		} else {
			param["value"] = paramValue
		}

		parameters = append(parameters, param)
	}

	dagRun := map[string]interface{}{
		"apiVersion": "kontroler.greedykomodo/v1alpha1",
		"kind":       "DagRun",
		"metadata": map[string]interface{}{
			"name": form.RunName,
			"labels": map[string]string{
				"app.kubernetes.io/name":       "dag",
				"app.kubernetes.io/instance":   form.Name,
				"app.kubernetes.io/part-of":    "kontroler",
				"app.kubernetes.io/created-by": "server",
			},
		},
		"spec": map[string]interface{}{
			"dagName":    form.Name,
			"parameters": parameters,
		},
	}

	return &unstructured.Unstructured{Object: dagRun}, nil
}

func NewClients(config *rest.Config) (dynamic.Interface, *kubernetes.Clientset, error) {
	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	// Create a Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	return dynClient, clientset, nil
}

func DeleteDAG(ctx context.Context, namespace string, name string, client dynamic.Interface) error {
	return client.Resource(dagsGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

func DeleteDagRun(ctx context.Context, namespace string, name string, client dynamic.Interface) error {
	return client.Resource(dagRunsGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

func waitForRunID(ctx context.Context, client dynamic.Interface, namespace, runName string, timeout time.Duration) (int64, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		dagRun, err := client.Resource(dagRunsGVR).Namespace(namespace).Get(ctx, runName, metav1.GetOptions{})
		if err != nil {
			return 0, err
		}

		status, found, err := unstructured.NestedInt64(dagRun.Object, "status", "dagRunId")
		if err != nil {
			return 0, err
		}

		if found && status != 0 {
			return status, nil
		}

		time.Sleep(1 * time.Second) // Polling interval
	}

	return 0, fmt.Errorf("timed out waiting for DagRun %s to be reconciled", runName)
}
