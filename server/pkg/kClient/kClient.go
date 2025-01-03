package kclient

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
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

	// Create the DAG object
	dag := map[string]interface{}{
		"apiVersion": "kontroler.greedykomodo/v1alpha1",
		"kind":       "DAG",
		"metadata": map[string]interface{}{
			"labels": labels,
			"name":   dagForm.Name,
		},
		"spec": map[string]interface{}{
			"parameters": parameters,
			"task":       tasks,
		},
	}

	// Define the GVR (Group, Version, Resource) for your custom resource
	gvr := schema.GroupVersionResource{
		Group:    "kontroler.greedykomodo",
		Version:  "v1alpha1",
		Resource: "dags",
	}

	// Define the custom resource object using an unstructured object
	customResource := &unstructured.Unstructured{
		Object: dag,
	}

	_, err := client.Resource(gvr).Namespace(dagForm.Namespace).Create(ctx, customResource, metav1.CreateOptions{})
	return err
}

func CreateDagRun(ctx context.Context, drForm DagRunForm, isSecretMap map[string]bool, namespace string, client dynamic.Interface) error {
	if drForm.Name == "" {
		return fmt.Errorf("cannot have an empty dagrun name")
	}

	labels := map[string]string{
		"app.kubernetes.io/name":       "dag",
		"app.kubernetes.io/instance":   drForm.Name,
		"app.kubernetes.io/part-of":    "kontroler",
		"app.kubernetes.io/created-by": "server",
	}

	dagRunSpec := map[string]interface{}{
		"dagName": drForm.Name,
	}

	parameters := []interface{}{}

	for k, v := range drForm.Parameters {
		isSecret, ok := isSecretMap[k]
		if !ok {
			return fmt.Errorf("missing parameter: %s", k)
		}

		if isSecret {
			parameters = append(parameters, SecretParameter{
				Name:       k,
				FromSecret: v,
			})
		} else {
			parameters = append(parameters, ValParameter{
				Name:  k,
				Value: v,
			})
		}
	}

	dagRunSpec["parameters"] = parameters

	dagRun := map[string]interface{}{
		"apiVersion": "kontroler.greedykomodo/v1alpha1",
		"kind":       "DagRun",
		"metadata": map[string]interface{}{
			"labels": labels,
			"name":   drForm.RunName,
		},
		"spec": dagRunSpec,
	}

	// Define the GVR (Group, Version, Resource) for your custom resource
	gvr := schema.GroupVersionResource{
		Group:    "kontroler.greedykomodo",
		Version:  "v1alpha1",
		Resource: "dagruns",
	}

	// Define the custom resource object using an unstructured object
	customResource := &unstructured.Unstructured{
		Object: dagRun,
	}

	_, err := client.Resource(gvr).Namespace(namespace).Create(ctx, customResource, metav1.CreateOptions{})
	return err
}

func NewClient() (dynamic.Interface, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return dynClient, nil
}
