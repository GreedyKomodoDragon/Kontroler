package db

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"kontroler-controller/api/v1alpha1"
)

func getTaskVersion(task *v1alpha1.TaskSpec) int {
	if task.TaskRef != nil {
		return task.TaskRef.Version
	}
	return 1
}

func hashDagSpec(s *v1alpha1.DAGSpec) ([]byte, error) {
	// Convert the DAGSpec to JSON
	data, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal DAGSpec: %w", err)
	}
	// Hash the JSON bytes
	hash := sha256.New()
	hash.Write(data)
	return hash.Sum(nil), nil
}

func hashDagTaskSpec(t *v1alpha1.DagTaskSpec) ([]byte, error) {
	// Use canonical JSON encoding to ensure consistent hashing
	encoder := json.NewEncoder(bytes.NewBuffer(nil))
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "")

	data, err := json.Marshal(t)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal DagTaskSpec: %w", err)
	}

	hash := sha256.New()
	hash.Write(data)

	return hash.Sum(nil), nil
}
