package db_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"kontroler-controller/api/v1alpha1"
	"kontroler-controller/internal/db"
	"kontroler-controller/internal/utils"

	cron "github.com/robfig/cron/v3"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// helper to build a DAG with many tasks and shared parameters
func buildHeavyDAG(numTasks, numParams int) *v1alpha1.DAG {
	params := make([]v1alpha1.DagParameterSpec, 0, numParams)
	for i := 0; i < numParams; i++ {
		params = append(params, v1alpha1.DagParameterSpec{Name: fmt.Sprintf("p%d", i), DefaultValue: fmt.Sprintf("v%d", i)})
	}

	tasks := make([]v1alpha1.TaskSpec, 0, numTasks)
	for i := 0; i < numTasks; i++ {
		pnames := make([]string, 0, numParams)
		for j := 0; j < numParams; j++ {
			pnames = append(pnames, fmt.Sprintf("p%d", j))
		}
		tasks = append(tasks, v1alpha1.TaskSpec{
			Name:       fmt.Sprintf("task-%d", i),
			Command:    []string{"echo"},
			Image:      "alpine",
			Parameters: pnames,
		})
	}

	return &v1alpha1.DAG{
		ObjectMeta: metav1.ObjectMeta{Name: "bench_dag"},
		Spec: v1alpha1.DAGSpec{
			Parameters: params,
			Task:       tasks,
		},
	}
}

func BenchmarkGetStartingTasks_SQLite(b *testing.B) {
	b.ReportAllocs()
	dbPath := fmt.Sprintf("/tmp/bench_%d.db", time.Now().UnixNano())
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	dm, dbConn, err := db.NewSqliteManager(context.Background(), &parser, &db.SQLiteConfig{DBPath: dbPath})
	require.NoError(b, err)
	defer func() { dbConn.Close(); os.Remove(dbPath) }()

	require.NoError(b, dm.InitaliseDatabase(context.Background()))

	dag := buildHeavyDAG(50, 5) // 50 tasks, 5 shared params
	require.NoError(b, dm.InsertDAG(context.Background(), dag, "default"))

	runID, err := dm.CreateDAGRun(context.Background(), "run", &v1alpha1.DagRunSpec{DagName: "bench_dag"}, map[string]v1alpha1.ParameterSpec{}, nil)
	require.NoError(b, err)

	// warmup
	_, err = dm.GetStartingTasks(context.Background(), "bench_dag", runID)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := dm.GetStartingTasks(context.Background(), "bench_dag", runID)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetStartingTasks_Postgres(b *testing.B) {
	b.ReportAllocs()
	pool, err := utils.SetupPostgresContainer(context.Background())
	require.NoError(b, err)
	defer func() { _ = pool.Close() }()

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	dm, err := db.NewPostgresDAGManager(context.Background(), pool, &parser)
	require.NoError(b, err)
	require.NoError(b, dm.InitaliseDatabase(context.Background()))

	dag := buildHeavyDAG(50, 5)
	require.NoError(b, dm.InsertDAG(context.Background(), dag, "default"))

	runID, err := dm.CreateDAGRun(context.Background(), "run", &v1alpha1.DagRunSpec{DagName: "bench_dag"}, map[string]v1alpha1.ParameterSpec{}, nil)
	require.NoError(b, err)

	// warmup
	_, err = dm.GetStartingTasks(context.Background(), "bench_dag", runID)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := dm.GetStartingTasks(context.Background(), "bench_dag", runID)
		if err != nil {
			b.Fatal(err)
		}
	}
}
