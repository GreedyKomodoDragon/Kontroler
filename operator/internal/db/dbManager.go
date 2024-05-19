package db

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
)

type CronJob struct {
	Id        types.UID
	Schedule  string
	ImageName string
	Command   []string
}
type DbManager interface {
	InitaliseDatabase(ctx context.Context) error
	UpsertCronJob(ctx context.Context, id types.UID, schedule string, imageName string, command []string) error
	DeleteCronJob(ctx context.Context, id types.UID) error
	GetAllCronJobs(ctx context.Context) ([]*CronJob, error)
	GetCronJobsToStart(ctx context.Context) ([]CronJob, error)
	UpdateNextTime(ctx context.Context, uid types.UID, schedule string) error
	Close()
}
