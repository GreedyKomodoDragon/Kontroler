package kclient

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// convertFormParameters converts form parameters to the API parameter representation used in the CRD.
func convertFormParameters(params []FormDagParameterSpec) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(params))
	for _, p := range params {
		m := map[string]interface{}{"name": p.Name}
		if p.IsSecret {
			m["defaultFromSecret"] = p.Value
		} else {
			m["defaultValue"] = p.Value
		}
		out = append(out, m)
	}
	return out
}

// convertFormTasks converts form tasks to the CRD task representation.
// paramNameByID maps form parameter IDs to parameter names.
func convertFormTasks(tasks []FormTaskSpec, paramNameByID map[string]string) ([]map[string]interface{}, error) {
	out := make([]map[string]interface{}, 0, len(tasks))
	for _, t := range tasks {
		paramNames := []string{}
		for _, pid := range t.Parameters {
			if n, ok := paramNameByID[pid]; ok {
				paramNames = append(paramNames, n)
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

			if t.Script == "" {
				task["command"] = t.Command
				task["args"] = t.Args
			} else {
				task["script"] = t.Script
			}

			if t.PodTemplate != "" {
				var result map[string]interface{}
				if err := json.Unmarshal([]byte(t.PodTemplate), &result); err != nil {
					return nil, fmt.Errorf("invalid podTemplate JSON for task %s: %w", t.Name, err)
				}
				task["podTemplate"] = result
			}
		}

		if len(t.RunAfter) > 0 {
			task["runAfter"] = t.RunAfter
		}

		out = append(out, task)
	}
	return out, nil
}

// BuildDAGUnstructured constructs an unstructured.Unstructured object for the given DagFormObj.
func BuildDAGUnstructured(dagForm DagFormObj) (*unstructured.Unstructured, error) {
	paramNameByID := make(map[string]string, len(dagForm.Parameters))
	for _, p := range dagForm.Parameters {
		paramNameByID[p.ID] = p.Name
	}

	parameters := convertFormParameters(dagForm.Parameters)
	tasks, err := convertFormTasks(dagForm.Tasks, paramNameByID)
	if err != nil {
		return nil, err
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

	dag := map[string]interface{}{
		"apiVersion": "kontroler.greedykomodo/v1alpha1",
		"kind":       "DAG",
		"metadata": map[string]interface{}{
			"labels": map[string]string{
				"app.kubernetes.io/name":       "dag",
				"app.kubernetes.io/instance":   dagForm.Name,
				"app.kubernetes.io/part-of":    "kontroler",
				"app.kubernetes.io/created-by": "server",
			},
			"name": dagForm.Name,
		},
		"spec": spec,
	}

	return &unstructured.Unstructured{Object: dag}, nil
}
