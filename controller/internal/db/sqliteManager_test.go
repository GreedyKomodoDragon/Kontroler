package db_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/GreedyKomodoDragon/Kontroler/operator/api/v1alpha1"
	"github.com/GreedyKomodoDragon/Kontroler/operator/internal/db"
	cron "github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func Test_Sqlite_UpsertDAG(t *testing.T) {
	dbPath := fmt.Sprintf("/tmp/%s.db", RandStringBytes(10))

	parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	manager, dbConn, err := db.NewSqliteManager(context.Background(), &parser, &db.SQLiteConfig{
		DBPath: dbPath,
	})
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
	require.NoError(t, err, "Failed to insert new DAG")

	// Verify the DAG was inserted
	var dagID int
	err = dbConn.QueryRow("SELECT dag_id FROM DAGs WHERE name = ?", dag.Name).Scan(&dagID)
	require.NoError(t, err, "Failed to query inserted DAG")

	require.NotZero(t, dagID, "DAG ID should not be zero")

	// Test updating the existing DAG
	dag.Spec.Schedule = "*/10 * * * *"
	err = manager.InsertDAG(context.Background(), dag, "default")
	require.NoError(t, err, "Failed to update existing DAG")

	// Verify the DAG was updated
	var updatedSchedule string
	err = dbConn.QueryRow("SELECT schedule FROM DAGs WHERE dag_id = ?", dagID).Scan(&updatedSchedule)
	require.NoError(t, err, "Failed to query updated DAG")
	require.Equal(t, "*/5 * * * *", updatedSchedule, "DAG schedule was updated")

	// Test DAG with additional task
	dag.Spec.Task = append(dag.Spec.Task, v1alpha1.TaskSpec{
		Name:     "task3",
		Command:  []string{"echo"},
		Args:     []string{"Another Task"},
		Image:    "alpine:latest",
		RunAfter: []string{"task2"},
	})
	err = manager.InsertDAG(context.Background(), dag, "default")
	require.NoError(t, err, "Failed to update DAG with additional task")

	// Verify the new task was inserted
	var taskName string
	err = dbConn.QueryRow("SELECT name FROM Tasks WHERE name = 'task3'").Scan(&taskName)
	require.NoError(t, err, "Failed to query new task")
	require.NotZero(t, taskName, "Task name should not be zero")

	// confirm there are now three versions
	var version int
	row := dbConn.QueryRow("SELECT version FROM DAGs ORDER BY version DESC")
	require.NoError(t, row.Scan(&version), "Failed to get third version")
	require.Equal(t, 2, version)
}

