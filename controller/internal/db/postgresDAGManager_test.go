package db_test

import (
	"context"
	"database/sql"
	"testing"

	"kontroler-controller/api/v1alpha1"
	"kontroler-controller/internal/db"
	"kontroler-controller/internal/utils"

	cron "github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUpsertDAG(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()

	parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	manager, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	if err != nil {
		t.Fatal(err)
	}

	if err := manager.InitaliseDatabase(context.Background()); err != nil {
		t.Fatal(err)
	}

	dag := &v1alpha1.DAG{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-dag",
		},
		Spec: v1alpha1.DAGSpec{
			Schedule: "*/5 * * * *",
			Task: []v1alpha1.TaskSpec{
				{
					Name:    "task1",
					Command: []string{"echo"},
					Args:    []string{"Hello, World!"},
					Image:   "alpine:latest",
				},
				{
					Name:     "task2",
					Command:  []string{"echo"},
					Args:     []string{"Goodbye, World!"},
					Image:    "alpine:latest",
					RunAfter: []string{"task1"},
				},
			},
		},
	}

	// Test inserting a new DAG
	err = manager.InsertDAG(context.Background(), dag, "default")
	assert.NoError(t, err, "Failed to insert new DAG")

	// Verify the DAG was inserted
	var dagID int
	err = pool.QueryRow(context.Background(), "SELECT dag_id FROM DAGs WHERE name = $1", dag.Name).Scan(&dagID)
	assert.NoError(t, err, "Failed to query inserted DAG")
	assert.NotZero(t, dagID, "DAG ID should not be zero")

	// Test updating the existing DAG
	dag.Spec.Schedule = "*/10 * * * *"
	err = manager.InsertDAG(context.Background(), dag, "default")
	assert.NoError(t, err, "Failed to update existing DAG")

	// Verify the DAG was updated
	var updatedSchedule string
	err = pool.QueryRow(context.Background(), "SELECT schedule FROM DAGs WHERE dag_id = $1", dagID).Scan(&updatedSchedule)
	assert.NoError(t, err, "Failed to query updated DAG")
	assert.Equal(t, "*/5 * * * *", updatedSchedule, "DAG schedule was updated")

	// Test DAG with additional task
	dag.Spec.Task = append(dag.Spec.Task, v1alpha1.TaskSpec{
		Name:     "task3",
		Command:  []string{"echo"},
		Args:     []string{"Another Task"},
		Image:    "alpine:latest",
		RunAfter: []string{"task2"},
	})
	err = manager.InsertDAG(context.Background(), dag, "default")
	assert.NoError(t, err, "Failed to update DAG with additional task")

	// Verify the new task was inserted
	var taskName string
	err = pool.QueryRow(context.Background(), "SELECT name FROM DAG_Tasks WHERE name = 'task3'").Scan(&taskName)
	assert.NoError(t, err, "Failed to query new task")
	assert.NotZero(t, taskName, "Task name should not be zero")

	// confirm there are now three versions
	var version int
	row := pool.QueryRow(context.Background(), "SELECT version FROM DAGs ORDER BY version DESC")
	assert.NoError(t, row.Scan(&version), "Failed to get third version")
	assert.Equal(t, 2, version)
}

func TestPostgresDAGManager_InsertDAG(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

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

	err = dm.InsertDAG(context.Background(), dag, "default")
	assert.NoError(t, err)
}

func TestPostgresDAGManager_CreateDAGRun(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

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

	err = dm.InsertDAG(context.Background(), dag, "default")
	require.NoError(t, err)

	dagRun := &v1alpha1.DagRunSpec{
		DagName: "test_dag",
	}

	runID, err := dm.CreateDAGRun(context.Background(), "name", dagRun, map[string]v1alpha1.ParameterSpec{}, nil)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, runID)
}

