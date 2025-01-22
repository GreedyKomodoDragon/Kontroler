package db_test

import (
	"context"
	"testing"
	"time"

	"kontroler-controller/api/v1alpha1"
	"kontroler-controller/internal/db"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Test case: IncrementAttempts increases the attempts count by 1
func testDAGManagerInsertDag_TaskRef(t *testing.T, dm db.DBDAGManager) {
	t.Run("insert dag with taskref", func(t *testing.T) {
		// Add task to be retrieved
		task := &v1alpha1.DagTask{
			ObjectMeta: metav1.ObjectMeta{
				Name: "retrieval_task",
			},
			Spec: v1alpha1.DagTaskSpec{
				Parameters: []string{},
				Command:    []string{"echo", "Hello"},
				Args:       []string{"arg1", "arg2"},
				Image:      "busybox",
			},
		}

		namespace := "default"
		require.NoError(t, dm.AddTask(context.Background(), task, namespace))

		dag := &v1alpha1.DAG{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test_dag",
			},
			Spec: v1alpha1.DAGSpec{
				Schedule: "*/5 * * * *",
				Task: []v1alpha1.TaskSpec{
					{
						Name: "task1",
						TaskRef: &v1alpha1.TaskRef{
							Name:    "retrieval_task",
							Version: 1,
						},
					},
					{
						Name: "task2",
						TaskRef: &v1alpha1.TaskRef{
							Name:    "retrieval_task",
							Version: 1,
						},
					},
					{
						Name: "task3",
						TaskRef: &v1alpha1.TaskRef{
							Name:    "retrieval_task",
							Version: 1,
						},
						RunAfter: []string{"task1", "task2"},
					},
				},
			},
		}

		require.NoError(t, dm.InsertDAG(context.Background(), dag, namespace))

		tasks, err := dm.GetStartingTasks(context.Background(), "test_dag")
		require.NoError(t, err)
		require.NotEmpty(t, tasks)
		require.Len(t, tasks, 2)

		assert.Equal(t, tasks[0].Command, []string{"echo", "Hello"})
		assert.Equal(t, tasks[0].Args, []string{"arg1", "arg2"})
		assert.Equal(t, tasks[0].Image, "busybox")

		assert.Equal(t, tasks[1].Command, []string{"echo", "Hello"})
		assert.Equal(t, tasks[1].Args, []string{"arg1", "arg2"})
		assert.Equal(t, tasks[1].Image, "busybox")

		assert.Contains(t, []string{tasks[0].Name, tasks[1].Name}, "task1")
		assert.Contains(t, []string{tasks[0].Name, tasks[1].Name}, "task2")
	})
}

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

		runID, _, err := dm.CreateDAGRun(context.Background(), "name", dagRun, map[string]v1alpha1.ParameterSpec{})
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

		runID, _, err := dm.CreateDAGRun(context.Background(), "name_2", dagRun, map[string]v1alpha1.ParameterSpec{})
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

		runID, _, err := dm.CreateDAGRun(context.Background(), "name", dagRun, map[string]v1alpha1.ParameterSpec{})
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

		runID, _, err := dm.CreateDAGRun(context.Background(), "name_MisMatchCode", dagRun, map[string]v1alpha1.ParameterSpec{})
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

		runID, _, err := dm.CreateDAGRun(context.Background(), "name_ValidCodeButNoAttemptsLeft", dagRun, map[string]v1alpha1.ParameterSpec{})
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

		runID, _, err := dm.CreateDAGRun(context.Background(), "name_Normal", dagRun, map[string]v1alpha1.ParameterSpec{})
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

		runID, _, err := dm.CreateDAGRun(context.Background(), "name_Normal", dagRun, map[string]v1alpha1.ParameterSpec{})
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

		runID, _, err := dm.CreateDAGRun(context.Background(), "name_Normal_twice", dagRun, map[string]v1alpha1.ParameterSpec{})
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

