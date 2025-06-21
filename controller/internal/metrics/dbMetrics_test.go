package metrics_test

import (
	"kontroler-controller/internal/metrics"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordQueryMetrics(t *testing.T) {
	// Reset metrics before test
	metrics.DatabaseQueryDuration.Reset()
	metrics.DatabaseQueryTotal.Reset()

	dbType := "test_db"
	operation := "select"
	table := "test_table"
	status := "success"
	duration := 0.123

	// Record metrics
	metrics.RecordQueryMetrics(dbType, operation, table, status, duration)

	// Verify counter was incremented
	counter := testutil.ToFloat64(metrics.DatabaseQueryTotal.WithLabelValues(dbType, operation, table, status))
	assert.Equal(t, float64(1), counter, "Query counter should be incremented")

	// Test multiple calls
	metrics.RecordQueryMetrics(dbType, operation, table, status, 0.456)
	metrics.RecordQueryMetrics(dbType, operation, table, "error", 0.789)

	successCounter := testutil.ToFloat64(metrics.DatabaseQueryTotal.WithLabelValues(dbType, operation, table, "success"))
	errorCounter := testutil.ToFloat64(metrics.DatabaseQueryTotal.WithLabelValues(dbType, operation, table, "error"))

	assert.Equal(t, float64(2), successCounter, "Success counter should be 2")
	assert.Equal(t, float64(1), errorCounter, "Error counter should be 1")
}

func TestRecordTransactionMetrics(t *testing.T) {
	// Reset metrics before test
	metrics.DatabaseTransactionDuration.Reset()
	metrics.DatabaseTransactionTotal.Reset()

	dbType := "test_db"
	operation := "insert_dag"
	status := "success"
	duration := 0.567

	// Record metrics
	metrics.RecordTransactionMetrics(dbType, operation, status, duration)

	// Verify counter was incremented
	counter := testutil.ToFloat64(metrics.DatabaseTransactionTotal.WithLabelValues(dbType, operation, status))
	assert.Equal(t, float64(1), counter, "Transaction counter should be incremented")

	// Test multiple calls with different statuses
	metrics.RecordTransactionMetrics(dbType, operation, "success", 0.234)
	metrics.RecordTransactionMetrics(dbType, operation, "error", 0.890)

	successCounter := testutil.ToFloat64(metrics.DatabaseTransactionTotal.WithLabelValues(dbType, operation, "success"))
	errorCounter := testutil.ToFloat64(metrics.DatabaseTransactionTotal.WithLabelValues(dbType, operation, "error"))

	assert.Equal(t, float64(2), successCounter, "Success transaction counter should be 2")
	assert.Equal(t, float64(1), errorCounter, "Error transaction counter should be 1")
}

func TestRecordErrorMetrics(t *testing.T) {
	// Reset metrics before test
	metrics.DatabaseErrorsTotal.Reset()

	dbType := "test_db"
	operation := "select"
	errorType := "connection_timeout"

	// Record error metrics
	metrics.RecordErrorMetrics(dbType, operation, errorType)

	// Verify counter was incremented
	counter := testutil.ToFloat64(metrics.DatabaseErrorsTotal.WithLabelValues(dbType, operation, errorType))
	assert.Equal(t, float64(1), counter, "Error counter should be incremented")

	// Test multiple calls
	metrics.RecordErrorMetrics(dbType, operation, errorType)
	metrics.RecordErrorMetrics(dbType, operation, "query_error")

	timeoutCounter := testutil.ToFloat64(metrics.DatabaseErrorsTotal.WithLabelValues(dbType, operation, "connection_timeout"))
	queryErrorCounter := testutil.ToFloat64(metrics.DatabaseErrorsTotal.WithLabelValues(dbType, operation, "query_error"))

	assert.Equal(t, float64(2), timeoutCounter, "Timeout error counter should be 2")
	assert.Equal(t, float64(1), queryErrorCounter, "Query error counter should be 1")
}

func TestUpdateConnectionMetrics(t *testing.T) {
	// Reset metrics before test
	metrics.DatabaseConnectionsActive.Reset()
	metrics.DatabaseConnectionsIdle.Reset()
	metrics.DatabaseConnectionsMax.Reset()

	dbType := "test_db"
	active := 5
	idle := 3
	max := 10

	// Update connection metrics
	metrics.UpdateConnectionMetrics(dbType, active, idle, max)

	// Verify gauges were set correctly
	activeGauge := testutil.ToFloat64(metrics.DatabaseConnectionsActive.WithLabelValues(dbType))
	idleGauge := testutil.ToFloat64(metrics.DatabaseConnectionsIdle.WithLabelValues(dbType))
	maxGauge := testutil.ToFloat64(metrics.DatabaseConnectionsMax.WithLabelValues(dbType))

	assert.Equal(t, float64(5), activeGauge, "Active connections gauge should be 5")
	assert.Equal(t, float64(3), idleGauge, "Idle connections gauge should be 3")
	assert.Equal(t, float64(10), maxGauge, "Max connections gauge should be 10")

	// Test updating values
	metrics.UpdateConnectionMetrics(dbType, 7, 1, 10)

	activeGauge = testutil.ToFloat64(metrics.DatabaseConnectionsActive.WithLabelValues(dbType))
	idleGauge = testutil.ToFloat64(metrics.DatabaseConnectionsIdle.WithLabelValues(dbType))

	assert.Equal(t, float64(7), activeGauge, "Active connections gauge should be updated to 7")
	assert.Equal(t, float64(1), idleGauge, "Idle connections gauge should be updated to 1")
}

func TestUpdateContentMetrics(t *testing.T) {
	// Reset metrics before test
	metrics.DatabaseDAGsTotal.Reset()
	metrics.DatabaseDAGRunsTotal.Reset()
	metrics.DatabaseTaskRunsTotal.Reset()

	dbType := "test_db"

	dags := map[string]int{
		"active":    10,
		"suspended": 2,
		"inactive":  1,
	}

	dagRuns := map[string]int{
		"running": 5,
		"success": 15,
		"failed":  3,
		"pending": 1,
	}

	taskRuns := map[string]int{
		"running": 8,
		"success": 25,
		"failed":  2,
		"pending": 3,
	}

	// Update content metrics
	metrics.UpdateContentMetrics(dbType, dags, dagRuns, taskRuns)

	// Verify DAG metrics
	activeDags := testutil.ToFloat64(metrics.DatabaseDAGsTotal.WithLabelValues(dbType, "default", "active"))
	suspendedDags := testutil.ToFloat64(metrics.DatabaseDAGsTotal.WithLabelValues(dbType, "default", "suspended"))
	inactiveDags := testutil.ToFloat64(metrics.DatabaseDAGsTotal.WithLabelValues(dbType, "default", "inactive"))

	assert.Equal(t, float64(10), activeDags, "Active DAGs should be 10")
	assert.Equal(t, float64(2), suspendedDags, "Suspended DAGs should be 2")
	assert.Equal(t, float64(1), inactiveDags, "Inactive DAGs should be 1")

	// Verify DAG run metrics
	runningDagRuns := testutil.ToFloat64(metrics.DatabaseDAGRunsTotal.WithLabelValues(dbType, "running"))
	successDagRuns := testutil.ToFloat64(metrics.DatabaseDAGRunsTotal.WithLabelValues(dbType, "success"))
	failedDagRuns := testutil.ToFloat64(metrics.DatabaseDAGRunsTotal.WithLabelValues(dbType, "failed"))
	pendingDagRuns := testutil.ToFloat64(metrics.DatabaseDAGRunsTotal.WithLabelValues(dbType, "pending"))

	assert.Equal(t, float64(5), runningDagRuns, "Running DAG runs should be 5")
	assert.Equal(t, float64(15), successDagRuns, "Success DAG runs should be 15")
	assert.Equal(t, float64(3), failedDagRuns, "Failed DAG runs should be 3")
	assert.Equal(t, float64(1), pendingDagRuns, "Pending DAG runs should be 1")

	// Verify task run metrics
	runningTaskRuns := testutil.ToFloat64(metrics.DatabaseTaskRunsTotal.WithLabelValues(dbType, "running"))
	successTaskRuns := testutil.ToFloat64(metrics.DatabaseTaskRunsTotal.WithLabelValues(dbType, "success"))
	failedTaskRuns := testutil.ToFloat64(metrics.DatabaseTaskRunsTotal.WithLabelValues(dbType, "failed"))
	pendingTaskRuns := testutil.ToFloat64(metrics.DatabaseTaskRunsTotal.WithLabelValues(dbType, "pending"))

	assert.Equal(t, float64(8), runningTaskRuns, "Running task runs should be 8")
	assert.Equal(t, float64(25), successTaskRuns, "Success task runs should be 25")
	assert.Equal(t, float64(2), failedTaskRuns, "Failed task runs should be 2")
	assert.Equal(t, float64(3), pendingTaskRuns, "Pending task runs should be 3")
}

func TestMetricsInitialization(t *testing.T) {
	// Test that all metrics are properly initialized and registered
	require.NotNil(t, metrics.DatabaseConnectionsActive, "DatabaseConnectionsActive should be initialized")
	require.NotNil(t, metrics.DatabaseConnectionsIdle, "DatabaseConnectionsIdle should be initialized")
	require.NotNil(t, metrics.DatabaseConnectionsMax, "DatabaseConnectionsMax should be initialized")
	require.NotNil(t, metrics.DatabaseQueryDuration, "DatabaseQueryDuration should be initialized")
	require.NotNil(t, metrics.DatabaseQueryTotal, "DatabaseQueryTotal should be initialized")
	require.NotNil(t, metrics.DatabaseTransactionDuration, "DatabaseTransactionDuration should be initialized")
	require.NotNil(t, metrics.DatabaseTransactionTotal, "DatabaseTransactionTotal should be initialized")
	require.NotNil(t, metrics.DatabaseDAGsTotal, "DatabaseDAGsTotal should be initialized")
	require.NotNil(t, metrics.DatabaseDAGRunsTotal, "DatabaseDAGRunsTotal should be initialized")
	require.NotNil(t, metrics.DatabaseTaskRunsTotal, "DatabaseTaskRunsTotal should be initialized")
	require.NotNil(t, metrics.DatabaseErrorsTotal, "DatabaseErrorsTotal should be initialized")
}
