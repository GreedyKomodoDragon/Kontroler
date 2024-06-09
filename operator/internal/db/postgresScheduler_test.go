package db_test

import (
	"context"
	"testing"

	"github.com/GreedyKomodoDragon/KubeConductor/operator/internal/db"
	cron "github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/types"
)

var (
	ctx context.Context
)

func TestNewPostgresManager_ValidConfig(t *testing.T) {
	pool := setupPostgresContainer(t)

	specParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	manager, err := db.NewPostgresSchedulerManager(ctx, pool, &specParser)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if manager == nil {
		t.Error("Expected non-nil manager, got nil")
	}
}

func TestPostgresManager_GetAllCronJobs(t *testing.T) {
	pool := setupPostgresContainer(t)

	specParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	manager, err := db.NewPostgresSchedulerManager(ctx, pool, &specParser)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Initialize the database
	err = manager.InitaliseDatabase(ctx)
	if err != nil {
		t.Fatalf("InitaliseDatabase returned an error: %v", err)
	}

	cron := db.CronJob{
		Id:           types.UID("test-id"),
		Schedule:     "0 0 * * *",
		ImageName:    "test-image",
		Command:      []string{"echo"},
		Args:         []string{`"Hello, World!"`},
		BackoffLimit: uint64(0),
		ConditionalRetry: db.ConditionalRetry{
			Enabled:    true,
			RetryCodes: []int32{1},
		},
	}
	// Insert a test cron job
	err = manager.UpsertCronJob(ctx, &cron)
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
	if cronJobs[0].Id != cron.Id || cronJobs[0].Schedule != cron.Schedule || cronJobs[0].ImageName != cron.ImageName {
		t.Fatalf("Retrieved cron job does not match expected values")
	}
}

func TestPostgresManager_UpsertCronJob(t *testing.T) {
	pool := setupPostgresContainer(t)

	specParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	manager, err := db.NewPostgresSchedulerManager(ctx, pool, &specParser)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = manager.InitaliseDatabase(ctx)
	if err != nil {
		t.Fatalf("InitaliseDatabase returned an error: %v", err)
	}

	cron := db.CronJob{
		Id:           types.UID("test-id"),
		Schedule:     "0 0 * * *",
		ImageName:    "test-image",
		Command:      []string{"echo"},
		Args:         []string{`"Hello, World!"`},
		BackoffLimit: uint64(0),
		ConditionalRetry: db.ConditionalRetry{
			Enabled:    true,
			RetryCodes: []int32{1},
		},
	}

	err = manager.UpsertCronJob(ctx, &cron)
	if err != nil {
		t.Fatalf("UpsertCronJob returned an error: %v", err)
	}
}

func TestPostgresManager_DeleteCronJob(t *testing.T) {
	pool := setupPostgresContainer(t)

	specParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	manager, err := db.NewPostgresSchedulerManager(ctx, pool, &specParser)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = manager.InitaliseDatabase(ctx)
	if err != nil {
		t.Fatalf("InitaliseDatabase returned an error: %v", err)
	}

	cron := db.CronJob{
		Id:           types.UID("test-id"),
		Schedule:     "0 0 * * *",
		ImageName:    "test-image",
		Command:      []string{"echo"},
		Args:         []string{`"Hello, World!"`},
		BackoffLimit: uint64(0),
		ConditionalRetry: db.ConditionalRetry{
			Enabled:    true,
			RetryCodes: []int32{1},
		},
	}

	err = manager.UpsertCronJob(ctx, &cron)
	if err != nil {
		t.Fatalf("UpsertCronJob returned an error: %v", err)
	}

	cronjobs, err := manager.GetAllCronJobs(ctx)
	if err != nil {
		t.Fatalf("GetAllCronJobs returned an error: %v", err)
	}

	beforeLen := len(cronjobs)

	err = manager.DeleteCronJob(ctx, cron.Id)
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