func testDAGManagerDeleteDAG_UsingTaskRefs_Not_Needed(t *testing.T, dm db.DBDAGManager) {
	t.Run("Successful", func(t *testing.T) {
		task := &v1alpha1.DagTask{
			ObjectMeta: metav1.ObjectMeta{
				Name: "retrieval_task",
			},
			Spec: v1alpha1.DagTaskSpec{
				Parameters: []string{},
				Command:    []string{"echo", "Hello"},
				Args:       []string{"arg1", "arg2"},
				Image:      "busybox",
			},
		}

		namespace := "default"
		require.NoError(t, dm.AddTask(context.Background(), task, namespace))

		taskTwo := &v1alpha1.DagTask{
			ObjectMeta: metav1.ObjectMeta{
				Name: "retrieval_task_two",
			},
			Spec: v1alpha1.DagTaskSpec{
				Parameters: []string{},
				Command:    []string{"echo", "Hello"},
				Args:       []string{"arg1", "arg2"},
				Image:      "busybox",
			},
		}

		require.NoError(t, dm.AddTask(context.Background(), taskTwo, namespace))

		dag := &v1alpha1.DAG{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test_dag",
			},
			Spec: v1alpha1.DAGSpec{
				Schedule: "*/5 * * * *",
				Task: []v1alpha1.TaskSpec{
					{
						Name: "task1",
						TaskRef: &v1alpha1.TaskRef{
							Name:    "retrieval_task",
							Version: 1,
						},
					},
					{
						Name: "task2",
						TaskRef: &v1alpha1.TaskRef{
							Name:    "retrieval_task",
							Version: 1,
						},
					},
					{
						Name: "task3",
						TaskRef: &v1alpha1.TaskRef{
							Name:    "retrieval_task",
							Version: 1,
						},
						RunAfter: []string{"task1", "task2"},
					},
				},
			},
		}

		err := dm.InsertDAG(context.Background(), dag, "default")
		require.NoError(t, err)

		tasks, err := dm.DeleteDAG(context.Background(), dag.Name, "default")
		require.NoError(t, err)
		require.Len(t, tasks, 1)
		require.Equal(t, tasks[0], "retrieval_task")
	})
}

func testDAGManagerDeleteDAG_UsingTaskRefs_Old_Version_Not_Needed(t *testing.T, dm db.DBDAGManager) {
	t.Run("Successful", func(t *testing.T) {
		task := &v1alpha1.DagTask{
			ObjectMeta: metav1.ObjectMeta{
				Name: "retrieval_task",
			},
			Spec: v1alpha1.DagTaskSpec{
				Parameters: []string{},
				Command:    []string{"echo", "Hello"},
				Args:       []string{"arg1", "arg2"},
				Image:      "busybox",
			},
		}

		namespace := "default"
		require.NoError(t, dm.AddTask(context.Background(), task, namespace))

		taskTwo := &v1alpha1.DagTask{
			ObjectMeta: metav1.ObjectMeta{
				Name: "retrieval_task_two",
			},
			Spec: v1alpha1.DagTaskSpec{
				Parameters: []string{},
				Command:    []string{"echo", "Hello"},
				Args:       []string{"arg1", "arg2"},
				Image:      "busybox",
			},
		}

		require.NoError(t, dm.AddTask(context.Background(), taskTwo, namespace))

		dag := &v1alpha1.DAG{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test_dag",
			},
			Spec: v1alpha1.DAGSpec{
				Schedule: "*/5 * * * *",
				Task: []v1alpha1.TaskSpec{
					{
						Name: "task1",
						TaskRef: &v1alpha1.TaskRef{
							Name:    "retrieval_task",
							Version: 1,
						},
					},
					{
						Name: "task2",
						TaskRef: &v1alpha1.TaskRef{
							Name:    "retrieval_task",
							Version: 1,
						},
					},
					{
						Name: "task3",
						TaskRef: &v1alpha1.TaskRef{
							Name:    "retrieval_task",
							Version: 1,
						},
						RunAfter: []string{"task1", "task2"},
					},
				},
			},
		}

		err := dm.InsertDAG(context.Background(), dag, "default")
		require.NoError(t, err)

		tasks, err := dm.DeleteDAG(context.Background(), dag.Name, "default")
		require.NoError(t, err)
		require.Len(t, tasks, 1)
		require.Equal(t, "retrieval_task", tasks[0])
	})
}

