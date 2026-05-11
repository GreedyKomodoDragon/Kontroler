package workers

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/stretchr/testify/require"
	"kontroler-controller/api/v1alpha1"
	"kontroler-controller/internal/queue"
	"kontroler-controller/internal/db"
	"kontroler-controller/internal/webhook"
)

// fakeDB is a minimal fake implementing db.DBDAGManager methods used by the worker tests.
type fakeDB struct {
	markPodStatusCalled int32
	lastPodUID          types.UID
	lastName            string
	lastTaskRunID       int
	lastPhase           v1.PodPhase
}

func (f *fakeDB) InitaliseDatabase(ctx context.Context) error { return nil }
func (f *fakeDB) GetID(ctx context.Context) (string, error)    { return "fake", nil }
func (f *fakeDB) GetDAGsToStartAndUpdate(ctx context.Context, tm time.Time) ([]*db.DagInfo, error) {
	return nil, nil
}
func (f *fakeDB) InsertDAG(ctx context.Context, dag *v1alpha1.DAG, namespace string) error { return nil }
func (f *fakeDB) CreateDAGRun(ctx context.Context, name string, dag *v1alpha1.DagRunSpec, parameters map[string]v1alpha1.ParameterSpec, pvcName *string) (int, error) {
	return 0, nil
}
func (f *fakeDB) GetStartingTasks(ctx context.Context, dagName string, dagrun int) ([]db.Task, error) { return nil, nil }
func (f *fakeDB) MarkTaskAsStarted(ctx context.Context, runId, taskId int) (int, error)     { return 0, nil }
func (f *fakeDB) IncrementAttempts(ctx context.Context, taskRunId int) error               { return nil }
func (f *fakeDB) MarkSuccessAndGetNextTasks(ctx context.Context, taskRunId int) ([]db.Task, error) {
	return nil, nil
}
func (f *fakeDB) MarkDAGRunOutcome(ctx context.Context, dagRunId int, outcome string) error { return nil }
func (f *fakeDB) GetDagParameters(ctx context.Context, dagName string) (map[string]*db.Parameter, error) {
	return nil, nil
}
func (f *fakeDB) DagExists(ctx context.Context, dagName string) (bool, int, error) { return false, 0, nil }
func (f *fakeDB) ShouldRerun(ctx context.Context, taskRunid int, exitCode int32) (bool, error) {
	return false, nil
}
func (f *fakeDB) MarkTaskAsFailed(ctx context.Context, taskRunId int) error { return nil }
func (f *fakeDB) MarkPodStatus(ctx context.Context, podUid types.UID, name string, taskRunID int, status v1.PodPhase, tStamp time.Time, exitCode *int32, namespace string) error {
	atomic.StoreInt32(&f.markPodStatusCalled, 1)
	f.lastPodUID = podUid
	f.lastName = name
	f.lastTaskRunID = taskRunID
	f.lastPhase = status
	return nil
}
func (f *fakeDB) DeleteDAG(ctx context.Context, name string, namespace string) ([]string, error) { return nil, nil }
func (f *fakeDB) FindExistingDAGRun(ctx context.Context, name string) (bool, error)                      { return false, nil }
func (f *fakeDB) GetTaskScriptAndInjectorImage(ctx context.Context, taskId int) (*string, *string, error) { return nil, nil, nil }
func (f *fakeDB) AddTask(ctx context.Context, task *v1alpha1.DagTask, namespace string) error             { return nil }
func (f *fakeDB) DeleteTask(ctx context.Context, taskName string, namespace string) error                 { return nil }
func (f *fakeDB) GetTaskRefsParameters(ctx context.Context, taskRefs []v1alpha1.TaskRef) (map[v1alpha1.TaskRef][]string, error) {
	return nil, nil
}
func (f *fakeDB) GetWebhookDetails(ctx context.Context, dagRunID int) (*v1alpha1.Webhook, error) { return &v1alpha1.Webhook{}, nil }
func (f *fakeDB) GetWorkspacePVCTemplate(ctx context.Context, dagId int) (*v1alpha1.PVC, error)    { return nil, nil }
func (f *fakeDB) CheckIfAllTasksDone(ctx context.Context, dagRunID int) (bool, error)               { return true, nil }
func (f *fakeDB) MarkConnectingTasksAsSuspended(ctx context.Context, dagRunID, taskRunId int) ([]string, error) {
	return nil, nil
}
func (f *fakeDB) AddPodDuration(ctx context.Context, taskRunId int, durationSec int64) error { return nil }
func (f *fakeDB) SuspendDagRun(ctx context.Context, dagRunId int) ([]db.RunningPodInfo, error) { return nil, nil }
func (f *fakeDB) DeleteDagRun(ctx context.Context, dagRunId int) error                       { return nil }
func (f *fakeDB) DagrunExists(ctx context.Context, dagrunId int) (bool, error)               { return false, nil }
func (f *fakeDB) GetTaskRunInfo(ctx context.Context, taskRunId int) (dagName, taskName, namespace string, err error) {
	return "d", "t", "ns", nil
}

func TestWorkerProcessesRunningPodAndWritesDB(t *testing.T) {
	q := queue.NewMemoryQueue(context.Background())
	fdb := &fakeDB{}
	webhookChan := make(chan webhook.WebhookPayload, 1)
	// create worker with minimal dependencies; clientset and taskAllocator not needed for this test
	w := NewWorker(q, nil, webhookChan, fdb, nil, nil, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// start worker
	go func() {
		_ = w.Run(ctx)
	}()

	// prepare pod with annotations
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "p1",
			Namespace: "default",
			Annotations: map[string]string{
				"kontroler/task-rid": "123",
				"kontroler/dagRun-id":   "456",
			},
			UID: types.UID("uid-1"),
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}

	// push event
	eventTime := time.Now()
	revent := &queue.PodEvent{Pod: pod, Event: "update", EventTime: &eventTime}
	require.NoError(t, q.Push(revent))

	// wait for processing
	waitUntil := time.Now().Add(2 * time.Second)
	for time.Now().Before(waitUntil) {
		if atomic.LoadInt32(&fdb.markPodStatusCalled) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	require.Equal(t, int32(1), atomic.LoadInt32(&fdb.markPodStatusCalled))
	require.Equal(t, types.UID("uid-1"), fdb.lastPodUID)
	require.Equal(t, "p1", fdb.lastName)
	require.Equal(t, 123, fdb.lastTaskRunID)
	require.Equal(t, v1.PodRunning, fdb.lastPhase)
}