func TestPostgresDAGManager_GetStartingTasks(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	dag := &v1alpha1.DAG{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test_dag",
		},
		Spec: v1alpha1.DAGSpec{
			Schedule: "*/5 * * * *",
			Task: []v1alpha1.TaskSpec{
				{
					Name:    "task1",
					Command: []string{"echo"},
					Args:    []string{"Hello, World!"},
					Image:   "alpine:latest",
				},
				{
					Name:     "task2",
					Command:  []string{"echo"},
					Args:     []string{"Goodbye, World!"},
					Image:    "alpine:latest",
					RunAfter: []string{"task1"},
				},
			},
		},
	}

	err = dm.InsertDAG(context.Background(), dag, "default")
	require.NoError(t, err)

	_, err = dm.CreateDAGRun(context.Background(), "name", &v1alpha1.DagRunSpec{
		DagName:    "test_dag",
		Parameters: []v1alpha1.ParameterSpec{},
	}, map[string]v1alpha1.ParameterSpec{}, nil)
	assert.NoError(t, err)

	tasks, err := dm.GetStartingTasks(context.Background(), "test_dag", 1)
	assert.NoError(t, err)
	assert.NotEmpty(t, tasks)
	assert.Len(t, tasks, 1)
	assert.Equal(t, tasks[0].Name, "task1")
}

func TestPostgresDAGManager_MarkDAGRunOutcome(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

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
					Command:  []string{"echo"},
					Args:     []string{"Goodbye, World!"},
					Image:    "alpine:latest",
					RunAfter: []string{"task1"},
				},
			},
		},
	}

	err = dm.InsertDAG(context.Background(), dag, "default")
	require.NoError(t, err)

	dagRun := &v1alpha1.DagRunSpec{
		DagName: "test_dag",
	}

	runID, err := dm.CreateDAGRun(context.Background(), "name", dagRun, map[string]v1alpha1.ParameterSpec{}, nil)
	require.NoError(t, err)

	err = dm.MarkDAGRunOutcome(context.Background(), runID, "success")
	assert.NoError(t, err)
}

func TestPostgresDAGManager_MarkOutcomeAndGetNextTasks(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

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
					Command:  []string{"echo"},
					Args:     []string{"Goodbye, World!"},
					Image:    "alpine:latest",
					RunAfter: []string{"task1"},
				},
			},
		},
	}

	err = dm.InsertDAG(context.Background(), dag, "default")
	require.NoError(t, err)

	dagRun := &v1alpha1.DagRunSpec{
		DagName: "test_dag",
	}

	runID, err := dm.CreateDAGRun(context.Background(), "name", dagRun, map[string]v1alpha1.ParameterSpec{}, nil)
	require.NoError(t, err)

	tasks, err := dm.GetStartingTasks(context.Background(), "test_dag", 1)
	assert.NoError(t, err)
	assert.NotEmpty(t, tasks)
	assert.Len(t, tasks, 1)
	assert.Equal(t, tasks[0].Name, "task1")

	tasRunID, err := dm.MarkTaskAsStarted(context.Background(), runID, tasks[0].Id)
	require.NoError(t, err)

	tasks, err = dm.MarkSuccessAndGetNextTasks(context.Background(), tasRunID)
	require.NoError(t, err)
	require.NotEmpty(t, tasks)
	require.Len(t, tasks, 1)
	require.Equal(t, tasks[0].Name, "task2")
}

func TestPostgresDAGManager_MarkOutcomeAndGetNextTasks_No_Task_Yet(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

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

	err = dm.InsertDAG(context.Background(), dag, "default")
	require.NoError(t, err)

	dagRun := &v1alpha1.DagRunSpec{
		DagName: "test_dag",
	}

	runID, err := dm.CreateDAGRun(context.Background(), "name", dagRun, map[string]v1alpha1.ParameterSpec{}, nil)
	require.NoError(t, err)

	tasks, err := dm.GetStartingTasks(context.Background(), "test_dag", 1)
	require.NoError(t, err)
	require.NotEmpty(t, tasks)
	require.Len(t, tasks, 2)
	require.ElementsMatch(t, []string{tasks[0].Name, tasks[1].Name}, []string{"task1", "task2"})

	tasRunID, err := dm.MarkTaskAsStarted(context.Background(), runID, tasks[0].Id)
	require.NoError(t, err)

	tasks, err = dm.MarkSuccessAndGetNextTasks(context.Background(), tasRunID)
	require.NoError(t, err)
	require.Empty(t, tasks)
}