func testDAGManagerDeleteDAG_UsingTaskRefs_Old_Version_Needed(t *testing.T, dm db.DBDAGManager) {
	t.Run("Successful", func(t *testing.T) {
		task := &v1alpha1.DagTask{
			ObjectMeta: metav1.ObjectMeta{
				Name: "retrieval_task_needed",
			},
			Spec: v1alpha1.DagTaskSpec{
				Parameters: []string{},
				Command:    []string{"echo", "Hello"},
				Args:       []string{"arg1", "arg2"},
				Image:      "busybox",
			},
		}

		namespace := "default"
		require.NoError(t, dm.AddTask(context.Background(), task, namespace))

		taskTwo := &v1alpha1.DagTask{
			ObjectMeta: metav1.ObjectMeta{
				Name: "retrieval_task_two_needed",
			},
			Spec: v1alpha1.DagTaskSpec{
				Parameters: []string{},
				Command:    []string{"echo", "Hello"},
				Args:       []string{"arg1", "arg2"},
				Image:      "busybox",
			},
		}

		require.NoError(t, dm.AddTask(context.Background(), taskTwo, namespace))

		taskTwoRev := &v1alpha1.DagTask{
			ObjectMeta: metav1.ObjectMeta{
				Name: "retrieval_task_two_needed",
			},
			Spec: v1alpha1.DagTaskSpec{
				Parameters: []string{},
				Command:    []string{"echo", "Hello two"},
				Args:       []string{"arg1", "arg2"},
				Image:      "busybox",
			},
		}

		require.NoError(t, dm.AddTask(context.Background(), taskTwoRev, namespace))

		dag := &v1alpha1.DAG{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test_dag",
			},
			Spec: v1alpha1.DAGSpec{
				Schedule: "*/5 * * * *",
				Task: []v1alpha1.TaskSpec{
					{
						Name: "task1",
						TaskRef: &v1alpha1.TaskRef{
							Name:    "retrieval_task_two_needed",
							Version: 1,
						},
					},
					{
						Name: "task2",
						TaskRef: &v1alpha1.TaskRef{
							Name:    "retrieval_task_two_needed",
							Version: 1,
						},
					},
					{
						Name: "task3",
						TaskRef: &v1alpha1.TaskRef{
							Name:    "retrieval_task_two_needed",
							Version: 1,
						},
						RunAfter: []string{"task1", "task2"},
					},
				},
			},
		}

		err := dm.InsertDAG(context.Background(), dag, "default")
		require.NoError(t, err)

		dagTwo := &v1alpha1.DAG{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test_dag_needed",
			},
			Spec: v1alpha1.DAGSpec{
				Schedule: "*/5 * * * *",
				Task: []v1alpha1.TaskSpec{
					{
						Name: "task1",
						TaskRef: &v1alpha1.TaskRef{
							Name:    "retrieval_task_two_needed",
							Version: 2,
						},
					},
					{
						Name: "task2",
						TaskRef: &v1alpha1.TaskRef{
							Name:    "retrieval_task_two_needed",
							Version: 2,
						},
					},
					{
						Name: "task3",
						TaskRef: &v1alpha1.TaskRef{
							Name:    "retrieval_task_two_needed",
							Version: 2,
						},
						RunAfter: []string{"task1", "task2"},
					},
				},
			},
		}

		err = dm.InsertDAG(context.Background(), dagTwo, "default")
		require.NoError(t, err)

		tasks, err := dm.DeleteDAG(context.Background(), dagTwo.Name, "default")
		require.NoError(t, err)
		require.Len(t, tasks, 0)
	})
}

func testDAGManagerDeleteDAG_Exists(t *testing.T, dm db.DBDAGManager) {
	t.Run("Successful", func(t *testing.T) {
		task := &v1alpha1.DagTask{
			ObjectMeta: metav1.ObjectMeta{
				Name: "retrieval_task",
			},
			Spec: v1alpha1.DagTaskSpec{
				Parameters: []string{},
				Command:    []string{"echo", "Hello"},
				Args:       []string{"arg1", "arg2"},
				Image:      "busybox",
			},
		}

		namespace := "default"
		require.NoError(t, dm.AddTask(context.Background(), task, namespace))

		taskTwo := &v1alpha1.DagTask{
			ObjectMeta: metav1.ObjectMeta{
				Name: "retrieval_task_two",
			},
			Spec: v1alpha1.DagTaskSpec{
				Parameters: []string{},
				Command:    []string{"echo", "Hello"},
				Args:       []string{"arg1", "arg2"},
				Image:      "busybox",
			},
		}

		require.NoError(t, dm.AddTask(context.Background(), taskTwo, namespace))

		dag := &v1alpha1.DAG{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test_dag",
			},
			Spec: v1alpha1.DAGSpec{
				Schedule: "*/5 * * * *",
				Task: []v1alpha1.TaskSpec{
					{
						Name: "task1",
						TaskRef: &v1alpha1.TaskRef{
							Name:    "retrieval_task",
							Version: 1,
						},
					},
					{
						Name: "task2",
						TaskRef: &v1alpha1.TaskRef{
							Name:    "retrieval_task",
							Version: 1,
						},
					},
					{
						Name: "task3",
						TaskRef: &v1alpha1.TaskRef{
							Name:    "retrieval_task",
							Version: 1,
						},
						RunAfter: []string{"task1", "task2"},
					},
				},
			},
		}

		err := dm.InsertDAG(context.Background(), dag, "default")
		require.NoError(t, err)

		tasks, err := dm.DeleteDAG(context.Background(), dag.Name, "default")
		require.NoError(t, err)
		require.Len(t, tasks, 1)
		require.Equal(t, "retrieval_task", tasks[0])
	})
}

