package db_test

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kontroler-controller/api/v1alpha1"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestFinalizeClaimToRunning_Ownership asserts only the owning worker can finalize a claim to running
func TestFinalizeClaimToRunning_Ownership(t *testing.T) {
	ctx := context.Background()
	dm := getPostgresManager(t)

	dag := &v1alpha1.DAG{ObjectMeta: metav1.ObjectMeta{Name: t.Name() + "-finalize-dag"}, Spec: v1alpha1.DAGSpec{Schedule: "*/5 * * * *", Task: []v1alpha1.TaskSpec{{Name: "t1", Command: []string{"echo"}, Image: "busybox"}}}}
	require.NoError(t, dm.InsertDAG(ctx, dag, "default"))
	runID, err := dm.CreateDAGRun(ctx, t.Name()+"-finalize-run", &v1alpha1.DagRunSpec{DagName: t.Name() + "-finalize-dag"}, map[string]v1alpha1.ParameterSpec{}, nil)
	require.NoError(t, err)

	taskRunID, err := dm.AddPendingTaskRun(ctx, runID, 1)
	require.NoError(t, err)

	owner := uuid.NewString()
	_, err = dm.ClaimTaskByID(ctx, taskRunID, owner, 60*time.Second)
	require.NoError(t, err)

	// wrong worker should fail
	if err := dm.FinalizeClaimToRunning(ctx, taskRunID, "wrong-worker", ""); err == nil {
		t.Fatalf("expected finalize to fail for non-owner")
	}

	// owner should succeed
	require.NoError(t, dm.FinalizeClaimToRunning(ctx, taskRunID, owner, ""))

	// verify status is running
	status, err := dm.GetTaskRunStatus(ctx, taskRunID)
	require.NoError(t, err)
	require.Equal(t, "running", status)
}

// TestClaimTasks_RespectScheduledStart ensures tasks with scheduled_start in the future are not claimed
func TestClaimTasks_RespectScheduledStart(t *testing.T) {
	ctx := context.Background()
	dm := getPostgresManager(t)

	dag := &v1alpha1.DAG{ObjectMeta: metav1.ObjectMeta{Name: t.Name() + "-sched-dag"}, Spec: v1alpha1.DAGSpec{Schedule: "*/5 * * * *", Task: []v1alpha1.TaskSpec{{Name: "t1", Command: []string{"echo"}, Image: "busybox"}}}}
	require.NoError(t, dm.InsertDAG(ctx, dag, "default"))
	runID, err := dm.CreateDAGRun(ctx, t.Name()+"-sched-run", &v1alpha1.DagRunSpec{DagName: t.Name() + "-sched-dag"}, map[string]v1alpha1.ParameterSpec{}, nil)
	require.NoError(t, err)

	// create two pending tasks
	r1, err := dm.AddPendingTaskRun(ctx, runID, 1)
	require.NoError(t, err)
	r2, err := dm.AddPendingTaskRun(ctx, runID, 1)
	require.NoError(t, err)

	// set scheduled_start for r2 to future via raw SQL
	pool := getPGPool(t)
	// set explicit scheduled_start times to make test deterministic
	_, err = pool.Exec(ctx, `UPDATE Task_Runs SET scheduled_start = now() - interval '1 minute' WHERE task_run_id = $1`, r1)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE Task_Runs SET scheduled_start = now() + interval '5 minute' WHERE task_run_id = $1`, r2)
	require.NoError(t, err)

	// claim batch of 10
	worker := uuid.NewString()
	claimed, err := dm.ClaimTasks(ctx, 10, worker, 60*time.Second)
	require.NoError(t, err)

	// ensure only r1 claimed (and not r2)
	foundR1 := false
	foundR2 := false
	for _, c := range claimed {
		if c.TaskRunID == r1 {
			foundR1 = true
		}
		if c.TaskRunID == r2 {
			foundR2 = true
		}
	}
	require.True(t, foundR1)
	require.False(t, foundR2)
}
