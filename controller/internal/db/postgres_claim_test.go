package db_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"kontroler-controller/api/v1alpha1"
	"kontroler-controller/internal/db"
	"kontroler-controller/internal/utils"

	"github.com/google/uuid"
	cron "github.com/robfig/cron/v3"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestClaimTaskByID_Concurrent ensures only one caller can successfully ClaimTaskByID for the same task_run
func TestClaimTaskByID_Concurrent(t *testing.T) {
	pool, err := utils.SetupPostgresContainer(context.Background())
	require.NoError(t, err)
	defer pool.Close()

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(t, err)

	require.NoError(t, dm.InitaliseDatabase(context.Background()))

	// create a simple DAG with one task
	dag := &v1alpha1.DAG{
		ObjectMeta: metav1.ObjectMeta{Name: "claim-test-dag"},
		Spec: v1alpha1.DAGSpec{
			Schedule: "*/5 * * * *",
			Task:     []v1alpha1.TaskSpec{{Name: "t1", Command: []string{"echo"}, Args: []string{"hello"}, Image: "busybox"}},
		},
	}

	require.NoError(t, dm.InsertDAG(context.Background(), dag, "default"))

	// create a dag run
	runID, err := dm.CreateDAGRun(context.Background(), "name", &v1alpha1.DagRunSpec{DagName: "claim-test-dag"}, map[string]v1alpha1.ParameterSpec{}, nil)
	require.NoError(t, err)

	// Add a pending task run for dag task id 1
	taskRunID, err := dm.AddPendingTaskRun(context.Background(), runID, 1)
	require.NoError(t, err)

	// Attempt concurrent ClaimTaskByID from multiple goroutines
	const runners = 10
	wg := sync.WaitGroup{}
	wg.Add(runners)

	var mu sync.Mutex
	successCount := 0

	for i := 0; i < runners; i++ {
		go func(i int) {
			defer wg.Done()
			workerID := uuid.NewString()
			_, err := dm.ClaimTaskByID(context.Background(), taskRunID, workerID, 60*time.Second)
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	require.Equal(t, 1, successCount, "exactly one ClaimTaskByID should succeed")
}