func testDAGManagerDeleteDAG_Does_Not_Exist(t *testing.T, dm db.DBDAGManager) {
	t.Run("Does not exist", func(t *testing.T) {
		tasks, err := dm.DeleteDAG(context.Background(), "random name", "default")
		require.Nil(t, err)
		require.Len(t, tasks, 0)
	})

}

func testDAGManagerDeleteDAG_Noop_on_double_delete(t *testing.T, dm db.DBDAGManager) {
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

		_, err = dm.DeleteDAG(context.Background(), dag.Name, "default")
		require.NoError(t, err)

		_, err = dm.DeleteDAG(context.Background(), dag.Name, "default")
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
		_, _, err = dm.CreateDAGRun(context.Background(), dagrunName, dagRun, map[string]v1alpha1.ParameterSpec{})
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

func testDAGManager_AddTask_Success(t *testing.T, dm db.DBDAGManager) {
	t.Run("Add Task Successfully", func(t *testing.T) {
		task := &v1alpha1.DagTask{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test_task",
			},
			Spec: v1alpha1.DagTaskSpec{
				Command:    []string{"echo", "hello"},
				Args:       []string{"world"},
				Image:      "busybox",
				Parameters: []string{"param1"},
				Backoff: v1alpha1.Backoff{
					Limit: 3,
				},
				Conditional: v1alpha1.Conditional{
					Enabled:    true,
					RetryCodes: []int{500},
				},
			},
		}
		namespace := "test_namespace"

		err := dm.AddTask(context.Background(), task, namespace)
		assert.NoError(t, err)
	})
}

func testDAGManager_AddTask_ExistingTask(t *testing.T, dm db.DBDAGManager) {
	t.Run("Add Task with Existing Task Name", func(t *testing.T) {
		task := &v1alpha1.DagTask{
			ObjectMeta: metav1.ObjectMeta{
				Name: "existing_task",
			},
			Spec: v1alpha1.DagTaskSpec{
				Command:    []string{"echo", "updated"},
				Args:       []string{"task"},
				Image:      "busybox:latest",
				Parameters: []string{"param2"},
				Backoff: v1alpha1.Backoff{
					Limit: 2,
				},
				Conditional: v1alpha1.Conditional{
					Enabled:    false,
					RetryCodes: []int{},
				},
			},
		}
		namespace := "test_namespace"

		// Insert the initial task
		err := dm.AddTask(context.Background(), task, namespace)
		assert.NoError(t, err)

		// Insert the initial task
		err = dm.AddTask(context.Background(), task, namespace)
		require.Error(t, err)
		require.Contains(t, err.Error(), "applying the same task")

		// Update with a new version
		task.Spec.Args = []string{"new_version"}
		err = dm.AddTask(context.Background(), task, namespace)
		assert.NoError(t, err)
	})
}

func testDAGManager_GetTaskRefsParameters_Success(t *testing.T, dm db.DBDAGManager) {
	t.Run("Get Parameters for Task Refs Successfully", func(t *testing.T) {
		// Add task to be retrieved
		task := &v1alpha1.DagTask{
			ObjectMeta: metav1.ObjectMeta{
				Name: "retrieval_task",
			},
			Spec: v1alpha1.DagTaskSpec{
				Parameters: []string{"param1", "param2"},
			},
		}
		namespace := "test_namespace"
		err := dm.AddTask(context.Background(), task, namespace)
		require.NoError(t, err)

		taskRefs := []v1alpha1.TaskRef{
			{Name: "retrieval_task", Version: 1},
		}

		params, err := dm.GetTaskRefsParameters(context.Background(), taskRefs)
		assert.NoError(t, err)
		assert.NotNil(t, params)
		assert.Equal(t, []string{"param1", "param2"}, params[taskRefs[0]])
	})
}

func testDAGManager_GetTaskRefsParameters_NonExistentTask(t *testing.T, dm db.DBDAGManager) {
	t.Run("Get Parameters for Non-Existent Task Ref", func(t *testing.T) {
		taskRefs := []v1alpha1.TaskRef{
			{Name: "non_existent_task", Version: 1},
		}

		params, err := dm.GetTaskRefsParameters(context.Background(), taskRefs)
		assert.Error(t, err)
		assert.Nil(t, params)
	})
}

