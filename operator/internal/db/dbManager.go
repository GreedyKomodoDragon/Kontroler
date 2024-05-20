package db

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
)

type CronJob struct {
	Id           types.UID
	Schedule     string
	ImageName    string
	Command      []string
	Args         []string
	BackoffLimit uint64
}

type DbManager interface {
	InitaliseDatabase(ctx context.Context) error
	UpsertCronJob(ctx context.Context, id types.UID, schedule string, imageName string, command []string, args []string, backoffLimit uint64) error
	DeleteCronJob(ctx context.Context, id types.UID) error
	GetAllCronJobs(ctx context.Context) ([]*CronJob, error)
	GetCronJobsToStart(ctx context.Context) ([]CronJob, error)
	UpdateNextTime(ctx context.Context, uid types.UID, schedule string) error
	StartRun(ctx context.Context, jobUid, runID types.UID) error
	IncrementRunCount(ctx context.Context, runID types.UID) error
	ShouldRerun(ctx context.Context, runID types.UID) (bool, error)

	Close()
}
