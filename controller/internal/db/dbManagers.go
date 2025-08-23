package db

import (
	"context"
	"time"

	"kontroler-controller/api/v1alpha1"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

type Task struct {
	Id                  int
	Name                string
	Image               string
	Command             []string
	Args                []string
	Parameters          []Parameter
	PodTemplate         *v1alpha1.PodTemplateSpec
	Script              string
	ScriptInjectorImage string
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

// Add new struct for pod info
type RunningPodInfo struct {
	Name      string
	Namespace string
}

type DBDAGManager interface {
	// InitaliseDatabase will ensure all create requires components such as tables in a relational database are within the database
	InitaliseDatabase(ctx context.Context) error
	GetID(ctx context.Context) (string, error)
	// Gets all dags to start, then updates to the next time it should be executed
	GetDAGsToStartAndUpdate(ctx context.Context, tm time.Time) ([]*DagInfo, error)
	// InsertDAG will add in the new dag into the database, if the dag already exists, it should create a new version
	InsertDAG(ctx context.Context, dag *v1alpha1.DAG, namespace string) error
	// Create the update to show that a new DAG has been started
	CreateDAGRun(ctx context.Context, name string, dag *v1alpha1.DagRunSpec, parameters map[string]v1alpha1.ParameterSpec, pvcName *string) (int, error)
	// Get all the tasks in the DAG that do not have any dependencies
	GetStartingTasks(ctx context.Context, dagName string, dagrun int) ([]Task, error)
	// Add an update to show the task has been started
	MarkTaskAsStarted(ctx context.Context, runId, taskId int) (int, error)
	// Mark the outcome of the taskRun
	IncrementAttempts(ctx context.Context, taskRunId int) error
	// Within the same transaction, and get next task(s) in the DAG
	MarkSuccessAndGetNextTasks(ctx context.Context, taskRunId int) ([]Task, error)
	// Update the DAGRun to show the overall outcome
	MarkDAGRunOutcome(ctx context.Context, dagRunId int, outcome string) error
	GetDagParameters(ctx context.Context, dagName string) (map[string]*Parameter, error)
	DagExists(ctx context.Context, dagName string) (bool, int, error)
	ShouldRerun(ctx context.Context, taskRunid int, exitCode int32) (bool, error)
	MarkTaskAsFailed(ctx context.Context, taskRunId int) error
	MarkPodStatus(ctx context.Context, podUid types.UID, name string, taskRunID int, status v1.PodPhase, tStamp time.Time, exitCode *int32, namespace string) error
	// Deletes the dag in database, and returns all the dagTasks no longer used
	DeleteDAG(ctx context.Context, name string, namespace string) ([]string, error)
	FindExistingDAGRun(ctx context.Context, name string) (bool, error)
	GetTaskScriptAndInjectorImage(ctx context.Context, taskId int) (*string, *string, error)
	AddTask(ctx context.Context, task *v1alpha1.DagTask, namespace string) error
	DeleteTask(ctx context.Context, taskName string, namespace string) error
	GetTaskRefsParameters(ctx context.Context, taskRefs []v1alpha1.TaskRef) (map[v1alpha1.TaskRef][]string, error)
	GetWebhookDetails(ctx context.Context, dagRunID int) (*v1alpha1.Webhook, error)
	GetWorkspacePVCTemplate(ctx context.Context, dagId int) (*v1alpha1.PVC, error)
	CheckIfAllTasksDone(ctx context.Context, dagRunID int) (bool, error)
	MarkConnectingTasksAsSuspended(ctx context.Context, dagRunID, taskRunId int) ([]string, error)
	AddPodDuration(ctx context.Context, taskRunId int, durationSec int64) error
	SuspendDagRun(ctx context.Context, dagRunId int) ([]RunningPodInfo, error)
	DeleteDagRun(ctx context.Context, dagRunId int) error
	DagrunExists(ctx context.Context, dagrunId int) (bool, error)
	// GetTaskRunInfo gets the DAG name, task name, and namespace for a task run ID - used for metrics
	GetTaskRunInfo(ctx context.Context, taskRunId int) (dagName, taskName, namespace string, err error)
}