func testDAGManager_Complex_Dag(t *testing.T, dm db.DBDAGManager) {
	t.Run("Checking complex dag works", func(t *testing.T) {
		dag := &v1alpha1.DAG{
			ObjectMeta: metav1.ObjectMeta{
				Name: "dag-sample-long",
			},
			Spec: v1alpha1.DAGSpec{
				Parameters: []v1alpha1.DagParameterSpec{
					{
						Name:              "first",
						DefaultFromSecret: "secret-name",
					},
					{
						Name:         "second",
						DefaultValue: "value",
					},
				},
				Webhook: v1alpha1.Webhook{
					URL:       "http://localhost:8080",
					VerifySSL: false,
				},
				Task: []v1alpha1.TaskSpec{
					{
						Name:       "random",
						Command:    []string{"sh", "-c"},
						Args:       []string{"if [ $((RANDOM%2)) -eq 0 ]; then echo $second; else exit 1; fi"},
						Image:      "alpine:latest",
						Backoff:    v1alpha1.Backoff{Limit: 5},
						Parameters: []string{"second"},
						Conditional: v1alpha1.Conditional{
							Enabled:    true,
							RetryCodes: []int{1},
						},
					},
					{
						Name:     "random-b",
						Command:  []string{"sh", "-c"},
						Args:     []string{"if [ $((RANDOM%2)) -eq 0 ]; then echo 'Hello, World!'; else exit 1; fi"},
						Image:    "alpine:latest",
						RunAfter: []string{"random"},
						Backoff:  v1alpha1.Backoff{Limit: 3},
						Conditional: v1alpha1.Conditional{
							Enabled:    true,
							RetryCodes: []int{1},
						},
						Parameters: []string{"first", "second"},
					},
					{
						Name:     "random-c",
						Command:  []string{"sh", "-c"},
						Args:     []string{"if [ $((RANDOM%2)) -eq 0 ]; then echo 'Hello, World!'; else exit 1; fi"},
						Image:    "alpine:latest",
						RunAfter: []string{"random"},
						Backoff:  v1alpha1.Backoff{Limit: 3},
						Conditional: v1alpha1.Conditional{
							Enabled:    true,
							RetryCodes: []int{1},
						},
					},
					{
						Name:     "random-d",
						Command:  []string{"sh", "-c"},
						Args:     []string{"if [ $((RANDOM%2)) -eq 0 ]; then echo 'Hello, World!'; else exit 1; fi"},
						Image:    "alpine:latest",
						RunAfter: []string{"random"},
						Backoff:  v1alpha1.Backoff{Limit: 3},
						Conditional: v1alpha1.Conditional{
							Enabled:    true,
							RetryCodes: []int{1},
						},
					},
					{
						Name:     "random-e",
						Command:  []string{"sh", "-c"},
						Args:     []string{"if [ $((RANDOM%2)) -eq 0 ]; then echo 'Hello, World!'; else exit 1; fi"},
						Image:    "alpine:latest",
						RunAfter: []string{"random-d"},
						Backoff:  v1alpha1.Backoff{Limit: 3},
						Conditional: v1alpha1.Conditional{
							Enabled:    true,
							RetryCodes: []int{1},
						},
					},
					{
						Name:     "random-f",
						Command:  []string{"sh", "-c"},
						Args:     []string{"if [ $((RANDOM%2)) -eq 0 ]; then echo 'Hello, World!'; else exit 1; fi"},
						Image:    "alpine:latest",
						RunAfter: []string{"random-e"},
						Backoff:  v1alpha1.Backoff{Limit: 3},
						Conditional: v1alpha1.Conditional{
							Enabled:    true,
							RetryCodes: []int{1},
						},
					},
					{
						Name:     "random-g",
						Command:  []string{"sh", "-c"},
						Args:     []string{"if [ $((RANDOM%2)) -eq 0 ]; then echo 'Hello, World!'; else exit 1; fi"},
						Image:    "alpine:latest",
						RunAfter: []string{"random-f"},
						Backoff:  v1alpha1.Backoff{Limit: 3},
						Conditional: v1alpha1.Conditional{
							Enabled:    true,
							RetryCodes: []int{1},
						},
					},
					{
						Name:     "random-h",
						Command:  []string{"sh", "-c"},
						Args:     []string{"if [ $((RANDOM%2)) -eq 0 ]; then echo 'Hello, World!'; else exit 1; fi"},
						Image:    "alpine:latest",
						RunAfter: []string{"random-e"},
						Backoff:  v1alpha1.Backoff{Limit: 3},
						Conditional: v1alpha1.Conditional{
							Enabled:    true,
							RetryCodes: []int{1},
						},
					},
					{
						Name:     "random-i",
						Command:  []string{"sh", "-c"},
						Args:     []string{"if [ $((RANDOM%2)) -eq 0 ]; then echo 'Hello, World!'; else exit 1; fi"},
						Image:    "alpine:latest",
						RunAfter: []string{"random-c"},
						Backoff:  v1alpha1.Backoff{Limit: 3},
						Conditional: v1alpha1.Conditional{
							Enabled:    true,
							RetryCodes: []int{1},
						},
					},
					{
						Name:     "random-j",
						Command:  []string{"sh", "-c"},
						Args:     []string{"if [ $((RANDOM%2)) -eq 0 ]; then echo 'Hello, World!'; else exit 1; fi"},
						Image:    "alpine:latest",
						RunAfter: []string{"random-b"},
						Backoff:  v1alpha1.Backoff{Limit: 3},
						Conditional: v1alpha1.Conditional{
							Enabled:    true,
							RetryCodes: []int{1},
						},
					},
				},
			},
		}

		dag2 := &v1alpha1.DAG{
			ObjectMeta: metav1.ObjectMeta{
				Name: "dag-sample-longv",
			},
			Spec: v1alpha1.DAGSpec{
				Parameters: []v1alpha1.DagParameterSpec{
					{
						Name:              "first",
						DefaultFromSecret: "secret-name",
					},
					{
						Name:         "second",
						DefaultValue: "value",
					},
				},
				Task: []v1alpha1.TaskSpec{
					{
						Name:       "random",
						Command:    []string{"sh", "-c"},
						Args:       []string{"if [ $((RANDOM%2)) -eq 0 ]; then echo $second; else exit 1; fi"},
						Image:      "alpine:latest",
						Backoff:    v1alpha1.Backoff{Limit: 5},
						Parameters: []string{"second"},
						Conditional: v1alpha1.Conditional{
							Enabled:    true,
							RetryCodes: []int{1},
						},
					},
					{
						Name:     "random-b",
						Command:  []string{"sh", "-c"},
						Args:     []string{"if [ $((RANDOM%2)) -eq 0 ]; then echo 'Hello, World!'; else exit 1; fi"},
						Image:    "alpine:latest",
						RunAfter: []string{"random"},
						Backoff:  v1alpha1.Backoff{Limit: 3},
						Conditional: v1alpha1.Conditional{
							Enabled:    true,
							RetryCodes: []int{1},
						},
						Parameters: []string{"first", "second"},
					},
					{
						Name:     "random-c",
						Command:  []string{"sh", "-c"},
						Args:     []string{"if [ $((RANDOM%2)) -eq 0 ]; then echo 'Hello, World!'; else exit 1; fi"},
						Image:    "alpine:latest",
						RunAfter: []string{"random"},
						Backoff:  v1alpha1.Backoff{Limit: 3},
						Conditional: v1alpha1.Conditional{
							Enabled:    true,
							RetryCodes: []int{1},
						},
					},
					{
						Name:     "random-d",
						Command:  []string{"sh", "-c"},
						Args:     []string{"if [ $((RANDOM%2)) -eq 0 ]; then echo 'Hello, World!'; else exit 1; fi"},
						Image:    "alpine:latest",
						RunAfter: []string{"random"},
						Backoff:  v1alpha1.Backoff{Limit: 3},
						Conditional: v1alpha1.Conditional{
							Enabled:    true,
							RetryCodes: []int{1},
						},
					},
					{
						Name:     "random-e",
						Command:  []string{"sh", "-c"},
						Args:     []string{"if [ $((RANDOM%2)) -eq 0 ]; then echo 'Hello, World!'; else exit 1; fi"},
						Image:    "alpine:latest",
						RunAfter: []string{"random-d"},
						Backoff:  v1alpha1.Backoff{Limit: 3},
						Conditional: v1alpha1.Conditional{
							Enabled:    true,
							RetryCodes: []int{1},
						},
					},
					{
						Name:     "random-f",
						Command:  []string{"sh", "-c"},
						Args:     []string{"if [ $((RANDOM%2)) -eq 0 ]; then echo 'Hello, World!'; else exit 1; fi"},
						Image:    "alpine:latest",
						RunAfter: []string{"random-e"},
						Backoff:  v1alpha1.Backoff{Limit: 3},
						Conditional: v1alpha1.Conditional{
							Enabled:    true,
							RetryCodes: []int{1},
						},
					},
					{
						Name:     "random-g",
						Command:  []string{"sh", "-c"},
						Args:     []string{"if [ $((RANDOM%2)) -eq 0 ]; then echo 'Hello, World!'; else exit 1; fi"},
						Image:    "alpine:latest",
						RunAfter: []string{"random-f"},
						Backoff:  v1alpha1.Backoff{Limit: 3},
						Conditional: v1alpha1.Conditional{
							Enabled:    true,
							RetryCodes: []int{1},
						},
					},
					{
						Name:     "random-h",
						Command:  []string{"sh", "-c"},
						Args:     []string{"if [ $((RANDOM%2)) -eq 0 ]; then echo 'Hello, World!'; else exit 1; fi"},
						Image:    "alpine:latest",
						RunAfter: []string{"random-e"},
						Backoff:  v1alpha1.Backoff{Limit: 3},
						Conditional: v1alpha1.Conditional{
							Enabled:    true,
							RetryCodes: []int{1},
						},
					},
					{
						Name:     "random-i",
						Command:  []string{"sh", "-c"},
						Args:     []string{"if [ $((RANDOM%2)) -eq 0 ]; then echo 'Hello, World!'; else exit 1; fi"},
						Image:    "alpine:latest",
						RunAfter: []string{"random-c"},
						Backoff:  v1alpha1.Backoff{Limit: 3},
						Conditional: v1alpha1.Conditional{
							Enabled:    true,
							RetryCodes: []int{1},
						},
					},
					{
						Name:     "random-j",
						Command:  []string{"sh", "-c"},
						Args:     []string{"if [ $((RANDOM%2)) -eq 0 ]; then echo 'Hello, World!'; else exit 1; fi"},
						Image:    "alpine:latest",
						RunAfter: []string{"random-b"},
						Backoff:  v1alpha1.Backoff{Limit: 3},
						Conditional: v1alpha1.Conditional{
							Enabled:    true,
							RetryCodes: []int{1},
						},
					},
				},
			},
		}

		err := dm.InsertDAG(context.Background(), dag, "default")
		require.NoError(t, err)

		err = dm.InsertDAG(context.Background(), dag2, "default")
		require.NoError(t, err)

		dagRun := &v1alpha1.DagRunSpec{
			DagName: "dag-sample-long",
		}

		runID, _, err := dm.CreateDAGRun(context.Background(), "name", dagRun, map[string]v1alpha1.ParameterSpec{})
		require.NoError(t, err)

		tasks, err := dm.GetStartingTasks(context.Background(), "dag-sample-long")
		require.NoError(t, err)
		require.NotEmpty(t, tasks)
		require.Len(t, tasks, 1)
		require.Equal(t, tasks[0].Name, "random")

		taskRunID, err := dm.MarkTaskAsStarted(context.Background(), runID, tasks[0].Id)
		require.NoError(t, err)

		tasksSecond, err := dm.MarkSuccessAndGetNextTasks(context.Background(), taskRunID)
		require.NoError(t, err)
		require.Len(t, tasksSecond, 3)

		taskRunOne, err := dm.MarkTaskAsStarted(context.Background(), runID, tasksSecond[0].Id)
		require.NoError(t, err)

		taskRunTwo, err := dm.MarkTaskAsStarted(context.Background(), runID, tasksSecond[1].Id)
		require.NoError(t, err)

		taskRunThree, err := dm.MarkTaskAsStarted(context.Background(), runID, tasksSecond[2].Id)
		require.NoError(t, err)

		tasksSecondOne, err := dm.MarkSuccessAndGetNextTasks(context.Background(), taskRunOne)
		require.NoError(t, err)
		require.NotEmpty(t, tasksSecondOne)

		tasksSecondTwo, err := dm.MarkSuccessAndGetNextTasks(context.Background(), taskRunTwo)
		require.NoError(t, err)
		require.NotEmpty(t, tasksSecondTwo)

		tasksSecondThree, err := dm.MarkSuccessAndGetNextTasks(context.Background(), taskRunThree)
		require.NoError(t, err)
		require.Len(t, tasksSecondThree, 3)
	})
}

