package db

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
)

type CronJob struct {
	Id        string
	Schedule  string
	ImageName string
}

type DbManager interface {
	InitaliseDatabase(ctx context.Context) error
	UpsertCronJob(ctx context.Context, id types.UID, schedule string, imageName string) error
	DeleteCronJob(ctx context.Context, id types.UID) error
	GetAllCronJobs(ctx context.Context) ([]*CronJob, error)
	Close()
}