func TestPostgresDAGManager_MarkTaskAsStarted(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

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

	err = dm.InsertDAG(context.Background(), dag, "default")
	require.NoError(t, err)

	dagRun := &v1alpha1.DagRunSpec{
		DagName: "test_dag",
	}

	runID, err := dm.CreateDAGRun(context.Background(), "name", dagRun, map[string]v1alpha1.ParameterSpec{}, nil)
	require.NoError(t, err)

	taskID, err := dm.MarkTaskAsStarted(context.Background(), runID, 1)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, taskID)
}

func TestPostgresDAGManager_GetID(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManagerGetID_ReturnsExistingID(t, dm)

	// Clean up the table to ensure no rows are present
	_, err = pool.Exec(context.Background(), "DELETE FROM IdTable")
	require.NoError(t, err)

	testDAGManagerGetID_InsertsAndReturnsNewID(t, dm)

	// Test case: Handle error if database query fails
	t.Run("HandlesQueryError", func(t *testing.T) {
		// Intentionally close the pool to simulate a database error
		pool.Close()

		uniqueID, err := dm.GetID(context.Background())
		assert.Error(t, err)
		assert.Empty(t, uniqueID)
	})
}

func TestPostgresDAGManager_IncrementAttempts(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManagerIncrementAttempts_IncrementAttempts(t, dm)

	// Clean up the table to ensure no rows are present
	attempts := 0
	err = pool.QueryRow(context.Background(), "SELECT attempts FROM Task_Runs where task_run_id = 1;").Scan(&attempts)
	require.NoError(t, err)
	require.Equal(t, 2, attempts)

	testDAGManagerIncrementAttempts_MultipleIncrements(t, dm)

	// Clean up the table to ensure no rows are present
	attempts = 0
	err = pool.QueryRow(context.Background(), "SELECT attempts FROM Task_Runs where task_run_id = 2;").Scan(&attempts)
	require.NoError(t, err)
	require.Equal(t, 3, attempts)
}

func TestPostgresDAGManager_GetDagParameters(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManagerGetParameters_Empty(t, dm)
	testDAGManagerGetParameters_HasValues(t, dm)
}

func TestPostgresDAGManager_DagExists(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManagerDagExists(t, dm)
}

func TestPostgresDAGManager_ShouldRerun(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManagerShouldRerun_MatchingExitCode(t, dm)
	testDAGManagerShouldRerun_MisMatchCode(t, dm)
	testDAGManagerShouldRerun_ValidCodeButNoAttemptsLeft(t, dm)
}

func TestPostgresDAGManager_MarkTaskAsFailed(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManagerMarkTaskAsFailed_Normal(t, dm)

	outcome := ""
	err = pool.QueryRow(context.Background(), "SELECT status FROM Task_Runs where task_run_id = 1;").Scan(&outcome)
	require.NoError(t, err)
	require.Equal(t, "failed", outcome)

	outcome = ""
	failedCount := 0
	err = pool.QueryRow(context.Background(), `
	SELECT failedCount, status 
	FROM DAG_Runs 
	WHERE run_id in (
		SELECT run_id
		FROM Task_Runs
		WHERE task_run_id = 1
	);`).Scan(&failedCount, &outcome)

	require.NoError(t, err)
	require.Equal(t, "failed", outcome)
	require.Equal(t, 1, failedCount)
}

func TestPostgresDAGManager_MarkPodStatus(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManagerMarkPodStatus_Insert(t, dm)

	status := ""
	err = pool.QueryRow(context.Background(), `
	SELECT status 
	FROM Task_Pods 
	WHERE name = $1;`, "pod-one").Scan(&status)

	require.NoError(t, err)
	require.Equal(t, string(v1.PodPending), status)

	testDAGManagerMarkPodStatus_Insert_Multiple(t, dm)

	status = ""
	var duration sql.NullInt64
	err = pool.QueryRow(context.Background(), `
	SELECT status, duration
	FROM Task_Pods 
	WHERE name = $1;`, "pod-two").Scan(&status, &duration)

	require.NoError(t, err)
	require.Equal(t, string(v1.PodSucceeded), status)
	require.Equal(t, int64(60*60), duration.Int64)

}