func testDAGManager_CreateDagRun_Sequential(t *testing.T, dm db.DBDAGManager) {
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
				{
					Name:     "task2",
					Command:  []string{"echo", "Hello"},
					Args:     []string{"arg1", "arg2"},
					Image:    "busybox",
					RunAfter: []string{"task1"},
				},
				{
					Name:     "task3",
					Command:  []string{"echo"},
					Args:     []string{"Goodbye, World!"},
					Image:    "alpine:latest",
					RunAfter: []string{"task1"},
				},
			},
		},
	}

	err := dm.InsertDAG(context.Background(), dag, "default")
	require.NoError(t, err)

	dagRun := &v1alpha1.DagRunSpec{
		DagName: "test_dag",
	}

	runID, _, err := dm.CreateDAGRun(context.Background(), "name", dagRun, map[string]v1alpha1.ParameterSpec{})
	assert.NoError(t, err)
	assert.NotEqual(t, 0, runID)

	tasks, err := dm.GetStartingTasks(context.Background(), "test_dag")
	require.NoError(t, err)
	require.NotEmpty(t, tasks)
	require.Len(t, tasks, 1)
	require.Equal(t, tasks[0].Name, "task1")

	taskRunID, err := dm.MarkTaskAsStarted(context.Background(), runID, tasks[0].Id)
	require.NoError(t, err)

	tasksSecond, err := dm.MarkSuccessAndGetNextTasks(context.Background(), taskRunID)
	require.NoError(t, err)
	require.Len(t, tasksSecond, 2)

	taskRunTwoID, err := dm.MarkTaskAsStarted(context.Background(), runID, tasksSecond[0].Id)
	require.NoError(t, err)

	_, err = dm.MarkTaskAsStarted(context.Background(), runID, tasksSecond[1].Id)
	require.NoError(t, err)

	taskTwoFollowingTasks, err := dm.MarkSuccessAndGetNextTasks(context.Background(), taskRunTwoID)
	require.NoError(t, err)
	require.Len(t, taskTwoFollowingTasks, 0)

	runIDTwo, _, err := dm.CreateDAGRun(context.Background(), "nametwo", dagRun, map[string]v1alpha1.ParameterSpec{})
	assert.NoError(t, err)
	assert.Equal(t, 2, runIDTwo)

	tasksTwo, err := dm.GetStartingTasks(context.Background(), "test_dag")
	require.NoError(t, err)
	require.NotEmpty(t, tasksTwo)
	require.Len(t, tasksTwo, 1)
	require.Equal(t, tasksTwo[0].Name, "task1")

	taskRunIDTwo, err := dm.MarkTaskAsStarted(context.Background(), runIDTwo, tasksTwo[0].Id)
	require.NoError(t, err)

	tasksSecondTwo, err := dm.MarkSuccessAndGetNextTasks(context.Background(), taskRunIDTwo)
	require.NoError(t, err)
	require.Len(t, tasksSecondTwo, 2)
}

