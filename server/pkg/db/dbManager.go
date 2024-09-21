package db

import (
	"context"
	"time"
)

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
	Schedule    *string       `json:"schedule"`
	Version     int           `json:"version"`
	Active      bool          `json:"active"`
	NextTime    *time.Time    `json:"nexttime"`
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

type DashboardStats struct {
	DAGCount          int                `json:"dag_count"`
	SuccessfulDagRuns int                `json:"successful_dag_runs"`
	FailedDagRuns     int                `json:"failed_dag_runs"`
	TotalDagRuns      int                `json:"total_dag_runs"`
	ActiveDagRuns     int                `json:"active_dag_runs"`
	DAGTypeCounts     map[string]int     `json:"dag_type_counts"`
	TaskOutcomes      map[string]int     `json:"task_outcomes"`
	DailyDagRunCounts []DailyDagRunCount `json:"daily_dag_run_counts"`
}

type DailyDagRunCount struct {
	Day             time.Time `json:"day"`
	SuccessfulCount int       `json:"successful_count"`
	FailedCount     int       `json:"failed_count"`
}

type DbManager interface {
	GetAllDagMetaData(ctx context.Context, limit int, offset int) ([]*DAGMetaData, error)
	GetDagRuns(ctx context.Context, limit int, offset int) ([]*DagRunMeta, error)
	GetDagRun(ctx context.Context, dagRunId int) (*DagRun, error)
	GetDagRunAll(ctx context.Context, dagRunId int) (*DagRunAll, error)
	GetTaskDetails(ctx context.Context, taskId int) (*TaskDetails, error)
	GetTaskRunDetails(ctx context.Context, dagRunId, taskId int) (*TaskRunDetails, error)
	GetDashboardStats(ctx context.Context) (*DashboardStats, error)
	GetDagRunPageCount(ctx context.Context, limit int) (int, error)
	GetDagPageCount(ctx context.Context, limit int) (int, error)
	GetDagNames(ctx context.Context, term string, limit int) ([]*string, error)

	Close()
}
