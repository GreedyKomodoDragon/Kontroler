package db_test

import (
	"context"
	"testing"

	"github.com/GreedyKomodoDragon/Kontroler/operator/api/v1alpha1"
	"github.com/GreedyKomodoDragon/Kontroler/operator/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test case: Retrieve existing unique_id if present in IdTable
func testDAGManagerGetID_ReturnsExistingID(t *testing.T, dm db.DBDAGManager) {
	t.Run("ReturnsExistingID", func(t *testing.T) {
		uniqueID1, err := dm.GetID(context.Background())
		require.NoError(t, err)
		require.NotEmpty(t, uniqueID1)

		uniqueID2, err := dm.GetID(context.Background())
		require.NoError(t, err)
		assert.Equal(t, uniqueID1, uniqueID2)
	})
}

// Test case: Insert and retrieve a new unique_id if none exists
func testDAGManagerGetID_InsertsAndReturnsNewID(t *testing.T, dm db.DBDAGManager) {
	t.Run("InsertsAndReturnsNewID", func(t *testing.T) {
		uniqueID, err := dm.GetID(context.Background())
		assert.NoError(t, err)
		assert.NotEmpty(t, uniqueID)

		idAfterInsert, err := dm.GetID(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, uniqueID, idAfterInsert)
	})
}

// Test case: IncrementAttempts increases the attempts count by 1
func testDAGManagerIncrementAttempts_IncrementAttempts(t *testing.T, dm db.DBDAGManager) {
	t.Run("IncrementAttempts", func(t *testing.T) {
		dag := &v1alpha1.DAG{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test_dag",
			},
			Spec: v1alpha1.DAGSpec{
				Schedule: "*/5 * * * *",
				Task: []v1alpha1.TaskSpec{
					{
						Name:    "task1",
						Command: []string{"echo", "Hello"},
						Args:    []string{"arg1", "arg2"},
						Image:   "busybox",
					},
				},
			},
		}

		err := dm.InsertDAG(context.Background(), dag, "default")
		require.NoError(t, err)

		dagRun := &v1alpha1.DagRunSpec{
			DagName: "test_dag",
		}

		runID, err := dm.CreateDAGRun(context.Background(), "name", dagRun, map[string]v1alpha1.ParameterSpec{})
		require.NoError(t, err)

		taskRunID, err := dm.MarkTaskAsStarted(context.Background(), runID, 1)
		assert.NoError(t, err)
		assert.NotEqual(t, 0, taskRunID)

		err = dm.IncrementAttempts(context.Background(), taskRunID)
		require.NoError(t, err)
	})

}

// Test case: Multiple calls to IncrementAttempts increase attempts correctly
func testDAGManagerIncrementAttempts_MultipleIncrements(t *testing.T, dm db.DBDAGManager) {
	t.Run("MultipleIncrements", func(t *testing.T) {
		dag := &v1alpha1.DAG{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test_dag_2",
			},
			Spec: v1alpha1.DAGSpec{
				Schedule: "*/5 * * * *",
				Task: []v1alpha1.TaskSpec{
					{
						Name:    "task1",
						Command: []string{"echo", "Hello"},
						Args:    []string{"arg1", "arg2"},
						Image:   "busybox",
					},
				},
			},
		}

		err := dm.InsertDAG(context.Background(), dag, "default")
		require.NoError(t, err)

		dagRun := &v1alpha1.DagRunSpec{
			DagName: "test_dag",
		}

		runID, err := dm.CreateDAGRun(context.Background(), "name_2", dagRun, map[string]v1alpha1.ParameterSpec{})
		require.NoError(t, err)

		taskRunID, err := dm.MarkTaskAsStarted(context.Background(), runID, 2)
		assert.NoError(t, err)
		assert.NotEqual(t, 0, taskRunID)

		err = dm.IncrementAttempts(context.Background(), taskRunID)
		require.NoError(t, err)

		err = dm.IncrementAttempts(context.Background(), taskRunID)
		require.NoError(t, err)
	})
}

