package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Worker task execution metrics
var (
	// Task outcome metrics
	TaskOutcomeTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kontroler_task_outcome_total",
		Help: "Total number of task outcomes by status",
	}, []string{"namespace", "dag_name", "task_name", "outcome"})

	// Task execution duration metrics
	TaskExecutionDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "kontroler_task_execution_duration_seconds",
		Help:    "Duration of task execution in seconds",
		Buckets: []float64{1, 5, 10, 30, 60, 300, 600, 1800, 3600}, // 1s to 1h
	}, []string{"namespace", "dag_name", "task_name", "outcome"})

	// Task retry metrics
	TaskRetryTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kontroler_task_retry_total",
		Help: "Total number of task retries",
	}, []string{"namespace", "dag_name", "task_name", "retry_reason"})

	// Worker processing metrics
	WorkerQueueSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kontroler_worker_queue_size",
		Help: "Current size of worker queue",
	}, []string{"worker_id"})

	WorkerTaskProcessingTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kontroler_worker_task_processing_total",
		Help: "Total number of tasks processed by workers",
	}, []string{"worker_id", "event_type"})

	// Claim/lease metrics
	TaskClaimsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kontroler_task_claims_total",
		Help: "Total number of task claims attempted",
	}, []string{"worker_id", "result"})

	LeaseRenewTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kontroler_lease_renew_total",
		Help: "Total number of lease renewals attempted",
	}, []string{"worker_id", "result"})

	LeaseExpiredTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kontroler_lease_expired_total",
		Help: "Total number of leases expired and recovered",
	}, []string{"worker_id"})

	ClaimedInFlight = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kontroler_claimed_in_flight",
		Help: "Number of tasks currently claimed by a worker",
	}, []string{"worker_id"})
)

func init() {
	// Register all worker metrics with controller-runtime's metrics registry.
	// Use Register and ignore AlreadyRegisteredError so multiple test runs or
	// re-imports don't panic.
	collectors := []prometheus.Collector{
		TaskOutcomeTotal,
		TaskExecutionDuration,
		TaskRetryTotal,
		WorkerQueueSize,
		WorkerTaskProcessingTotal,
		TaskClaimsTotal,
		LeaseRenewTotal,
		LeaseExpiredTotal,
		ClaimedInFlight,
	}

	for _, c := range collectors {
		if err := metrics.Registry.Register(c); err != nil {
			if _, ok := err.(prometheus.AlreadyRegisteredError); ok {
				// already registered, ignore
				continue
			}
			// other errors are fatal
			panic(err)
		}
	}
}

// RecordTaskOutcome records metrics for a task outcome
func RecordTaskOutcome(namespace, dagName, taskName, outcome string) {
	TaskOutcomeTotal.WithLabelValues(namespace, dagName, taskName, outcome).Inc()
}

// RecordTaskExecutionDuration records metrics for task execution duration
func RecordTaskExecutionDuration(namespace, dagName, taskName, outcome string, duration float64) {
	TaskExecutionDuration.WithLabelValues(namespace, dagName, taskName, outcome).Observe(duration)
}

// RecordTaskRetry records metrics for task retries
func RecordTaskRetry(namespace, dagName, taskName, retryReason string) {
	TaskRetryTotal.WithLabelValues(namespace, dagName, taskName, retryReason).Inc()
}

// UpdateWorkerQueueSize updates the worker queue size metric
func UpdateWorkerQueueSize(workerID string, size int) {
	WorkerQueueSize.WithLabelValues(workerID).Set(float64(size))
}

// RecordWorkerTaskProcessing records metrics for worker task processing
func RecordWorkerTaskProcessing(workerID, eventType string) {
	WorkerTaskProcessingTotal.WithLabelValues(workerID, eventType).Inc()
}

// RecordTaskClaim records claim attempts and results
func RecordTaskClaim(workerID, result string) {
	TaskClaimsTotal.WithLabelValues(workerID, result).Inc()
}

// RecordLeaseRenew records lease renewal attempts and results
func RecordLeaseRenew(workerID, result string) {
	LeaseRenewTotal.WithLabelValues(workerID, result).Inc()
}

// RecordLeaseExpired records lease expirations recovered by the system
func RecordLeaseExpired(workerID string, count int) {
	for i := 0; i < count; i++ {
		LeaseExpiredTotal.WithLabelValues(workerID).Inc()
	}
}

// IncClaimed increments claimed-in-flight gauge
func IncClaimed(workerID string) {
	ClaimedInFlight.WithLabelValues(workerID).Inc()
}

// DecClaimed decrements claimed-in-flight gauge
func DecClaimed(workerID string) {
	ClaimedInFlight.WithLabelValues(workerID).Dec()
}
