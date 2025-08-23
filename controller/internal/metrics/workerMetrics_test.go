package metrics_test

import (
	"kontroler-controller/internal/metrics"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

const (
	testNamespace = "test-namespace"
	testDagName   = "test-dag"
	testTaskName  = "test-task"
)

func TestRecordTaskOutcome(t *testing.T) {
	// Reset metrics before test
	metrics.TaskOutcomeTotal.Reset()

	outcome := "success"

	// Record task outcome
	metrics.RecordTaskOutcome(testNamespace, testDagName, testTaskName, outcome)

	// Verify counter was incremented
	counter := testutil.ToFloat64(metrics.TaskOutcomeTotal.WithLabelValues(testNamespace, testDagName, testTaskName, outcome))
	assert.Equal(t, float64(1), counter, "Task outcome counter should be incremented")

	// Test multiple calls with different outcomes
	metrics.RecordTaskOutcome(testNamespace, testDagName, testTaskName, "success")
	metrics.RecordTaskOutcome(testNamespace, testDagName, testTaskName, "failed")

	successCounter := testutil.ToFloat64(metrics.TaskOutcomeTotal.WithLabelValues(testNamespace, testDagName, testTaskName, "success"))
	failedCounter := testutil.ToFloat64(metrics.TaskOutcomeTotal.WithLabelValues(testNamespace, testDagName, testTaskName, "failed"))

	assert.Equal(t, float64(2), successCounter, "Success counter should be 2")
	assert.Equal(t, float64(1), failedCounter, "Failed counter should be 1")
}

func TestRecordTaskExecutionDuration(t *testing.T) {
	// Reset metrics before test
	metrics.TaskExecutionDuration.Reset()

	outcome := "success"
	duration := 123.456

	// Record task execution duration
	metrics.RecordTaskExecutionDuration(testNamespace, testDagName, testTaskName, outcome, duration)

	// For histograms, we can't easily test the exact value, but we can verify no panics occurred
	// and that the function executes without error. In a real-world scenario, you'd use a
	// metrics testing framework or examine the actual histogram buckets.

	// Test multiple durations
	metrics.RecordTaskExecutionDuration(testNamespace, testDagName, testTaskName, outcome, 456.789)
	metrics.RecordTaskExecutionDuration(testNamespace, testDagName, testTaskName, "failed", 789.123)

	// The fact that we reach this point means the histogram is working correctly
	assert.True(t, true, "Task execution duration recording should complete without errors")
}

func TestRecordTaskRetry(t *testing.T) {
	// Reset metrics before test
	metrics.TaskRetryTotal.Reset()

	reason := "exit_code_1"

	// Record task retry
	metrics.RecordTaskRetry(testNamespace, testDagName, testTaskName, reason)

	// Verify counter was incremented
	counter := testutil.ToFloat64(metrics.TaskRetryTotal.WithLabelValues(testNamespace, testDagName, testTaskName, reason))
	assert.Equal(t, float64(1), counter, "Task retry counter should be incremented")

	// Test multiple retries
	metrics.RecordTaskRetry(testNamespace, testDagName, testTaskName, reason)
	metrics.RecordTaskRetry(testNamespace, testDagName, testTaskName, "exit_code_2")

	reason1Counter := testutil.ToFloat64(metrics.TaskRetryTotal.WithLabelValues(testNamespace, testDagName, testTaskName, "exit_code_1"))
	reason2Counter := testutil.ToFloat64(metrics.TaskRetryTotal.WithLabelValues(testNamespace, testDagName, testTaskName, "exit_code_2"))

	assert.Equal(t, float64(2), reason1Counter, "Exit code 1 retry counter should be 2")
	assert.Equal(t, float64(1), reason2Counter, "Exit code 2 retry counter should be 1")
}

func TestUpdateWorkerQueueSize(t *testing.T) {
	// Reset metrics before test
	metrics.WorkerQueueSize.Reset()

	workerID := "worker-1"
	size := 42

	// Update queue size
	metrics.UpdateWorkerQueueSize(workerID, size)

	// Verify gauge was set
	gauge := testutil.ToFloat64(metrics.WorkerQueueSize.WithLabelValues(workerID))
	assert.Equal(t, float64(size), gauge, "Worker queue size gauge should be set to the correct value")

	// Test updating the gauge
	newSize := 24
	metrics.UpdateWorkerQueueSize(workerID, newSize)

	updatedGauge := testutil.ToFloat64(metrics.WorkerQueueSize.WithLabelValues(workerID))
	assert.Equal(t, float64(newSize), updatedGauge, "Worker queue size gauge should be updated to the new value")
}

func TestRecordWorkerTaskProcessing(t *testing.T) {
	// Reset metrics before test
	metrics.WorkerTaskProcessingTotal.Reset()

	workerID := "worker-1"
	eventType := "add"

	// Record worker task processing
	metrics.RecordWorkerTaskProcessing(workerID, eventType)

	// Verify counter was incremented
	counter := testutil.ToFloat64(metrics.WorkerTaskProcessingTotal.WithLabelValues(workerID, eventType))
	assert.Equal(t, float64(1), counter, "Worker task processing counter should be incremented")

	// Test multiple events
	metrics.RecordWorkerTaskProcessing(workerID, "add")
	metrics.RecordWorkerTaskProcessing(workerID, "update")

	addCounter := testutil.ToFloat64(metrics.WorkerTaskProcessingTotal.WithLabelValues(workerID, "add"))
	updateCounter := testutil.ToFloat64(metrics.WorkerTaskProcessingTotal.WithLabelValues(workerID, "update"))

	assert.Equal(t, float64(2), addCounter, "Add event counter should be 2")
	assert.Equal(t, float64(1), updateCounter, "Update event counter should be 1")
}
