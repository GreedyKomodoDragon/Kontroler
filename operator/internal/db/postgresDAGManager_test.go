package db_test

import (
	"context"
	"testing"

	"github.com/GreedyKomodoDragon/KubeConductor/operator/api/v1alpha1"
	"github.com/GreedyKomodoDragon/KubeConductor/operator/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
	cron "github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func setupPostgresContainer(t *testing.T) *pgxpool.Pool {
	ctx := context.Background()

	// Request a PostgreSQL container
	req := testcontainers.ContainerRequest{
		Image:        "postgres:13",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_PASSWORD": "password",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp"),
	}
	postgresC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatal(err)
	}

	host, err := postgresC.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}

	port, err := postgresC.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatal(err)
	}

	databaseURL := "postgres://postgres:password@" + host + ":" + port.Port() + "/testdb"
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}

	return pool
}

func TestUpsertDAG(t *testing.T) {
	pool := setupPostgresContainer(t)
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
	err = manager.UpsertDAG(context.Background(), dag)
	assert.NoError(t, err, "Failed to insert new DAG")

	// Verify the DAG was inserted
	var dagID int
	err = pool.QueryRow(context.Background(), "SELECT dag_id FROM DAGs WHERE name = $1", dag.Name).Scan(&dagID)
	assert.NoError(t, err, "Failed to query inserted DAG")
	assert.NotZero(t, dagID, "DAG ID should not be zero")

	// Test updating the existing DAG
	dag.Spec.Schedule = "*/10 * * * *"
	err = manager.UpsertDAG(context.Background(), dag)
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
	err = manager.UpsertDAG(context.Background(), dag)
	assert.NoError(t, err, "Failed to update DAG with additional task")

	// Verify the new task was inserted
	var taskName string
	err = pool.QueryRow(context.Background(), "SELECT name FROM Tasks WHERE name = 'task3'").Scan(&taskName)
	assert.NoError(t, err, "Failed to query new task")
	assert.NotZero(t, taskName, "Task name should not be zero")

	// confirm there are now three versions
	var version int
	row := pool.QueryRow(context.Background(), "SELECT version FROM DAGs ORDER BY version DESC")
	assert.NoError(t, row.Scan(&version), "Failed to get third version")
	assert.Equal(t, 2, version)
}
