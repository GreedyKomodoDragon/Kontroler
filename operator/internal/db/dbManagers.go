package db

import (
	"context"

	"github.com/GreedyKomodoDragon/KubeConductor/operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

type CronJob struct {
	Id               types.UID
	Schedule         string
	ImageName        string
	Command          []string
	Args             []string
	BackoffLimit     uint64
	ConditionalRetry ConditionalRetry
	Namespace        string
}

type Task struct {
	Id         int
	Name       string
	Image      string
	Command    []string
	Args       []string
	Parameters []Parameter
	// TODO: Implement retries
	// BackoffLimit     uint64
	// ConditionalRetry ConditionalRetry
}

type Parameter struct {
	Name     string
	IsSecret bool
	Value    string
}

type ConditionalRetry struct {
	Enabled    bool
	RetryCodes []int32
}

type DBSchedulerManager interface {
	InitaliseDatabase(ctx context.Context) error
	UpsertCronJob(ctx context.Context, cron *CronJob) error
	DeleteCronJob(ctx context.Context, id types.UID) error
	GetAllCronJobs(ctx context.Context) ([]*CronJob, error)
	GetCronJobsToStart(ctx context.Context) ([]*CronJob, error)
	UpdateNextTime(ctx context.Context, uid types.UID, schedule string) error
	StartRun(ctx context.Context, jobUid, runID types.UID) error
	IncrementRunCount(ctx context.Context, runID types.UID) error
	ShouldRerun(ctx context.Context, runID types.UID, exitCode int32) (bool, error)
	MarkRunOutcome(ctx context.Context, runID types.UID, status string) error
	AddPodToRun(ctx context.Context, podName string, runID types.UID, exitCode int32) error
}

type DBDAGManager interface {
	// InitaliseDatabase will ensure all create requires components such as tables in a relational database are within the database
	InitaliseDatabase(ctx context.Context) error
	// Gets all dags to start, then updates to the next time it should be executed
	GetDAGsToStartAndUpdate(ctx context.Context) ([]int, []string, error)
	// InsertDAG will add in the new dag into the database, if the dag already exists, it should create a new version
	InsertDAG(ctx context.Context, dag *v1alpha1.DAG, namespace string) error
	// Create the update to show that a new DAG has been started
	CreateDAGRun(ctx context.Context, dag *v1alpha1.DagRunSpec, parameters map[string]v1alpha1.ParameterSpec) (int, error)
	// Get all the tasks in the DAG that do not have any dependencies
	GetStartingTasks(ctx context.Context, dagId int) ([]Task, error)
	// Add an update to show the task has been started
	MarkTaskAsStarted(ctx context.Context, runId, taskId int) (int, error)
	// Mark the outcome of the taskRun
	MarkOutcomeAsFailed(ctx context.Context, taskRunId int) error
	// Within the same transaction, and get next task(s) in the DAG
	MarkSuccessAndGetNextTasks(ctx context.Context, taskRunId int) ([]Task, error)
	// Update the DAGRun to show the overall outcome
	MarkDAGRunOutcome(ctx context.Context, dagRunId int, outcome string) error
	GetDagParameters(ctx context.Context, dagId int) (map[string]*Parameter, error)
	DagExists(ctx context.Context, dagId int) (bool, error)
	ShouldRerun(ctx context.Context, taskRunid int, exitCode int32) (bool, error)
	MarkTaskAsFailed(ctx context.Context, taskRunId int) error
}