func TestPostgresDAGManager_DeleteDag(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManagerDeleteDAG_Exists(t, dm)
	testDAGManagerDeleteDAG_Does_Not_Exist(t, dm)
	testDAGManagerDeleteDAG_Noop_on_double_delete(t, dm)
}

func TestPostgresDAGManager_DeleteDag_TaskRefs(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManagerDeleteDAG_UsingTaskRefs_Not_Needed(t, dm)
}

func TestPostgresDAGManager_DeleteDag_TaskRefs_Versioning(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManagerDeleteDAG_UsingTaskRefs_Old_Version_Not_Needed(t, dm)
	testDAGManagerDeleteDAG_UsingTaskRefs_Old_Version_Needed(t, dm)
}

func TestPostgresDAGManager_FindExistingDAGRun(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManagerFindExistingDAGRun_Exists(t, dm)
	testDAGManagerFindExistingDAGRun_Not_Exists(t, dm)
}

func TestPostgresDAGManager_AddTask(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManager_AddTask_Success(t, dm)
	testDAGManager_AddTask_ExistingTask(t, dm)
}

func TestPostgresDAGManager_GetTaskRefsParameters(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManager_GetTaskRefsParameters_Success(t, dm)
	testDAGManager_GetTaskRefsParameters_NonExistentTask(t, dm)
}

func TestPostgresDAGManager_InsertDag_TaskRef(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManagerInsertDag_TaskRef(t, dm)
}

func Test_Postgres_Task_Before_InsertDag(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManager_AddTask_Success(t, dm)

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

	err = dm.InsertDAG(context.Background(), dag, "default")
	require.NoError(t, err)

	dagRun := &v1alpha1.DagRunSpec{
		DagName: "test_dag",
	}

	runID, err := dm.CreateDAGRun(context.Background(), "name", dagRun, map[string]v1alpha1.ParameterSpec{}, nil)
	require.NoError(t, err)

	tasks, err := dm.GetStartingTasks(context.Background(), "test_dag", 1)
	require.NoError(t, err)
	require.NotEmpty(t, tasks)
	require.Len(t, tasks, 2)
	require.ElementsMatch(t, []string{tasks[0].Name, tasks[1].Name}, []string{"task1", "task2"})

	tasRunID, err := dm.MarkTaskAsStarted(context.Background(), runID, tasks[0].Id)
	require.NoError(t, err)

	tasksEmpty, err := dm.MarkSuccessAndGetNextTasks(context.Background(), tasRunID)
	require.NoError(t, err)
	require.Empty(t, tasksEmpty, "taskIdMarked", tasks[0].Id)

	taskRun, err := dm.MarkTaskAsStarted(context.Background(), runID, tasks[1].Id)
	require.NoError(t, err)

	tasks, err = dm.MarkSuccessAndGetNextTasks(context.Background(), taskRun)
	require.NoError(t, err)
	require.NotEmpty(t, tasks)
	require.Equal(t, tasks[0].Name, "task3")
}

func Test_Postgres_Complex_Example(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManager_AddTask_Success(t, dm)
	testDAGManager_Complex_Dag(t, dm)
}

func TestPostgresDAGManager_CreateDAGRun_Sequential(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManager_CreateDagRun_Sequential(t, dm)
}

func TestPostgresDAGManager_CreateDAGRun_Scripts(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManager_CreateDagRun_Scripts(t, dm)
}

