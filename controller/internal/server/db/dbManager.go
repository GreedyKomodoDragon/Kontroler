package db

import (
	"context"
	v1 "kontroler-controller/api/v1alpha1"
	"time"
)

// Reuse Backoff definition from the API package to avoid duplication
type Backoff = v1.Backoff

type DBDAGMetaData struct {
	DagId       int           `json:"dagId"`
	Name        string        `json:"name"`
	Namespace   string        `json:"namespace"`
	Schedule    *string       `json:"schedule"`
	Version     int           `json:"version"`
	Active      bool          `json:"active"`
	NextTime    *time.Time    `json:"nexttime"`
	IsSuspended bool          `json:"isSuspended"`
	Connections map[int][]int `json:"connections"`
}

type DBTaskInfo struct {
	Status string `json:"status"`
	Name   string `json:"name"`
}

type DBDagRunMeta struct {
	Id              int    `json:"id"`
	DagId           string `json:"dagId"`
	Status          string `json:"status"`
	SuccessfulCount int    `json:"successfulCount"`
	FailedCount     int    `json:"failedCount"`
	Namespace       string `json:"namespace"`
	Name            string `json:"name"`
}

type DBDagRun struct {
	Connections map[int][]int      `json:"connections"`
	TaskInfo    map[int]DBTaskInfo `json:"taskInfo"`
}

type DBDagRunAll struct {
	Id              int                `json:"id"`
	DagId           int                `json:"dagId"`
	Status          string             `json:"status"`
	SuccessfulCount int                `json:"successfulCount"`
	FailedCount     int                `json:"failedCount"`
	Connections     map[int][]int      `json:"connections"`
	TaskInfo        map[int]DBTaskInfo `json:"taskInfo"`
}

type DBTaskRunDetails struct {
	Id       int          `json:"id"`
	Status   string       `json:"status"`
	Attempts int          `json:"attempts"`
	Pods     []*DBTaskPod `json:"pods"`
}

type DBTaskPod struct {
	PodUID   string `json:"podUID"`
	ExitCode *int   `json:"exitCode"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Duration *int64 `json:"duration"`
}

// DBParameter represents a task parameter returned by the server DB layer.
type DBParameter struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	IsSecret     bool   `json:"isSecret"`
	DefaultValue string `json:"defaultValue"`
}

// DBTaskDetails represents the details of a task returned by the DB layer.
type DBTaskDetails struct {
	ID            int           `json:"id"`
	Name          string        `json:"name"`
	Command       []string      `json:"command,omitempty"`
	Args          []string      `json:"args,omitempty"`
	Image         string        `json:"image"`
	Parameters    []DBParameter `json:"parameters"`
	BackOffLimit  int           `json:"backOffLimit"`
	IsConditional bool          `json:"isConditional"`
	PodTemplate   string        `json:"podTemplate"`
	RetryCodes    []int         `json:"retryCodes"`
	Script        string        `json:"script,omitempty"`
}

type DBDagTaskDetails struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`
	Command       []string `json:"command,omitempty"`
	Args          []string `json:"args,omitempty"`
	Image         string   `json:"image"`
	Parameters    []string `json:"parameters"`
	BackOffLimit  int      `json:"backOffLimit"`
	IsConditional bool     `json:"isConditional"`
	PodTemplate   string   `json:"podTemplate"`
	RetryCodes    []int    `json:"retryCodes"`
	Script        string   `json:"script,omitempty"`
}

type DBDashboardStats struct {
	DAGCount          int                  `json:"dag_count"`
	SuccessfulDagRuns int                  `json:"successful_dag_runs"`
	FailedDagRuns     int                  `json:"failed_dag_runs"`
	TotalDagRuns      int                  `json:"total_dag_runs"`
	ActiveDagRuns     int                  `json:"active_dag_runs"`
	DAGTypeCounts     map[string]int       `json:"dag_type_counts"`
	TaskOutcomes      map[string]int       `json:"task_outcomes"`
	DailyDagRunCounts []DBDailyDagRunCount `json:"daily_dag_run_counts"`
}

type DBDailyDagRunCount struct {
	Day             time.Time `json:"day"`
	SuccessfulCount int       `json:"successful_count"`
	FailedCount     int       `json:"failed_count"`
}

type DbManager interface {
	GetAllDagMetaData(ctx context.Context, limit int, offset int) ([]*DBDAGMetaData, error)
	GetDagRuns(ctx context.Context, limit int, offset int) ([]*DBDagRunMeta, error)
	GetDagRun(ctx context.Context, dagRunId int) (*DBDagRun, error)
	GetDagRunAll(ctx context.Context, dagRunId int) (*DBDagRunAll, error)
	GetTaskDetails(ctx context.Context, taskID int) (*DBTaskDetails, error)
	GetTaskRunDetails(ctx context.Context, dagRunId, taskID int) (*DBTaskRunDetails, error)
	GetDashboardStats(ctx context.Context) (*DBDashboardStats, error)
	GetDagRunPageCount(ctx context.Context, limit int) (int, error)
	GetDagPageCount(ctx context.Context, limit int) (int, error)
	GetDagNames(ctx context.Context, term string, limit int) ([]*string, error)
	GetDagParameters(ctx context.Context, dagName string) ([]*DBParameter, error)
	GetIsSecrets(ctx context.Context, dagName string, parameterNames []string) (map[string]bool, error)
	GetDagTasks(ctx context.Context, limit int, offset int) ([]*DBDagTaskDetails, error)
	GetDagTaskPageCount(ctx context.Context, limit int) (int, error)
	PodExists(ctx context.Context, podUID string) (bool, error)
	GetPodNameAndNamespace(ctx context.Context, podUID string) (string, string, error)

	Close()
}