// Test case: Shows Parameters are empty
func testDAGManagerGetParameters_Empty(t *testing.T, dm db.DBDAGManager) {
	t.Run("Empty Parameters", func(t *testing.T) {
		dag := &v1alpha1.DAG{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test_dag",
			},
			Spec: v1alpha1.DAGSpec{
				Schedule:   "*/5 * * * *",
				Parameters: []v1alpha1.DagParameterSpec{},
				Task: []v1alpha1.TaskSpec{
					{
						Name:    "task1",
						Command: []string{"echo", "Hello"},
						Args:    []string{"arg1", "arg2"},
						Image:   "busybox",
					},
					{
						Name:    "task2",
						Command: []string{"echo", "Hello"},
						Args:    []string{"arg1", "arg2"},
						Image:   "busybox",
					},
					{
						Name:     "task3",
						Command:  []string{"echo"},
						Args:     []string{"Goodbye, World!"},
						Image:    "alpine:latest",
						RunAfter: []string{"task1", "task2"},
					},
				},
			},
		}

		err := dm.InsertDAG(context.Background(), dag, "default")
		require.NoError(t, err)

		paramsMap, err := dm.GetDagParameters(context.Background(), "test_dag")
		require.NoError(t, err)
		require.Empty(t, paramsMap)
	})
}

// Test case: Shows Parameters has values in it
func testDAGManagerGetParameters_HasValues(t *testing.T, dm db.DBDAGManager) {
	t.Run("Has Parameters", func(t *testing.T) {
		dag := &v1alpha1.DAG{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test_dag_2",
			},
			Spec: v1alpha1.DAGSpec{
				Schedule: "*/5 * * * *",
				Parameters: []v1alpha1.DagParameterSpec{
					{
						Name:         "one",
						DefaultValue: "random",
					},
					{
						Name:              "two",
						DefaultFromSecret: "randomSecret",
					},
				},
				Task: []v1alpha1.TaskSpec{
					{
						Name:    "task1",
						Command: []string{"echo", "Hello"},
						Args:    []string{"arg1", "arg2"},
						Image:   "busybox",
					},
					{
						Name:    "task2",
						Command: []string{"echo", "Hello"},
						Args:    []string{"arg1", "arg2"},
						Image:   "busybox",
					},
					{
						Name:     "task3",
						Command:  []string{"echo"},
						Args:     []string{"Goodbye, World!"},
						Image:    "alpine:latest",
						RunAfter: []string{"task1", "task2"},
					},
				},
			},
		}

		err := dm.InsertDAG(context.Background(), dag, "default")
		require.NoError(t, err)

		paramsMap, err := dm.GetDagParameters(context.Background(), "test_dag_2")
		require.NoError(t, err)
		require.NotEmpty(t, paramsMap)
		require.Equal(t, 2, len(paramsMap))

		val, ok := paramsMap["one"]
		require.True(t, ok)
		require.Equal(t, "random", val.Value)
		require.False(t, val.IsSecret)

		val, ok = paramsMap["two"]
		require.True(t, ok)
		require.Equal(t, "randomSecret", val.Value)
		require.True(t, val.IsSecret)

		val, ok = paramsMap["three"]
		require.False(t, ok)
		require.Nil(t, val)
	})
}

func testDAGManagerDagExists(t *testing.T, dm db.DBDAGManager) {
	t.Run("Has Parameters", func(t *testing.T) {
		dag := &v1alpha1.DAG{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test_dag",
			},
			Spec: v1alpha1.DAGSpec{
				Schedule: "*/5 * * * *",
				Parameters: []v1alpha1.DagParameterSpec{
					{
						Name:         "one",
						DefaultValue: "random",
					},
					{
						Name:              "two",
						DefaultFromSecret: "randomSecret",
					},
				},
				Task: []v1alpha1.TaskSpec{
					{
						Name:    "task1",
						Command: []string{"echo", "Hello"},
						Args:    []string{"arg1", "arg2"},
						Image:   "busybox",
					},
					{
						Name:    "task2",
						Command: []string{"echo", "Hello"},
						Args:    []string{"arg1", "arg2"},
						Image:   "busybox",
					},
					{
						Name:     "task3",
						Command:  []string{"echo"},
						Args:     []string{"Goodbye, World!"},
						Image:    "alpine:latest",
						RunAfter: []string{"task1", "task2"},
					},
				},
			},
		}

		ok, err := dm.DagExists(context.Background(), "test_dag")
		require.False(t, ok)
		require.Nil(t, err)

		err = dm.InsertDAG(context.Background(), dag, "default")
		require.NoError(t, err)

		ok, err = dm.DagExists(context.Background(), "test_dag")
		require.True(t, ok)
		require.Nil(t, err)

	})
}