func TestPostgresDAGManager_Multi_TaskRef(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

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
	assert.NoError(t, dm.AddTask(context.Background(), task, namespace))

	dag := &v1alpha1.DAG{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test_dag",
		},
		Spec: v1alpha1.DAGSpec{
			Parameters: []v1alpha1.DagParameterSpec{
				{Name: "param1", DefaultValue: "value1"},
			},
			Schedule: "*/5 * * * *",
			Task: []v1alpha1.TaskSpec{
				{
					Name: "task1",
					TaskRef: &v1alpha1.TaskRef{
						Name:    "test_task",
						Version: 1,
					},
				},
				{
					Name: "task2",
					TaskRef: &v1alpha1.TaskRef{
						Name:    "test_task",
						Version: 1,
					},
				},
				{
					Name: "task3",
					TaskRef: &v1alpha1.TaskRef{
						Name:    "test_task",
						Version: 1,
					},
					RunAfter: []string{"task1", "task2"},
				},
			},
		},
	}

	err = dm.InsertDAG(context.Background(), dag, "default")
	require.NoError(t, err)

	dagRun := &v1alpha1.DagRunSpec{
		DagName: "test_dag",
	}

	runID, err := dm.CreateDAGRun(context.Background(), "name", dagRun, map[string]v1alpha1.ParameterSpec{}, nil)
	require.NoError(t, err)

	tasks, err := dm.GetStartingTasks(context.Background(), "test_dag", 1)
	require.NoError(t, err)
	require.NotEmpty(t, tasks)
	require.Len(t, tasks, 2)
	require.ElementsMatch(t, []string{tasks[0].Name, tasks[1].Name}, []string{"task1", "task2"})

	tasRunID, err := dm.MarkTaskAsStarted(context.Background(), runID, tasks[0].Id)
	require.NoError(t, err)

	tasksEmpty, err := dm.MarkSuccessAndGetNextTasks(context.Background(), tasRunID)
	require.NoError(t, err)
	require.Empty(t, tasksEmpty)

	taskRun, err := dm.MarkTaskAsStarted(context.Background(), runID, tasks[1].Id)
	require.NoError(t, err)

	tasks, err = dm.MarkSuccessAndGetNextTasks(context.Background(), taskRun)
	require.NoError(t, err)
	require.NotEmpty(t, tasks)

	// create second dag run
	runID2, err := dm.CreateDAGRun(context.Background(), "name2", dagRun, map[string]v1alpha1.ParameterSpec{}, nil)
	require.NoError(t, err)

	tasksTwo, err := dm.GetStartingTasks(context.Background(), "test_dag", 2)
	require.NoError(t, err)
	require.NotEmpty(t, tasksTwo)
	require.Len(t, tasksTwo, 2)
	require.ElementsMatch(t, []string{tasksTwo[0].Name, tasksTwo[1].Name}, []string{"task1", "task2"})

	tasRunID2, err := dm.MarkTaskAsStarted(context.Background(), runID2, tasksTwo[0].Id)
	require.NoError(t, err)

	tasksEmpty2, err := dm.MarkSuccessAndGetNextTasks(context.Background(), tasRunID2)
	require.NoError(t, err)
	require.Empty(t, tasksEmpty2)

	taskRun2, err := dm.MarkTaskAsStarted(context.Background(), runID2, tasksTwo[1].Id)
	require.NoError(t, err)

	tasks, err = dm.MarkSuccessAndGetNextTasks(context.Background(), taskRun2)
	require.NoError(t, err)
	require.NotEmpty(t, tasks)
}

func TestPostgresDAGManager_Workspace_full(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManager_Workspace_full(t, dm)
}

func TestPostgresDAGManager_Workspace_non_optional_only(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManager_Workspace_non_optional_only(t, dm)
}

func TestPostgresDAGManager_Workspace_disabled(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManager_Workspace_disabled(t, dm)
}

func TestPostgresDAGManager_MarkConnectingTasksAsSuspended_Single(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManager_MarkConnectingTasksAsSuspended_single(t, dm)

	var suspendedCount int
	err = pool.QueryRow(context.Background(), `
	SELECT suspendedCount
	FROM DAG_Runs
	WHERE run_id = 1;`).Scan(&suspendedCount)
	require.NoError(t, err)
	require.Equal(t, 3, suspendedCount)
}

func TestPostgresDAGManager_MarkConnectingTasksAsSuspended_deduplicate_tasks(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManager_MarkConnectingTasksAsSuspended_deduplicate_tasks(t, dm)

	var suspendedCount int
	err = pool.QueryRow(context.Background(), `
	SELECT suspendedCount
	FROM DAG_Runs
	WHERE run_id = 1;`).Scan(&suspendedCount)
	require.NoError(t, err)
	require.Equal(t, 3, suspendedCount)
}