func Test_Sqlite_DAGManager_InsertDAG(t *testing.T) {
	dbPath := fmt.Sprintf("/tmp/%s.db", RandStringBytes(10))

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, _, err := db.NewSqliteManager(context.Background(), &parser, &db.SQLiteConfig{
		DBPath: dbPath,
	})
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

func Test_Sqlite_DAGManager_CreateDAGRun(t *testing.T) {
	dbPath := fmt.Sprintf("/tmp/%s.db", RandStringBytes(10))

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, _, err := db.NewSqliteManager(context.Background(), &parser, &db.SQLiteConfig{
		DBPath: dbPath,
	})
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

	runID, err := dm.CreateDAGRun(context.Background(), "name", dagRun, map[string]v1alpha1.ParameterSpec{})
	assert.NoError(t, err)
	assert.NotEqual(t, 0, runID)
}

func Test_Sqlite_DAGManager_GetStartingTasks(t *testing.T) {
	dbPath := fmt.Sprintf("/tmp/%s.db", RandStringBytes(10))

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, _, err := db.NewSqliteManager(context.Background(), &parser, &db.SQLiteConfig{
		DBPath: dbPath,
	})
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

	tasks, err := dm.GetStartingTasks(context.Background(), "test_dag")
	require.NoError(t, err)
	require.NotEmpty(t, tasks)
	require.Len(t, tasks, 1)
	require.Equal(t, tasks[0].Name, "task1")
}

func Test_Sqlite_DAGManager_MarkDAGRunOutcome(t *testing.T) {
	dbPath := fmt.Sprintf("/tmp/%s.db", RandStringBytes(10))
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, _, err := db.NewSqliteManager(context.Background(), &parser, &db.SQLiteConfig{
		DBPath: dbPath,
	})
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

	runID, err := dm.CreateDAGRun(context.Background(), "name", dagRun, map[string]v1alpha1.ParameterSpec{})
	require.NoError(t, err)

	err = dm.MarkDAGRunOutcome(context.Background(), runID, "success")
	assert.NoError(t, err)
}

func Test_Sqlite_DAGManager_MarkOutcomeAndGetNextTasks(t *testing.T) {
	dbPath := fmt.Sprintf("/tmp/%s.db", RandStringBytes(10))
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, _, err := db.NewSqliteManager(context.Background(), &parser, &db.SQLiteConfig{
		DBPath: dbPath,
	})
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

	runID, err := dm.CreateDAGRun(context.Background(), "name", dagRun, map[string]v1alpha1.ParameterSpec{})
	require.NoError(t, err)

	tasks, err := dm.GetStartingTasks(context.Background(), "test_dag")
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

func Test_Sqlite_DAGManager_MarkOutcomeAndGetNextTasks_No_Task_Yet(t *testing.T) {
	dbPath := fmt.Sprintf("/tmp/%s.db", RandStringBytes(10))
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, _, err := db.NewSqliteManager(context.Background(), &parser, &db.SQLiteConfig{
		DBPath: dbPath,
	})
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

	runID, err := dm.CreateDAGRun(context.Background(), "name", dagRun, map[string]v1alpha1.ParameterSpec{})
	require.NoError(t, err)

	tasks, err := dm.GetStartingTasks(context.Background(), "test_dag")
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

func Test_Sqlite_DAGManager_MarkTaskAsStarted(t *testing.T) {
	dbPath := fmt.Sprintf("/tmp/%s.db", RandStringBytes(10))
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, _, err := db.NewSqliteManager(context.Background(), &parser, &db.SQLiteConfig{
		DBPath: dbPath,
	})
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

	runID, err := dm.CreateDAGRun(context.Background(), "name", dagRun, map[string]v1alpha1.ParameterSpec{})
	require.NoError(t, err)

	taskID, err := dm.MarkTaskAsStarted(context.Background(), runID, 1)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, taskID)
}

func Test_SQLite_DAGManager_GetID(t *testing.T) {
	dbPath := fmt.Sprintf("/tmp/%s.db", RandStringBytes(10))
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, dbConn, err := db.NewSqliteManager(context.Background(), &parser, &db.SQLiteConfig{
		DBPath: dbPath,
	})
	require.NoError(t, err)

	defer dbConn.Close()

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManagerGetID_ReturnsExistingID(t, dm)

	// Clean up the table to ensure no rows are present
	_, err = dbConn.Exec("DELETE FROM IdTable")
	require.NoError(t, err)

	testDAGManagerGetID_InsertsAndReturnsNewID(t, dm)
}

func Test_SQLite_DAGManager_IncrementAttempts(t *testing.T) {
	dbPath := fmt.Sprintf("/tmp/%s.db", RandStringBytes(10))
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, dbConn, err := db.NewSqliteManager(context.Background(), &parser, &db.SQLiteConfig{
		DBPath: dbPath,
	})
	require.NoError(t, err)

	defer dbConn.Close()

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManagerIncrementAttempts_IncrementAttempts(t, dm)

	// Clean up the table to ensure no rows are present
	attempts := 0

	err = dbConn.QueryRow("SELECT attempts FROM Task_Runs where task_run_id = 1;").Scan(&attempts)
	require.NoError(t, err)
	require.Equal(t, 2, attempts)

	testDAGManagerIncrementAttempts_MultipleIncrements(t, dm)

	// Clean up the table to ensure no rows are present
	attempts = 0
	err = dbConn.QueryRow("SELECT attempts FROM Task_Runs where task_run_id = 2;").Scan(&attempts)
	require.NoError(t, err)
	require.Equal(t, 3, attempts)
}

func Test_SQLite_DAGManager_GetDagParameters(t *testing.T) {
	dbPath := fmt.Sprintf("/tmp/%s.db", RandStringBytes(10))
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, _, err := db.NewSqliteManager(context.Background(), &parser, &db.SQLiteConfig{
		DBPath: dbPath,
	})
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManagerGetParameters_Empty(t, dm)
	testDAGManagerGetParameters_HasValues(t, dm)
}

func Test_SQLite_DAGManager_ShouldRerun(t *testing.T) {
	dbPath := fmt.Sprintf("/tmp/%s.db", RandStringBytes(10))
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, _, err := db.NewSqliteManager(context.Background(), &parser, &db.SQLiteConfig{
		DBPath: dbPath,
	})
	require.NoError(t, err)

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManagerShouldRerun_MatchingExitCode(t, dm)
	testDAGManagerShouldRerun_MisMatchCode(t, dm)
	testDAGManagerShouldRerun_ValidCodeButNoAttemptsLeft(t, dm)
}

func Test_SQLite_DAGManager_MarkTaskAsFailed(t *testing.T) {
	dbPath := fmt.Sprintf("/tmp/%s.db", RandStringBytes(10))
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, dbConn, err := db.NewSqliteManager(context.Background(), &parser, &db.SQLiteConfig{
		DBPath: dbPath,
	})
	require.NoError(t, err)

	defer dbConn.Close()

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManagerMarkTaskAsFailed_Normal(t, dm)

	outcome := ""
	err = dbConn.QueryRow("SELECT status FROM Task_Runs where task_run_id = 1;").Scan(&outcome)
	require.NoError(t, err)
	require.Equal(t, "failed", outcome)

	outcome = ""
	failedCount := 0
	err = dbConn.QueryRow(`
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

func Test_SQLite_DAGManager_MarkPodStatus(t *testing.T) {
	dbPath := fmt.Sprintf("/tmp/%s.db", RandStringBytes(10))
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, dbConn, err := db.NewSqliteManager(context.Background(), &parser, &db.SQLiteConfig{
		DBPath: dbPath,
	})
	require.NoError(t, err)

	defer dbConn.Close()

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManagerMarkPodStatus_Insert(t, dm)

	status := ""
	err = dbConn.QueryRow(`
	SELECT status 
	FROM Task_Pods 
	WHERE name = ?;`, "pod-one").Scan(&status)

	require.NoError(t, err)
	require.Equal(t, string(v1.PodPending), status)

	testDAGManagerMarkPodStatus_Insert_Multiple(t, dm)

	status = ""
	err = dbConn.QueryRow(`
	SELECT status 
	FROM Task_Pods 
	WHERE name = ?;`, "pod-two").Scan(&status)

	require.NoError(t, err)
	require.Equal(t, string(v1.PodSucceeded), status)
}

func Test_SQLite_DAGManager_SoftDeleteDag(t *testing.T) {
	dbPath := fmt.Sprintf("/tmp/%s.db", RandStringBytes(10))
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, dbConn, err := db.NewSqliteManager(context.Background(), &parser, &db.SQLiteConfig{
		DBPath: dbPath,
	})
	require.NoError(t, err)

	defer dbConn.Close()

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManagerSoftDeleteDAG_Exists(t, dm)
	testDAGManagerSoftDeleteDAG_Does_Not_Exist(t, dm)
	testDAGManagerSoftDeleteDAG_Noop_on_double_delete(t, dm)
}

func Test_SQLite_DAGManager_FindExistingDAGRun(t *testing.T) {
	dbPath := fmt.Sprintf("/tmp/%s.db", RandStringBytes(10))
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	dm, dbConn, err := db.NewSqliteManager(context.Background(), &parser, &db.SQLiteConfig{
		DBPath: dbPath,
	})
	require.NoError(t, err)

	defer dbConn.Close()

	err = dm.InitaliseDatabase(context.Background())
	require.NoError(t, err)

	testDAGManagerFindExistingDAGRun_Exists(t, dm)
	testDAGManagerFindExistingDAGRun_Not_Exists(t, dm)
}
