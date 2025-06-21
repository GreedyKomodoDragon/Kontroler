package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Database operation metrics
var (
	// Connection pool metrics
	DatabaseConnectionsActive = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kontroler_database_connections_active",
		Help: "Number of active database connections",
	}, []string{"database_type"})

	DatabaseConnectionsIdle = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kontroler_database_connections_idle",
		Help: "Number of idle database connections",
	}, []string{"database_type"})

	DatabaseConnectionsMax = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kontroler_database_connections_max",
		Help: "Maximum number of database connections",
	}, []string{"database_type"})

	// Query performance metrics
	DatabaseQueryDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "kontroler_database_query_duration_seconds",
		Help:    "Duration of database queries in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"database_type", "operation", "table"})

	DatabaseQueryTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kontroler_database_queries_total",
		Help: "Total number of database queries",
	}, []string{"database_type", "operation", "table", "status"})

	// Transaction metrics
	DatabaseTransactionDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "kontroler_database_transaction_duration_seconds",
		Help:    "Duration of database transactions in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"database_type", "operation"})

	DatabaseTransactionTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kontroler_database_transactions_total",
		Help: "Total number of database transactions",
	}, []string{"database_type", "operation", "status"})

	// Database content metrics
	DatabaseDAGsTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kontroler_database_dags_total",
		Help: "Total number of DAGs in the database",
	}, []string{"database_type", "namespace", "status"})

	DatabaseDAGRunsTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kontroler_database_dag_runs_total",
		Help: "Total number of DAG runs in the database",
	}, []string{"database_type", "status"})

	DatabaseTaskRunsTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kontroler_database_task_runs_total",
		Help: "Total number of task runs in the database",
	}, []string{"database_type", "status"})

	// Error metrics
	DatabaseErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kontroler_database_errors_total",
		Help: "Total number of database errors",
	}, []string{"database_type", "operation", "error_type"})

	// Migration metrics
	DatabaseMigrationDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "kontroler_database_migration_duration_seconds",
		Help:    "Duration of database migrations in seconds",
		Buckets: []float64{0.1, 0.5, 1, 5, 10, 30, 60, 300},
	}, []string{"database_type", "migration_version"})

	DatabaseMigrationTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kontroler_database_migrations_total",
		Help: "Total number of database migrations executed",
	}, []string{"database_type", "migration_version", "status"})

	// Cleanup metrics
	DatabaseCleanupDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "kontroler_database_cleanup_duration_seconds",
		Help:    "Duration of database cleanup operations in seconds",
		Buckets: []float64{0.1, 0.5, 1, 5, 10, 30, 60, 300},
	}, []string{"database_type", "cleanup_type"})

	DatabaseCleanupItemsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kontroler_database_cleanup_items_total",
		Help: "Total number of items cleaned up from database",
	}, []string{"database_type", "cleanup_type"})
)

// init function registers all metrics with controller-runtime's metrics registry
func init() {
	// Register all metrics with controller-runtime's metrics registry
	metrics.Registry.MustRegister(
		DatabaseConnectionsActive,
		DatabaseConnectionsIdle,
		DatabaseConnectionsMax,
		DatabaseQueryDuration,
		DatabaseQueryTotal,
		DatabaseTransactionDuration,
		DatabaseTransactionTotal,
		DatabaseDAGsTotal,
		DatabaseDAGRunsTotal,
		DatabaseTaskRunsTotal,
		DatabaseErrorsTotal,
		DatabaseMigrationDuration,
		DatabaseMigrationTotal,
		DatabaseCleanupDuration,
		DatabaseCleanupItemsTotal,
	)
}

// RecordQueryMetrics records metrics for a database query
func RecordQueryMetrics(dbType, operation, table, status string, duration float64) {
	DatabaseQueryDuration.WithLabelValues(dbType, operation, table).Observe(duration)
	DatabaseQueryTotal.WithLabelValues(dbType, operation, table, status).Inc()
}

// RecordTransactionMetrics records metrics for a database transaction
func RecordTransactionMetrics(dbType, operation, status string, duration float64) {
	DatabaseTransactionDuration.WithLabelValues(dbType, operation).Observe(duration)
	DatabaseTransactionTotal.WithLabelValues(dbType, operation, status).Inc()
}

// RecordErrorMetrics records metrics for a database error
func RecordErrorMetrics(dbType, operation, errorType string) {
	DatabaseErrorsTotal.WithLabelValues(dbType, operation, errorType).Inc()
}

// UpdateConnectionMetrics updates connection pool metrics
func UpdateConnectionMetrics(dbType string, active, idle, max int) {
	DatabaseConnectionsActive.WithLabelValues(dbType).Set(float64(active))
	DatabaseConnectionsIdle.WithLabelValues(dbType).Set(float64(idle))
	DatabaseConnectionsMax.WithLabelValues(dbType).Set(float64(max))
}

// UpdateContentMetrics updates database content metrics
func UpdateContentMetrics(dbType string, dags map[string]int, dagRuns map[string]int, taskRuns map[string]int) {
	// Update DAG metrics
	for status, count := range dags {
		DatabaseDAGsTotal.WithLabelValues(dbType, "default", status).Set(float64(count))
	}

	// Update DAG run metrics
	for status, count := range dagRuns {
		DatabaseDAGRunsTotal.WithLabelValues(dbType, status).Set(float64(count))
	}

	// Update task run metrics
	for status, count := range taskRuns {
		DatabaseTaskRunsTotal.WithLabelValues(dbType, status).Set(float64(count))
	}
}
