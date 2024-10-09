package dag_test

import (
	"testing"

	"github.com/GreedyKomodoDragon/Kontroler/operator/internal/dag"
	"github.com/GreedyKomodoDragon/Kontroler/operator/internal/db"
)

func FuzzCreateEnvs(f *testing.F) {
	// Seed corpus with simple valid data
	f.Add("param1", "value1", false)
	f.Add("param2", "value2", true)

	// Define the fuzzing function
	f.Fuzz(func(t *testing.T, name string, value string, isSecret bool) {
		// Create a task with one random parameter
		task := db.Task{
			Parameters: []db.Parameter{
				{
					Name:     name,
					Value:    value,
					IsSecret: isSecret,
				},
			},
		}

		// Create a taskAllocator instance
		ta := dag.NewTaskAllocator(nil, "random")

		// Run CreateEnvs function
		envs := ta.CreateEnvs(task)

		// Check if the function returns valid results
		if envs == nil || len(*envs) == 0 {
			t.Errorf("Expected non-empty env vars, got: %v", envs)
		}

		// Additional checks can be added here, e.g., ensuring specific behavior
		for _, env := range *envs {
			if isSecret && env.ValueFrom == nil {
				t.Errorf("Expected ValueFrom for secret but got: %v", env)
			}
			if !isSecret && env.Value != value {
				t.Errorf("Expected Value: %v, got: %v", value, env.Value)
			}
		}
	})
}
