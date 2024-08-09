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
	DagId       int           `json:"dagId"`
	Name        string        `json:"name"`
	Schedule    string        `json:"schedule"`
	Version     int           `json:"version"`
	Active      bool          `json:"active"`
	NextTime    time.Time     `json:"nexttime"`
	Connections map[int][]int `json:"connections"`
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

type DagRunAll struct {
	Id              int              `json:"id"`
	DagId           int              `json:"dagId"`
	Status          string           `json:"status"`
	SuccessfulCount int              `json:"successfulCount"`
	FailedCount     int              `json:"failedCount"`
	Connections     map[int][]int    `json:"connections"`
	TaskInfo        map[int]TaskInfo `json:"taskInfo"`
}

type TaskRunDetails struct {
	Id       int        `json:"id"`
	Status   string     `json:"status"`
	Attempts int        `json:"attempts"`
	Pods     []*TaskPod `json:"pods"`
}

type TaskPod struct {
	PodUID   string `json:"podUID"`
	ExitCode *int   `json:"exitCode"`
	Name     string `json:"name"`
	Status   string `json:"status"`
}

// Parameter represents a task parameter.
type Parameter struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	IsSecret     bool   `json:"isSecret"`
	DefaultValue string `json:"defaultValue"`
}

// TaskDetails represents the details of a task.
type TaskDetails struct {
	ID            int         `json:"id"`
	Name          string      `json:"name"`
	Command       []string    `json:"command"`
	Args          []string    `json:"args"`
	Image         string      `json:"image"`
	Parameters    []Parameter `json:"parameters"`
	BackOffLimit  int         `json:"backOffLimit"`
	IsConditional bool        `json:"isConditional"`
	PodTemplate   string      `json:"podTemplate"`
	RetryCodes    []int       `json:"retryCodes"`
}

type DbManager interface {
	GetAllCronJobs(ctx context.Context) ([]*CronJob, error)
	GetAllRuns(ctx context.Context, limit int, offset int) ([]*Run, error)
	GetRunsPods(ctx context.Context, runId types.UID) ([]*PodWithExitCode, error)
	GetAllDagMetaData(ctx context.Context, limit int, offset int) ([]*DAGMetaData, error)
	GetDagRuns(ctx context.Context, limit int, offset int) ([]*DagRunMeta, error)
	GetDagRun(ctx context.Context, dagRunId int) (*DagRun, error)
	GetDagRunAll(ctx context.Context, dagRunId int) (*DagRunAll, error)
	GetTaskDetails(ctx context.Context, taskId int) (*TaskDetails, error)
	GetTaskRunDetails(ctx context.Context, dagRunId, taskId int) (*TaskRunDetails, error)

	Close()
}
