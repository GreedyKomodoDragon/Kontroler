package db

import (
	"context"
	"time"

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
	RunId            types.UID `json:"runId"`
	JobUid           types.UID `json:"jobUid"`
	NumberOfAttempts int64     `json:"numberOfAttempts"`
	Status           string    `json:"status"`
}

type PodWithExitCode struct {
	Name     string `json:"name"`
	ExitCode int32  `json:"exitCode"`
}

type TaskSpec struct {
	Name     string   `json:"name"`
	Command  []string `json:"command"`
	Args     []string `json:"args"`
	Image    string   `json:"image"`
	RunAfter []string `json:"runAfter,omitempty"`
	Backoff  Backoff  `json:"backoff"`
}

// Backoff defines the backoff strategy for a task
type Backoff struct {
	Limit int `json:"limit"`
}

type DAG struct {
	Schedule string     `json:"schedule"`
	Tasks    []TaskSpec `json:"tasks"`
}

type DAGMetaData struct {
	DagId    string    `json:"dagId"`
	Name     string    `json:"name"`
	Schedule string    `json:"schedule"`
	Version  int       `json:"version"`
	Active   bool      `json:"active"`
	NextTime time.Time `json:"nexttime"`
}

type TaskInfo struct {
	Status string `json:"status"`
}

type DagRunMeta struct {
	Id              int    `json:"id"`
	DagId           string `json:"dagId"`
	Status          string `json:"status"`
	SuccessfulCount int    `json:"successfulCount"`
	FailedCount     int    `json:"failedCount"`
}

type DagRun struct {
	Connections map[int][]int    `json:"connections"`
	TaskInfo    map[int]TaskInfo `json:"taskInfo"`
}

type DbManager interface {
	GetAllCronJobs(ctx context.Context) ([]*CronJob, error)
	GetAllRuns(ctx context.Context, limit int, offset int) ([]*Run, error)
	GetRunsPods(ctx context.Context, runId types.UID) ([]*PodWithExitCode, error)
	GetAllDagMetaData(ctx context.Context, limit int, offset int) ([]*DAGMetaData, error)
	GetDagRuns(ctx context.Context, limit int, offset int) ([]*DagRunMeta, error)
	GetDagRun(ctx context.Context, dagRunId int) (*DagRun, error)

	Close()
}