func testDAGManager_CreateDagRun_Scripts(t *testing.T, dm db.DBDAGManager) {
	dag := &v1alpha1.DAG{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test_dag",
		},
		Spec: v1alpha1.DAGSpec{
			Schedule: "*/5 * * * *",
			Task: []v1alpha1.TaskSpec{
				{
					Name:   "task1",
					Script: "echo Hello",
					Image:  "busybox",
				},
				{
					Name:     "task2",
					Script:   "echo Hello",
					Image:    "busybox",
					RunAfter: []string{"task1"},
				},
				{
					Name:     "task3",
					Script:   "echo Hello",
					Image:    "alpine:latest",
					RunAfter: []string{"task1"},
				},
			},
		},
	}

	err := dm.InsertDAG(context.Background(), dag, "default")
	require.NoError(t, err)

	dagRun := &v1alpha1.DagRunSpec{
		DagName: "test_dag",
	}

	runID, _, err := dm.CreateDAGRun(context.Background(), "name", dagRun, map[string]v1alpha1.ParameterSpec{})
	assert.NoError(t, err)
	assert.NotEqual(t, 0, runID)

	tasks, err := dm.GetStartingTasks(context.Background(), "test_dag")
	require.NoError(t, err)
	require.NotEmpty(t, tasks)
	require.Len(t, tasks, 1)
	require.Equal(t, tasks[0].Name, "task1")
	require.Equal(t, tasks[0].Script, "echo Hello")

	taskRunID, err := dm.MarkTaskAsStarted(context.Background(), runID, tasks[0].Id)
	require.NoError(t, err)

	tasksSecond, err := dm.MarkSuccessAndGetNextTasks(context.Background(), taskRunID)
	require.NoError(t, err)
	require.Len(t, tasksSecond, 2)
	require.Equal(t, tasksSecond[0].Script, "echo Hello")
	require.Equal(t, tasksSecond[1].Script, "echo Hello")

	taskRunTwoID, err := dm.MarkTaskAsStarted(context.Background(), runID, tasksSecond[0].Id)
	require.NoError(t, err)

	_, err = dm.MarkTaskAsStarted(context.Background(), runID, tasksSecond[1].Id)
	require.NoError(t, err)

	taskTwoFollowingTasks, err := dm.MarkSuccessAndGetNextTasks(context.Background(), taskRunTwoID)
	require.NoError(t, err)
	require.Len(t, taskTwoFollowingTasks, 0)

	runIDTwo, _, err := dm.CreateDAGRun(context.Background(), "nametwo", dagRun, map[string]v1alpha1.ParameterSpec{})
	assert.NoError(t, err)
	assert.Equal(t, 2, runIDTwo)

	tasksTwo, err := dm.GetStartingTasks(context.Background(), "test_dag")
	require.NoError(t, err)
	require.NotEmpty(t, tasksTwo)
	require.Len(t, tasksTwo, 1)
	require.Equal(t, tasksTwo[0].Name, "task1")

	taskRunIDTwo, err := dm.MarkTaskAsStarted(context.Background(), runIDTwo, tasksTwo[0].Id)
	require.NoError(t, err)

	tasksSecondTwo, err := dm.MarkSuccessAndGetNextTasks(context.Background(), taskRunIDTwo)
	require.NoError(t, err)
	require.Len(t, tasksSecondTwo, 2)
}
