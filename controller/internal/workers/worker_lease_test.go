package workers

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	v1alpha1 "kontroler-controller/api/v1alpha1"
	"kontroler-controller/internal/db"
	"kontroler-controller/internal/queue"
	"kontroler-controller/internal/webhook"
)

// fakeDBLease implements the minimal DBDAGManager methods used by processClaim
// with instrumentation for RenewLease and FinalizeClaimToRunning.
type fakeDBLease struct {
	renewCount   int32
	finalized    int32
	lastFinalPod string
}

func (f *fakeDBLease) InitaliseDatabase(ctx context.Context) error { return nil }
func (f *fakeDBLease) GetID(ctx context.Context) (string, error)   { return "fake", nil }
func (f *fakeDBLease) GetDAGsToStartAndUpdate(ctx context.Context, tm time.Time) ([]*db.DagInfo, error) {
	return nil, nil
}
func (f *fakeDBLease) InsertDAG(ctx context.Context, dag *v1.Pod, namespace string) error { return nil }
func (f *fakeDBLease) CreateDAGRun(ctx context.Context, name string, dag *db.DagRunSpec, parameters map[string]db.Parameter, pvcName *string) (int, error) {
	return 0, nil
}

// Many methods are unused by this test — provide simple stubs
func (f *fakeDBLease) GetStartingTasks(ctx context.Context, dagName string, dagrun int) ([]db.Task, error) {
	return nil, nil
}
func (f *fakeDBLease) MarkTaskAsStarted(ctx context.Context, runId, taskId int) (int, error) {
	return 0, nil
}
func (f *fakeDBLease) IncrementAttempts(ctx context.Context, taskRunId int) error { return nil }
func (f *fakeDBLease) MarkSuccessAndGetNextTasks(ctx context.Context, taskRunId int) ([]db.Task, error) {
	return nil, nil
}
func (f *fakeDBLease) MarkDAGRunOutcome(ctx context.Context, dagRunId int, outcome string) error {
	return nil
}
func (f *fakeDBLease) GetDagParameters(ctx context.Context, dagName string) (map[string]*db.Parameter, error) {
	return nil, nil
}
func (f *fakeDBLease) DagExists(ctx context.Context, dagName string) (bool, int, error) {
	return false, 0, nil
}
func (f *fakeDBLease) ShouldRerun(ctx context.Context, taskRunid int, exitCode int32) (bool, error) {
	return false, nil
}
func (f *fakeDBLease) MarkTaskAsFailed(ctx context.Context, taskRunId int) error { return nil }
func (f *fakeDBLease) MarkPodStatus(ctx context.Context, podUid types.UID, name string, taskRunID int, status v1.PodPhase, tStamp time.Time, exitCode *int32, namespace string) error {
	return nil
}
func (f *fakeDBLease) DeleteDAG(ctx context.Context, name string, namespace string) ([]string, error) {
	return nil, nil
}
func (f *fakeDBLease) FindExistingDAGRun(ctx context.Context, name string) (bool, error) {
	return false, nil
}
func (f *fakeDBLease) GetTaskScriptAndInjectorImage(ctx context.Context, taskId int) (*string, *string, error) {
	return nil, nil, nil
}
func (f *fakeDBLease) AddTask(ctx context.Context, task *v1.Pod, namespace string) error { return nil }
func (f *fakeDBLease) DeleteTask(ctx context.Context, taskName string, namespace string) error {
	return nil
}
func (f *fakeDBLease) GetTaskRefsParameters(ctx context.Context, taskRefs []db.TaskRef) (map[db.TaskRef][]string, error) {
	return nil, nil
}
func (f *fakeDBLease) GetWebhookDetails(ctx context.Context, dagRunID int) (*v1alpha1.Webhook, error) {
	return nil, nil
}
func (f *fakeDBLease) GetWorkspacePVCTemplate(ctx context.Context, dagId int) (*db.PVC, error) {
	return nil, nil
}
func (f *fakeDBLease) CheckIfAllTasksDone(ctx context.Context, dagRunID int) (bool, error) {
	return true, nil
}
func (f *fakeDBLease) MarkConnectingTasksAsSuspended(ctx context.Context, dagRunID, taskRunId int) ([]string, error) {
	return nil, nil
}
func (f *fakeDBLease) AddPodDuration(ctx context.Context, taskRunId int, durationSec int64) error {
	return nil
}
func (f *fakeDBLease) SuspendDagRun(ctx context.Context, dagRunId int) ([]db.RunningPodInfo, error) {
	return nil, nil
}
func (f *fakeDBLease) DeleteDagRun(ctx context.Context, dagRunId int) error { return nil }
func (f *fakeDBLease) DagrunExists(ctx context.Context, dagrunId int) (bool, error) {
	return false, nil
}
func (f *fakeDBLease) GetTaskRunInfo(ctx context.Context, taskRunId int) (dagName, taskName, namespace string, err error) {
	return "d", "t", "ns", nil
}