func TestPostgresDAGManager_MarkConnectingTasksAsSuspended_overlapping_dependencies(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManager_MarkConnectingTasksAsSuspended_overlapping_dependencies(t, dm)
}

func TestPostgresDAGManager_DeleteDagRun(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManager_DeleteDagRun_Simple(t, dm)
	// query the database to check if the dag run was deleted
	var count int
	err = pool.QueryRow(context.Background(), `
	SELECT COUNT(*)
	FROM DAG_Runs
	WHERE run_id = 1;`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	// check tasks are deleted
	var taskCount int
	err = pool.QueryRow(context.Background(), `
	SELECT COUNT(*)
	FROM Task_Runs
	WHERE run_id = 1;`).Scan(&taskCount)
	require.NoError(t, err)
	require.Equal(t, 0, taskCount)

	testDAGManager_DeleteDagRun_WithTasks(t, dm)

	// query the database to check if the dag run was deleted
	err = pool.QueryRow(context.Background(), `
	SELECT COUNT(*)
	FROM DAG_Runs
	WHERE run_id = 2;`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)
	// check tasks are deleted
	err = pool.QueryRow(context.Background(), `
	SELECT COUNT(*)
	FROM Task_Runs
	WHERE run_id = 2;`).Scan(&taskCount)
	require.NoError(t, err)
	require.Equal(t, 0, taskCount)

	testDAGManager_DeleteDagRun_WithParameters(t, dm)

	// query the database to check if the dag run was deleted
	err = pool.QueryRow(context.Background(), `
	SELECT COUNT(*)
	FROM DAG_Runs
	WHERE run_id = 3;`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)
	// check tasks are deleted
	err = pool.QueryRow(context.Background(), `
	SELECT COUNT(*)
	FROM Task_Runs
	WHERE run_id = 3;`).Scan(&taskCount)
	require.NoError(t, err)
	require.Equal(t, 0, taskCount)
	// check parameters are deleted
	var paramCount int
	err = pool.QueryRow(context.Background(), `
	SELECT COUNT(*)
	FROM DAG_Run_Parameters
	WHERE run_id = 3;`).Scan(&paramCount)
	require.NoError(t, err)
	require.Equal(t, 0, paramCount)
}

func TestPostgresDAGManager_SuspendDagRun(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManager_SuspendDagRun(t, dm)
}

func TestPostgresDAGManager_SuspendDag(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManagerUpdateSuspended(t, dm)

	// check if the dag is suspended
	var suspended bool
	err = pool.QueryRow(context.Background(), `
	SELECT suspended
	FROM DAGS
	WHERE name = $1;`, "test_dag").Scan(&suspended)
	require.NoError(t, err)
	require.Equal(t, true, suspended)

	// count number of dags
	var count int
	err = pool.QueryRow(context.Background(), `
	SELECT COUNT(*)
	FROM DAGS
	WHERE name = $1;`, "test_dag").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestPostgresDAGManager_UnsuspendDag(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManagerUpdateSuspended_Unsuspended(t, dm)

	// check if the dag is suspended
	var suspended bool
	err = pool.QueryRow(context.Background(), `
	SELECT suspended
	FROM DAGS
	WHERE name = $1;`, "test_dag").Scan(&suspended)
	require.NoError(t, err)
	require.Equal(t, false, suspended)
}

func TestPostgresDAGManager_Insert_Suspended_Dag(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManager_insert_suspended_dag(t, dm)
}

func TestPostgresDAGManager_Suspended_Dag_Cannot_Be_Executed_Via_Scheduler(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManager_Suspended_Dag_Cannot_Be_Executed_Via_Scheduler(t, dm)
}

func TestPostgresDAGManager_Scheduler_works(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	if err != nil {
		t.Fatalf("Could not set up PostgreSQL container: %v", err)
	}
	defer pool.Close()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManager_scheduler_works(t, dm)
}
