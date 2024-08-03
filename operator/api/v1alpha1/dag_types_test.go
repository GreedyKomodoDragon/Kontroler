package v1alpha1_test

import (
	"testing"

	"github.com/GreedyKomodoDragon/KubeConductor/operator/api/v1alpha1"
)

func TestValidateDAG(t *testing.T) {
	tests := []struct {
		name    string
		dag     v1alpha1.DAG
		wantErr bool
	}{
		{
			name: "valid DAG",
			dag: v1alpha1.DAG{
				Spec: v1alpha1.DAGSpec{
					Schedule: "*/5 * * * *",
					Task: []v1alpha1.TaskSpec{
						{
							Name:    "task1",
							Command: []string{"sh", "-c"},
							Args:    []string{"echo 'Hello, World!'"},
							Image:   "alpine:latest",
						},
						{
							Name:     "task2",
							Command:  []string{"sh", "-c"},
							Args:     []string{"echo 'Hello again!'"},
							Image:    "alpine:latest",
							RunAfter: []string{"task1"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "single task DAG",
			dag: v1alpha1.DAG{
				Spec: v1alpha1.DAGSpec{
					Schedule: "*/5 * * * *",
					Task: []v1alpha1.TaskSpec{
						{
							Name:    "task1",
							Command: []string{"sh", "-c"},
							Args:    []string{"echo 'Hello, World!'"},
							Image:   "alpine:latest",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing fields",
			dag: v1alpha1.DAG{
				Spec: v1alpha1.DAGSpec{
					Task: []v1alpha1.TaskSpec{
						{
							Name:    "task1",
							Command: []string{},
							Image:   "",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "cyclic dependency",
			dag: v1alpha1.DAG{
				Spec: v1alpha1.DAGSpec{
					Schedule: "*/5 * * * *",
					Task: []v1alpha1.TaskSpec{
						{
							Name:     "task1",
							Command:  []string{"sh", "-c"},
							Args:     []string{"echo 'Hello, World!'"},
							Image:    "alpine:latest",
							RunAfter: []string{"task2"},
						},
						{
							Name:     "task2",
							Command:  []string{"sh", "-c"},
							Args:     []string{"echo 'Hello again!'"},
							Image:    "alpine:latest",
							RunAfter: []string{"task1"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "non-existent runAfter task",
			dag: v1alpha1.DAG{
				Spec: v1alpha1.DAGSpec{
					Schedule: "*/5 * * * *",
					Task: []v1alpha1.TaskSpec{
						{
							Name:     "task1",
							Command:  []string{"sh", "-c"},
							Args:     []string{"echo 'Hello, World!'"},
							Image:    "alpine:latest",
							RunAfter: []string{"task2"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "disconnected tasks",
			dag: v1alpha1.DAG{
				Spec: v1alpha1.DAGSpec{
					Schedule: "*/5 * * * *",
					Task: []v1alpha1.TaskSpec{
						{
							Name:    "task1",
							Command: []string{"sh", "-c"},
							Args:    []string{"echo 'Hello, World!'"},
							Image:   "alpine:latest",
						},
						{
							Name:    "task2",
							Command: []string{"sh", "-c"},
							Args:    []string{"echo 'Hello again!'"},
							Image:   "alpine:latest",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "no starting task",
			dag: v1alpha1.DAG{
				Spec: v1alpha1.DAGSpec{
					Schedule: "*/5 * * * *",
					Task: []v1alpha1.TaskSpec{
						{
							Name:     "task1",
							Command:  []string{"sh", "-c"},
							Args:     []string{"echo 'Hello, World!'"},
							Image:    "alpine:latest",
							RunAfter: []string{"task2"},
						},
						{
							Name:     "task2",
							Command:  []string{"sh", "-c"},
							Args:     []string{"echo 'Hello again!'"},
							Image:    "alpine:latest",
							RunAfter: []string{"task1"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "non-existent runAfter task",
			dag: v1alpha1.DAG{
				Spec: v1alpha1.DAGSpec{
					Schedule: "*/5 * * * *",
					Task: []v1alpha1.TaskSpec{
						{
							Name:     "task1",
							Command:  []string{"sh", "-c"},
							Args:     []string{"echo 'Hello, World!'"},
							Image:    "alpine:latest",
							RunAfter: []string{"task2"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty task list",
			dag: v1alpha1.DAG{
				Spec: v1alpha1.DAGSpec{
					Schedule: "*/5 * * * *",
					Task:     []v1alpha1.TaskSpec{},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate task names",
			dag: v1alpha1.DAG{
				Spec: v1alpha1.DAGSpec{
					Schedule: "*/5 * * * *",
					Task: []v1alpha1.TaskSpec{
						{
							Name:    "task1",
							Command: []string{"sh", "-c"},
							Args:    []string{"echo 'Hello, World!'"},
							Image:   "alpine:latest",
						},
						{
							Name:    "task1", // duplicate name
							Command: []string{"sh", "-c"},
							Args:    []string{"echo 'Hello again!'"},
							Image:   "alpine:latest",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "task self-reference",
			dag: v1alpha1.DAG{
				Spec: v1alpha1.DAGSpec{
					Schedule: "*/5 * * * *",
					Task: []v1alpha1.TaskSpec{
						{
							Name:     "task1",
							Command:  []string{"sh", "-c"},
							Args:     []string{"echo 'Hello, World!'"},
							Image:    "alpine:latest",
							RunAfter: []string{"task1"}, // self-reference
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid DAG with complex dependencies",
			dag: v1alpha1.DAG{
				Spec: v1alpha1.DAGSpec{
					Schedule: "*/5 * * * *",
					Task: []v1alpha1.TaskSpec{
						{
							Name:    "task1",
							Command: []string{"sh", "-c"},
							Args:    []string{"echo 'Task 1'"},
							Image:   "alpine:latest",
						},
						{
							Name:     "task2",
							Command:  []string{"sh", "-c"},
							Args:     []string{"echo 'Task 2'"},
							Image:    "alpine:latest",
							RunAfter: []string{"task1"},
						},
						{
							Name:     "task3",
							Command:  []string{"sh", "-c"},
							Args:     []string{"echo 'Task 3'"},
							Image:    "alpine:latest",
							RunAfter: []string{"task1"},
						},
						{
							Name:     "task4",
							Command:  []string{"sh", "-c"},
							Args:     []string{"echo 'Task 4'"},
							Image:    "alpine:latest",
							RunAfter: []string{"task2", "task3"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "allow multiple start tasks",
			dag: v1alpha1.DAG{
				Spec: v1alpha1.DAGSpec{
					Schedule: "*/5 * * * *",
					Task: []v1alpha1.TaskSpec{
						{
							Name:    "task1",
							Command: []string{"sh", "-c"},
							Args:    []string{"echo 'Task 1'"},
							Image:   "alpine:latest",
						},
						{
							Name:    "task2",
							Command: []string{"sh", "-c"},
							Args:    []string{"echo 'Task 2'"},
							Image:   "alpine:latest",
						},
						{
							Name:     "task3",
							Command:  []string{"sh", "-c"},
							Args:     []string{"echo 'Task 3'"},
							Image:    "alpine:latest",
							RunAfter: []string{"task1", "task2"},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.dag.ValidateDAG(); (err != nil) != tt.wantErr {
				t.Errorf("ValidateDAG() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
