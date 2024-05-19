package db_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/GreedyKomodoDragon/KubeConductor/operator/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
	cron "github.com/robfig/cron/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"k8s.io/apimachinery/pkg/types"
)

var (
	ctx               context.Context
	postgresContainer testcontainers.Container
)

func SetupContainer(t *testing.T) {
	ctx = context.Background()

	dbName := "kubeconductor"
	dbUser := "user"
	dbPassword := "password"

	var err error
	postgresContainer, err = postgres.RunContainer(ctx,
		testcontainers.WithImage("docker.io/postgres:16.3"),
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
		testcontainers.WithLogConsumers(&testcontainers.StdoutLogConsumer{}),
	)
	if err != nil {
		t.Fatalf("failed to start container: %s", err)
	}
}

func teardownContainer(t *testing.T) {
	if postgresContainer != nil {
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}
}

func TestNewPostgresManager_ValidConfig(t *testing.T) {
	SetupContainer(t)
	defer teardownContainer(t)

	// Get PostgreSQL container endpoint
	pgEndpoint, err := postgresContainer.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("failed to get PostgreSQL container endpoint: %s", err)
	}

	dbName := "users"
	dbUser := "user"
	dbPassword := "password"

	config, err := pgxpool.ParseConfig(fmt.Sprintf("postgres://%s:%s@%s/%s", dbUser, dbPassword, pgEndpoint, dbName))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	specParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	manager, err := db.NewPostgresManager(ctx, config, &specParser)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if manager == nil {
		t.Error("Expected non-nil manager, got nil")
	}
}

func TestPostgresManager_GetAllCronJobs(t *testing.T) {
	SetupContainer(t)
	defer teardownContainer(t)

	config := getTestDBConfig(t)

	specParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	manager, err := db.NewPostgresManager(ctx, config, &specParser)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()

	// Initialize the database
	err = manager.InitaliseDatabase(ctx)
	if err != nil {
		t.Fatalf("InitaliseDatabase returned an error: %v", err)
	}

	id := types.UID("test-id")
	schedule := "0 0 * * *"
	imageName := "test-image"
	command := []string{"echo"}
	args := []string{`"Hello, World!"`}

	// Insert a test cron job
	err = manager.UpsertCronJob(ctx, id, schedule, imageName, command, args)
	if err != nil {
		t.Fatalf("UpsertCronJob returned an error: %v", err)
	}

	// Retrieve all cron jobs
	cronJobs, err := manager.GetAllCronJobs(ctx)
	if err != nil {
		t.Fatalf("GetAllCronJobs returned an error: %v", err)
	}

	// Assert that cronJobs contains the inserted cron job
	if len(cronJobs) != 1 {
		t.Fatalf("Expected 1 cron job, got %d", len(cronJobs))
	}
	if cronJobs[0].Id != id || cronJobs[0].Schedule != schedule || cronJobs[0].ImageName != imageName {
		t.Fatalf("Retrieved cron job does not match expected values")
	}
}

func TestPostgresManager_UpsertCronJob(t *testing.T) {
	SetupContainer(t)
	defer teardownContainer(t)

	config := getTestDBConfig(t)

	specParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	manager, err := db.NewPostgresManager(ctx, config, &specParser)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = manager.InitaliseDatabase(ctx)
	if err != nil {
		t.Fatalf("InitaliseDatabase returned an error: %v", err)
	}

	id := types.UID("test-id")
	schedule := "0 0 * * *"
	imageName := "test-image"
	command := []string{"echo"}
	args := []string{`"Hello, World!"`}

	err = manager.UpsertCronJob(ctx, id, schedule, imageName, command, args)
	if err != nil {
		t.Fatalf("UpsertCronJob returned an error: %v", err)
	}
}

func TestPostgresManager_DeleteCronJob(t *testing.T) {
	SetupContainer(t)
	defer teardownContainer(t)

	config := getTestDBConfig(t)

	specParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	manager, err := db.NewPostgresManager(ctx, config, &specParser)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = manager.InitaliseDatabase(ctx)
	if err != nil {
		t.Fatalf("InitaliseDatabase returned an error: %v", err)
	}

	id := types.UID("test-id")
	schedule := "0 0 * * *"
	imageName := "test-image"
	command := []string{"echo"}
	args := []string{`"Hello, World!"`}

	err = manager.UpsertCronJob(ctx, id, schedule, imageName, command, args)
	if err != nil {
		t.Fatalf("UpsertCronJob returned an error: %v", err)
	}

	cronjobs, err := manager.GetAllCronJobs(ctx)
	if err != nil {
		t.Fatalf("GetAllCronJobs returned an error: %v", err)
	}

	beforeLen := len(cronjobs)

	err = manager.DeleteCronJob(ctx, id)
	if err != nil {
		t.Fatalf("DeleteCronJob returned an error: %v", err)
	}

	cronjobs, err = manager.GetAllCronJobs(ctx)
	if err != nil {
		t.Fatalf("GetAllCronJobs returned an error: %v", err)
	}

	afterLen := len(cronjobs)
	if afterLen >= beforeLen {
		t.Errorf("deletion did not work correctly, before size: %v, after size: %v", beforeLen, afterLen)
	}

}

func getTestDBConfig(t *testing.T) *pgxpool.Config {
	dbName := "kubeconductor"
	dbUser := "user"
	dbPassword := "password"

	pgEndpoint, err := postgresContainer.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("failed to get PostgreSQL container endpoint: %s", err)
	}

	config, err := pgxpool.ParseConfig(fmt.Sprintf("postgres://%s:%s@%s/%s", dbUser, dbPassword, pgEndpoint, dbName))
	if err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}

	return config
}
