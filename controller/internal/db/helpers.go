package db

import (
	"crypto/sha256"
	"encoding/json"

	"github.com/GreedyKomodoDragon/Kontroler/operator/api/v1alpha1"
)

func getTaskVersion(task *v1alpha1.TaskSpec) int {
	if task.TaskRef != nil {
		return task.TaskRef.Version
	}
	return 1
}

func hashDagSpec(s *v1alpha1.DAGSpec) []byte {
	// Convert the DAGSpec to JSON
	data, err := json.Marshal(s)
	if err != nil {
		// Handle the error appropriately
		return nil
	}

	// Hash the JSON bytes
	hash := sha256.New()
	hash.Write(data)

	return hash.Sum(nil)
}

func hashDagTaskSpec(t *v1alpha1.DagTaskSpec) []byte {
	data, err := json.Marshal(t)
	if err != nil {
		return nil
	}

	hash := sha256.New()
	hash.Write(data)

	return hash.Sum(nil)
}
