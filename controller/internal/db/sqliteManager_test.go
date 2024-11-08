package db_test

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"testing"

	"github.com/GreedyKomodoDragon/Kontroler/operator/api/v1alpha1"
	"github.com/GreedyKomodoDragon/Kontroler/operator/internal/db"
	cron "github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	manager, err := db.NewSqliteManager(context.Background(), dbPath, &parser)
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

	dbConn, err := sql.Open("sqlite3", dbPath)
	require.Nil(t, err, "failed to open SQLite database in test", "error", err)

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

	dm, err := db.NewSqliteManager(context.Background(), dbPath, &parser)
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

	dm, err := db.NewSqliteManager(context.Background(), dbPath, &parser)
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

	dm, err := db.NewSqliteManager(context.Background(), dbPath, &parser)
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

	dm, err := db.NewSqliteManager(context.Background(), dbPath, &parser)
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

	dm, err := db.NewSqliteManager(context.Background(), dbPath, &parser)
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

// func Test_Sqlite_DAGManager_MarkOutcomeAndGetNextTasks_No_Task_Yet(t *testing.T) {
// 	pool, err := utils.SetupContainer(context.Background())
// 	if err != nil {
// 		t.Fatalf("Could not set up QL container: %v", err)
// 	}
// 	defer pool.Close()
// 	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// 	dm, err := db.NewDAGManager(context.Background(), pool, &parser)
// 	require.NoError(t, err)

// 	err = dm.InitaliseDatabase(context.Background())
// 	require.NoError(t, err)

// 	dag := &v1alpha1.DAG{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name: "test_dag",
// 		},
// 		Spec: v1alpha1.DAGSpec{
// 			Schedule: "*/5 * * * *",
// 			Task: []v1alpha1.TaskSpec{
// 				{
// 					Name:    "task1",
// 					Command: []string{"echo", "Hello"},
// 					Args:    []string{"arg1", "arg2"},
// 					Image:   "busybox",
// 				},
// 				{
// 					Name:    "task2",
// 					Command: []string{"echo", "Hello"},
// 					Args:    []string{"arg1", "arg2"},
// 					Image:   "busybox",
// 				},
// 				{
// 					Name:     "task3",
// 					Command:  []string{"echo"},
// 					Args:     []string{"Goodbye, World!"},
// 					Image:    "alpine:latest",
// 					RunAfter: []string{"task1", "task2"},
// 				},
// 			},
// 		},
// 	}

// 	err = dm.InsertDAG(context.Background(), dag, "default")
// 	require.NoError(t, err)

// 	dagRun := &v1alpha1.DagRunSpec{
// 		DagName: "test_dag",
// 	}

// 	runID, err := dm.CreateDAGRun(context.Background(), "name", dagRun, map[string]v1alpha1.ParameterSpec{})
// 	require.NoError(t, err)

// 	tasks, err := dm.GetStartingTasks(context.Background(), "test_dag")
// 	require.NoError(t, err)
// 	require.NotEmpty(t, tasks)
// 	require.Len(t, tasks, 2)
// 	require.ElementsMatch(t, []string{tasks[0].Name, tasks[1].Name}, []string{"task1", "task2"})

// 	tasRunID, err := dm.MarkTaskAsStarted(context.Background(), runID, tasks[0].Id)
// 	require.NoError(t, err)

// 	tasks, err = dm.MarkSuccessAndGetNextTasks(context.Background(), tasRunID)
// 	require.NoError(t, err)
// 	require.Empty(t, tasks)
// }

// func Test_Sqlite_DAGManager_MarkTaskAsStarted(t *testing.T) {
// 	pool, err := utils.SetupContainer(context.Background())
// 	if err != nil {
// 		t.Fatalf("Could not set up QL container: %v", err)
// 	}
// 	defer pool.Close()

// 	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// 	dm, err := db.NewDAGManager(context.Background(), pool, &parser)
// 	require.NoError(t, err)

// 	err = dm.InitaliseDatabase(context.Background())
// 	require.NoError(t, err)

// 	dag := &v1alpha1.DAG{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name: "test_dag",
// 		},
// 		Spec: v1alpha1.DAGSpec{
// 			Schedule: "*/5 * * * *",
// 			Task: []v1alpha1.TaskSpec{
// 				{
// 					Name:    "task1",
// 					Command: []string{"echo", "Hello"},
// 					Args:    []string{"arg1", "arg2"},
// 					Image:   "busybox",
// 				},
// 			},
// 		},
// 	}

// 	err = dm.InsertDAG(context.Background(), dag, "default")
// 	require.NoError(t, err)

// 	dagRun := &v1alpha1.DagRunSpec{
// 		DagName: "test_dag",
// 	}

// 	runID, err := dm.CreateDAGRun(context.Background(), "name", dagRun, map[string]v1alpha1.ParameterSpec{})
// 	require.NoError(t, err)

// 	taskID, err := dm.MarkTaskAsStarted(context.Background(), runID, 1)
// 	assert.NoError(t, err)
// 	assert.NotEqual(t, 0, taskID)
// }
