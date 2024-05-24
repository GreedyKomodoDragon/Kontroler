package db

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
)

type CronJob struct {
	Id               types.UID        `json:"id"`
	Schedule         string           `json:"schedule"`
	ImageName        string           `json:"imageName"`
	Command          []string         `json:"command"`
	Args             []string         `json:"args"`
	BackoffLimit     uint64           `json:"backoffLimit"`
	ConditionalRetry ConditionalRetry `json:"conditionalRetry"`
}

type ConditionalRetry struct {
	Enabled    bool    `json:"enabled"`
	RetryCodes []int32 `json:"retryCodes"`
}

type Run struct {
	RunId   types.UID `json:"runId"`
	PodName string    `json:"podName"`
}

type DbManager interface {
	GetAllCronJobs(ctx context.Context) ([]*CronJob, error)
	GetAllRuns(ctx context.Context) ([]*Run, error)

	Close()
}