func testDAGManagerShouldRerun_MatchingExitCode(t *testing.T, dm db.DBDAGManager) {
	t.Run("Matching Retry Code", func(t *testing.T) {
		dag := &v1alpha1.DAG{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test_dag",
			},
			Spec: v1alpha1.DAGSpec{
				Schedule: "*/5 * * * *",
				Task: []v1alpha1.TaskSpec{
					{
						Name:    "task1",
						Command: []string{"echo", "Hello"},
						Args:    []string{"arg1", "arg2"},
						Image:   "busybox",
						Conditional: v1alpha1.Conditional{
							Enabled:    true,
							RetryCodes: []int{3},
						},
						Backoff: v1alpha1.Backoff{
							Limit: 3,
						},
					},
				},
			},
		}

		err := dm.InsertDAG(context.Background(), dag, "default")
		require.NoError(t, err)

		dagRun := &v1alpha1.DagRunSpec{
			DagName: "test_dag",
		}

		runID, err := dm.CreateDAGRun(context.Background(), "name", dagRun, map[string]v1alpha1.ParameterSpec{})
		require.NoError(t, err)

		taskRunID, err := dm.MarkTaskAsStarted(context.Background(), runID, 1)
		assert.NoError(t, err)
		assert.NotEqual(t, 0, taskRunID)

		ok, err := dm.ShouldRerun(context.Background(), taskRunID, 3)
		require.NoError(t, err)
		require.True(t, ok)
	})

}

func testDAGManagerShouldRerun_MisMatchCode(t *testing.T, dm db.DBDAGManager) {
	t.Run("Mismatch Retry Code", func(t *testing.T) {
		dag := &v1alpha1.DAG{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test_dag__MisMatchCode",
			},
			Spec: v1alpha1.DAGSpec{
				Schedule: "*/5 * * * *",
				Task: []v1alpha1.TaskSpec{
					{
						Name:    "task1",
						Command: []string{"echo", "Hello"},
						Args:    []string{"arg1", "arg2"},
						Image:   "busybox",
						Conditional: v1alpha1.Conditional{
							Enabled:    true,
							RetryCodes: []int{3},
						},
						Backoff: v1alpha1.Backoff{
							Limit: 3,
						},
					},
				},
			},
		}

		err := dm.InsertDAG(context.Background(), dag, "default")
		require.NoError(t, err)

		dagRun := &v1alpha1.DagRunSpec{
			DagName: "test_dag__MisMatchCode",
		}

		runID, err := dm.CreateDAGRun(context.Background(), "name_MisMatchCode", dagRun, map[string]v1alpha1.ParameterSpec{})
		require.NoError(t, err)

		taskRunID, err := dm.MarkTaskAsStarted(context.Background(), runID, 1)
		assert.NoError(t, err)
		assert.NotEqual(t, 0, taskRunID)

		ok, err := dm.ShouldRerun(context.Background(), taskRunID, 127)
		require.NoError(t, err)
		require.False(t, ok)
	})

}

func testDAGManagerShouldRerun_ValidCodeButNoAttemptsLeft(t *testing.T, dm db.DBDAGManager) {
	t.Run("Valid Code But No Attempts Left", func(t *testing.T) {
		dag := &v1alpha1.DAG{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test_dag_ValidCodeButNoAttemptsLeft",
			},
			Spec: v1alpha1.DAGSpec{
				Schedule: "*/5 * * * *",
				Task: []v1alpha1.TaskSpec{
					{
						Name:    "task1",
						Command: []string{"echo", "Hello"},
						Args:    []string{"arg1", "arg2"},
						Image:   "busybox",
						Conditional: v1alpha1.Conditional{
							Enabled:    true,
							RetryCodes: []int{3},
						},
						Backoff: v1alpha1.Backoff{
							Limit: 1,
						},
					},
				},
			},
		}

		err := dm.InsertDAG(context.Background(), dag, "default")
		require.NoError(t, err)

		dagRun := &v1alpha1.DagRunSpec{
			DagName: "test_dag_ValidCodeButNoAttemptsLeft",
		}

		runID, err := dm.CreateDAGRun(context.Background(), "name_ValidCodeButNoAttemptsLeft", dagRun, map[string]v1alpha1.ParameterSpec{})
		require.NoError(t, err)

		taskRunID, err := dm.MarkTaskAsStarted(context.Background(), runID, 1)
		assert.NoError(t, err)
		assert.NotEqual(t, 0, taskRunID)

		ok, err := dm.ShouldRerun(context.Background(), taskRunID, 127)
		require.NoError(t, err)
		require.False(t, ok)
	})

}
