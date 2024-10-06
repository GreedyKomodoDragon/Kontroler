package db

import (
	"context"
	"time"

	"github.com/GreedyKomodoDragon/Kontroler/operator/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

type Task struct {
	Id          int
	Name        string
	Image       string
	Command     []string
	Args        []string
	Parameters  []Parameter
	PodTemplate *v1alpha1.PodTemplateSpec
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

type DBDAGManager interface {
	// InitaliseDatabase will ensure all create requires components such as tables in a relational database are within the database
	InitaliseDatabase(ctx context.Context) error
	// Gets all dags to start, then updates to the next time it should be executed
	GetDAGsToStartAndUpdate(ctx context.Context) ([]*DagInfo, error)
	// InsertDAG will add in the new dag into the database, if the dag already exists, it should create a new version
	InsertDAG(ctx context.Context, dag *v1alpha1.DAG, namespace string) error
	// Create the update to show that a new DAG has been started
	CreateDAGRun(ctx context.Context, name string, dag *v1alpha1.DagRunSpec, parameters map[string]v1alpha1.ParameterSpec) (int, error)
	// Get all the tasks in the DAG that do not have any dependencies
	GetStartingTasks(ctx context.Context, dagName string) ([]Task, error)
	// Add an update to show the task has been started
	MarkTaskAsStarted(ctx context.Context, runId, taskId int) (int, error)
	// Mark the outcome of the taskRun
	IncrementAttempts(ctx context.Context, taskRunId int) error
	// Within the same transaction, and get next task(s) in the DAG
	MarkSuccessAndGetNextTasks(ctx context.Context, taskRunId int) ([]Task, error)
	// Update the DAGRun to show the overall outcome
	MarkDAGRunOutcome(ctx context.Context, dagRunId int, outcome string) error
	GetDagParameters(ctx context.Context, dagName string) (map[string]*Parameter, error)
	DagExists(ctx context.Context, dagName string) (bool, error)
	ShouldRerun(ctx context.Context, taskRunid int, exitCode int32) (bool, error)
	MarkTaskAsFailed(ctx context.Context, taskRunId int) error
	MarkPodStatus(ctx context.Context, podUid types.UID, name string, taskRunID int, status v1.PodPhase, tStamp time.Time, exitCode *int32, resourceVersion string, namespace string) error
	// Soft deletes the dag in database
	SoftDeleteDAG(ctx context.Context, name string, namespace string) error
	FindExistingDAGRun(ctx context.Context, name string) (bool, error)
}