// Claim/lease primitives
func (f *fakeDBLease) ClaimTasks(ctx context.Context, limit int, workerId string, leaseTTL time.Duration) ([]db.TaskClaim, error) {
	return nil, nil
}

func (f *fakeDBLease) RenewLease(ctx context.Context, taskRunId int, workerId string, leaseTTL time.Duration) error {
	atomic.AddInt32(&f.renewCount, 1)
	return nil
}

func (f *fakeDBLease) FinalizeClaimToRunning(ctx context.Context, taskRunId int, workerId string, podUID string) error {
	atomic.StoreInt32(&f.finalized, 1)
	f.lastFinalPod = podUID
	return nil
}

func (f *fakeDBLease) RecoverExpiredLeases(ctx context.Context) (int, error) { return 0, nil }
func (f *fakeDBLease) AddPendingTaskRun(ctx context.Context, runId int, dagTaskId int) (int, error) {
	return 0, nil
}
func (f *fakeDBLease) GetTaskForRun(ctx context.Context, runId int, dagTaskId int) (db.Task, string, string, error) {
	// return simple task
	return db.Task{Id: dagTaskId, Name: "t", Image: "busybox", Command: []string{"/bin/sh"}, Args: []string{"-c", "echo hi"}}, "default", "", nil
}
func (f *fakeDBLease) ClaimTaskByID(ctx context.Context, taskRunId int, workerId string, leaseTTL time.Duration) (db.TaskClaim, error) {
	return db.TaskClaim{}, nil
}
func (f *fakeDBLease) SaveRetryEnv(ctx context.Context, taskRunId int, envJSON string) error {
	return nil
}
func (f *fakeDBLease) GetTaskRunStatus(ctx context.Context, taskRunId int) (string, error) {
	return "", nil
}

// fakeAllocator sleeps to simulate slow allocation and returns a pod UID
type fakeAllocator struct {
	delay time.Duration
}

func (f *fakeAllocator) AllocateTask(ctx context.Context, task *db.Task, dagRunId, taskRunId int, namespace string, claimedBy string) (types.UID, error) {
	time.Sleep(f.delay)
	return types.UID("fake-pod-uid"), nil
}
func (f *fakeAllocator) AllocateTaskWithEnv(ctx context.Context, task *db.Task, dagRunId, taskRunId int, namespace string, envs []v1.EnvVar, resources *v1.ResourceRequirements, claimedBy string) (types.UID, error) {
	return f.AllocateTask(ctx, task, dagRunId, taskRunId, namespace, claimedBy)
}
func (f *fakeAllocator) CreateEnvs(task *db.Task) *[]v1.EnvVar { v := []v1.EnvVar{}; return &v }

func TestRenewLeaseCalledDuringSlowAllocation(t *testing.T) {
	// make the lease small so renew ticker fires quickly
	old := defaultLeaseTTL
	defaultLeaseTTL = 200 * time.Millisecond
	defer func() { defaultLeaseTTL = old }()

	fdb := &fakeDBLease{}
	alloc := &fakeAllocator{delay: 700 * time.Millisecond} // longer than defaultLeaseTTL so renew should happen
	q := queue.NewMemoryQueue(context.Background())
	webhookChan := make(chan webhook.WebhookPayload, 1)
	wIface := NewWorker(q, nil, webhookChan, fdb, nil, alloc, 10*time.Millisecond)
	w := wIface.(*worker)

	// run processClaim directly with a fake claim
	cl := db.TaskClaim{TaskRunID: 1, TaskID: 1, RunID: 1}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go w.processClaim(ctx, cl)

	// wait for RenewLease to be called at least once
	waitUntil := time.Now().Add(2 * time.Second)
	for time.Now().Before(waitUntil) {
		if atomic.LoadInt32(&fdb.renewCount) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	require.GreaterOrEqual(t, atomic.LoadInt32(&fdb.renewCount), int32(1), "expected RenewLease to be called at least once")

	// ensure finalize was called eventually
	tWait := time.Now().Add(2 * time.Second)
	for time.Now().Before(tWait) {
		if atomic.LoadInt32(&fdb.finalized) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	require.Equal(t, int32(1), atomic.LoadInt32(&fdb.finalized), "expected FinalizeClaimToRunning to be called")
}
