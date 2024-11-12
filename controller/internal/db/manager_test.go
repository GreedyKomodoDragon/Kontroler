package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/GreedyKomodoDragon/Kontroler/operator/api/v1alpha1"
	"github.com/GreedyKomodoDragon/Kontroler/operator/internal/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

func testDAGManagerMarkTaskAsFailed_Normal(t *testing.T, dm db.DBDAGManager) {
	t.Run("Successful Path", func(t *testing.T) {
		dag := &v1alpha1.DAG{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test_dag_Normal",
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
			DagName: "test_dag_Normal",
		}

		runID, err := dm.CreateDAGRun(context.Background(), "name_Normal", dagRun, map[string]v1alpha1.ParameterSpec{})
		require.NoError(t, err)

		taskRunID, err := dm.MarkTaskAsStarted(context.Background(), runID, 1)
		assert.NoError(t, err)
		assert.NotEqual(t, 0, taskRunID)

		err = dm.MarkTaskAsFailed(context.Background(), taskRunID)
		require.NoError(t, err)
	})

}

func testDAGManagerMarkPodStatus_Insert(t *testing.T, dm db.DBDAGManager) {
	t.Run("Successful Path", func(t *testing.T) {
		dag := &v1alpha1.DAG{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test_dag_Normal",
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
			DagName: "test_dag_Normal",
		}

		runID, err := dm.CreateDAGRun(context.Background(), "name_Normal", dagRun, map[string]v1alpha1.ParameterSpec{})
		require.NoError(t, err)

		taskRunID, err := dm.MarkTaskAsStarted(context.Background(), runID, 1)
		assert.NoError(t, err)
		assert.NotEqual(t, 0, taskRunID)

		err = dm.MarkPodStatus(context.Background(), types.UID(uuid.New().String()), "pod-one", 1, v1.PodPending, time.Now(), nil, "default")
		require.NoError(t, err)
	})

}

func testDAGManagerMarkPodStatus_Insert_Multiple(t *testing.T, dm db.DBDAGManager) {
	t.Run("Successful Path Twice", func(t *testing.T) {
		dag := &v1alpha1.DAG{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test_dag_Normal_twice",
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
			DagName: "test_dag_Normal_twice",
		}

		runID, err := dm.CreateDAGRun(context.Background(), "name_Normal_twice", dagRun, map[string]v1alpha1.ParameterSpec{})
		require.NoError(t, err)

		taskRunID, err := dm.MarkTaskAsStarted(context.Background(), runID, 2)
		assert.NoError(t, err)
		assert.NotEqual(t, 0, taskRunID)

		uid := types.UID(uuid.New().String())
		err = dm.MarkPodStatus(context.Background(), uid, "pod-two", 2, v1.PodPending, time.Now(), nil, "default")
		require.NoError(t, err)

		err = dm.MarkPodStatus(context.Background(), uid, "pod-two", 2, v1.PodSucceeded, time.Now().Add(time.Hour), nil, "default")
		require.NoError(t, err)
	})

}

func testDAGManagerSoftDeleteDAG_Exists(t *testing.T, dm db.DBDAGManager) {
	t.Run("Successful", func(t *testing.T) {
		dag := &v1alpha1.DAG{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test_dag_Normal",
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

		err = dm.SoftDeleteDAG(context.Background(), dag.Name, "default")
		require.NoError(t, err)
	})
}

func testDAGManagerSoftDeleteDAG_Does_Not_Exist(t *testing.T, dm db.DBDAGManager) {
	t.Run("Does not exist", func(t *testing.T) {
		err := dm.SoftDeleteDAG(context.Background(), "random name", "default")
		require.Error(t, err)
	})

}

func testDAGManagerSoftDeleteDAG_Noop_on_double_delete(t *testing.T, dm db.DBDAGManager) {
	t.Run("Successful Noop", func(t *testing.T) {
		dag := &v1alpha1.DAG{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test_dag_Noop",
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

		err = dm.SoftDeleteDAG(context.Background(), dag.Name, "default")
		require.NoError(t, err)

		err = dm.SoftDeleteDAG(context.Background(), dag.Name, "default")
		require.NoError(t, err)
	})
}

func testDAGManagerFindExistingDAGRun_Exists(t *testing.T, dm db.DBDAGManager) {
	t.Run("Successful Path Exists", func(t *testing.T) {
		dag := &v1alpha1.DAG{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test_dag_exists",
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
			DagName: "test_dag_exists",
		}

		dagrunName := "test_dag_exists_run"
		_, err = dm.CreateDAGRun(context.Background(), dagrunName, dagRun, map[string]v1alpha1.ParameterSpec{})
		require.NoError(t, err)

		ok, err := dm.FindExistingDAGRun(context.Background(), dagrunName)
		assert.NoError(t, err)
		assert.True(t, ok)
	})
}

func testDAGManagerFindExistingDAGRun_Not_Exists(t *testing.T, dm db.DBDAGManager) {
	t.Run("Successful Path Not Exists", func(t *testing.T) {
		dagrunName := "test_dag_exists_run"

		ok, err := dm.FindExistingDAGRun(context.Background(), dagrunName)
		assert.NoError(t, err)
		assert.True(t, ok)
	})
}
