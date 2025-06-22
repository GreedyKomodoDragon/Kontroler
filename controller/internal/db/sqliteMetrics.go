package db

import (
	"context"
	"database/sql"
	"time"

	"kontroler-controller/api/v1alpha1"
	"kontroler-controller/internal/metrics"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// MetricsSqliteDAGManager wraps sqliteDAGManager with metrics
type MetricsSqliteDAGManager struct {
	*sqliteDAGManager
	db     *sql.DB
	ctx    context.Context
	cancel context.CancelFunc
}

// NewMetricsSqliteDAGManager creates a new SQLite DAG manager with metrics
func NewMetricsSqliteDAGManager(manager *sqliteDAGManager, db *sql.DB) DBDAGManager {
	ctx, cancel := context.WithCancel(context.Background())
	wrapped := &MetricsSqliteDAGManager{
		sqliteDAGManager: manager,
		db:               db,
		ctx:              ctx,
		cancel:           cancel,
	}

	// Start background metrics collection
	go wrapped.collectConnectionMetrics()
	go wrapped.collectContentMetrics()

	return wrapped
}

// collectConnectionMetrics periodically collects connection pool metrics
func (m *MetricsSqliteDAGManager) collectConnectionMetrics() {
	logger := log.FromContext(m.ctx).WithName("metrics").WithValues("type", "connection")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Collect initial metrics
	stats := m.db.Stats()
	metrics.UpdateConnectionMetrics(
		"sqlite",
		stats.InUse,
		stats.Idle,
		stats.MaxOpenConnections,
	)
	log.Log.Info("Initial connection metrics collected",
		"active", stats.InUse,
		"idle", stats.Idle,
		"max", stats.MaxOpenConnections)

	for {
		select {
		case <-m.ctx.Done():
			logger.Info("Connection metrics collection stopped")
			return
		case <-ticker.C:
			stats := m.db.Stats()
			metrics.UpdateConnectionMetrics(
				"sqlite",
				stats.InUse,
				stats.Idle,
				stats.MaxOpenConnections,
			)
			log.Log.Info("Connection metrics updated",
				"active", stats.InUse,
				"idle", stats.Idle,
				"max", stats.MaxOpenConnections)
		}
	}
}

// collectContentMetrics periodically collects database content metrics
func (m *MetricsSqliteDAGManager) collectContentMetrics() {
	logger := log.FromContext(m.ctx).WithName("metrics").WithValues("type", "content")
	ticker := time.NewTicker(60 * time.Second) // Collect every minute
	defer ticker.Stop()

	// Collect initial metrics
	m.updateContentMetrics(logger)

	for {
		select {
		case <-m.ctx.Done():
			logger.Info("Content metrics collection stopped")
			return
		case <-ticker.C:
			m.updateContentMetrics(logger)
		}
	}
}

// updateContentMetrics collects and updates database content metrics
func (m *MetricsSqliteDAGManager) updateContentMetrics(logger logr.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Collect DAG metrics grouped by namespace
	rows, err := m.db.QueryContext(ctx, `
		SELECT 
			namespace,
			COUNT(CASE WHEN active = 1 AND suspended = 0 THEN 1 END) as active,
			COUNT(CASE WHEN active = 1 AND suspended = 1 THEN 1 END) as suspended,
			COUNT(CASE WHEN active = 0 THEN 1 END) as inactive
		FROM DAGs
		GROUP BY namespace
	`)

	if err != nil {
		logger.Error(err, "Failed to collect DAG metrics")
		metrics.RecordErrorMetrics("sqlite", "collect_content_metrics", "dag_counts_error")
		return
	}
	defer rows.Close()

	// Process each namespace's DAG counts
	for rows.Next() {
		var namespace string
		var active, suspended, inactive int

		if err := rows.Scan(&namespace, &active, &suspended, &inactive); err != nil {
			logger.Error(err, "Failed to scan DAG metrics row")
			metrics.RecordErrorMetrics("sqlite", "collect_content_metrics", "dag_scan_error")
			continue
		}

		log.Log.Info("DAG metrics collected", "namespace", namespace, "active", active, "suspended", suspended, "inactive", inactive)

		dagCounts := map[string]int{
			"active":    active,
			"suspended": suspended,
			"inactive":  inactive,
		}

		// Collect DAG run metrics for this namespace
		var runningRuns, successRuns, failedRuns, pendingRuns int
		dagRunErr := m.db.QueryRowContext(ctx, `
			SELECT 
				COUNT(CASE WHEN dr.status = 'running' THEN 1 END) as running,
				COUNT(CASE WHEN dr.status = 'success' THEN 1 END) as success,
				COUNT(CASE WHEN dr.status = 'failed' THEN 1 END) as failed,
				COUNT(CASE WHEN dr.status = 'pending' THEN 1 END) as pending
			FROM DAG_Runs dr
			JOIN DAGs d ON dr.dag_id = d.dag_id
			WHERE d.namespace = ?
		`, namespace).Scan(&runningRuns, &successRuns, &failedRuns, &pendingRuns)

		if dagRunErr != nil {
			logger.Error(dagRunErr, "Failed to collect DAG run metrics", "namespace", namespace)
			metrics.RecordErrorMetrics("sqlite", "collect_content_metrics", "dag_run_counts_error")
			continue
		}

		log.Log.Info("DAG run metrics collected", "namespace", namespace, "running", runningRuns, "success", successRuns, "failed", failedRuns, "pending", pendingRuns)

		dagRunCounts := map[string]int{
			"running": runningRuns,
			"success": successRuns,
			"failed":  failedRuns,
			"pending": pendingRuns,
		}

		// Collect task run metrics for this namespace
		var runningTasks, successTasks, failedTasks, pendingTasks int
		taskRunErr := m.db.QueryRowContext(ctx, `
			SELECT 
				COUNT(CASE WHEN tr.status = 'running' THEN 1 END) as running,
				COUNT(CASE WHEN tr.status = 'success' THEN 1 END) as success,
				COUNT(CASE WHEN tr.status = 'failed' THEN 1 END) as failed,
				COUNT(CASE WHEN tr.status = 'pending' THEN 1 END) as pending
			FROM Task_Runs tr
			JOIN DAG_Runs dr ON tr.run_id = dr.run_id
			JOIN DAGs d ON dr.dag_id = d.dag_id
			WHERE d.namespace = ?
		`, namespace).Scan(&runningTasks, &successTasks, &failedTasks, &pendingTasks)

		if taskRunErr != nil {
			logger.Error(taskRunErr, "Failed to collect task run metrics", "namespace", namespace)
			metrics.RecordErrorMetrics("sqlite", "collect_content_metrics", "task_run_counts_error")
			continue
		}

		log.Log.Info("Task run metrics collected", "namespace", namespace, "running", runningTasks, "success", successTasks, "failed", failedTasks, "pending", pendingTasks)

		taskRunCounts := map[string]int{
			"running": runningTasks,
			"success": successTasks,
			"failed":  failedTasks,
			"pending": pendingTasks,
		}

		// Update metrics for this namespace
		metrics.UpdateContentMetrics("sqlite", namespace, dagCounts, dagRunCounts, taskRunCounts)
		log.Log.Info("Content metrics updated successfully", "namespace", namespace)
	}
}

// recordQueryMetrics is a helper to record query metrics
func (m *MetricsSqliteDAGManager) recordQueryMetrics(operation, table string, start time.Time, err error) {
	duration := time.Since(start).Seconds()
	status := "success"
	if err != nil {
		status = "error"
		metrics.RecordErrorMetrics("sqlite", operation, "query_error")
	}
	metrics.RecordQueryMetrics("sqlite", operation, table, status, duration)
}

// recordTransactionMetrics is a helper to record transaction metrics
func (m *MetricsSqliteDAGManager) recordTransactionMetrics(operation string, start time.Time, err error) {
	duration := time.Since(start).Seconds()
	status := "success"
	if err != nil {
		status = "error"
		metrics.RecordErrorMetrics("sqlite", operation, "transaction_error")
	}
	metrics.RecordTransactionMetrics("sqlite", operation, status, duration)
}

// Stop gracefully stops the metrics collection
func (m *MetricsSqliteDAGManager) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

// Implement the interface methods with metrics wrapper

func (m *MetricsSqliteDAGManager) InitaliseDatabase(ctx context.Context) error {
	start := time.Now()
	err := m.sqliteDAGManager.InitaliseDatabase(ctx)
	m.recordTransactionMetrics("initialize_database", start, err)
	return err
}

func (m *MetricsSqliteDAGManager) GetID(ctx context.Context) (string, error) {
	start := time.Now()
	result, err := m.sqliteDAGManager.GetID(ctx)
	m.recordQueryMetrics("select", "idtable", start, err)
	return result, err
}

func (m *MetricsSqliteDAGManager) GetDAGsToStartAndUpdate(ctx context.Context, tm time.Time) ([]*DagInfo, error) {
	start := time.Now()
	result, err := m.sqliteDAGManager.GetDAGsToStartAndUpdate(ctx, tm)
	m.recordTransactionMetrics("get_dags_to_start_and_update", start, err)
	return result, err
}

func (m *MetricsSqliteDAGManager) InsertDAG(ctx context.Context, dag *v1alpha1.DAG, namespace string) error {
	start := time.Now()
	err := m.sqliteDAGManager.InsertDAG(ctx, dag, namespace)
	m.recordTransactionMetrics("insert_dag", start, err)
	return err
}

func (m *MetricsSqliteDAGManager) CreateDAGRun(ctx context.Context, name string, dag *v1alpha1.DagRunSpec, parameters map[string]v1alpha1.ParameterSpec, pvcName *string) (int, error) {
	start := time.Now()
	result, err := m.sqliteDAGManager.CreateDAGRun(ctx, name, dag, parameters, pvcName)
	m.recordTransactionMetrics("create_dag_run", start, err)
	return result, err
}

func (m *MetricsSqliteDAGManager) GetStartingTasks(ctx context.Context, dagName string, dagrun int) ([]Task, error) {
	start := time.Now()
	result, err := m.sqliteDAGManager.GetStartingTasks(ctx, dagName, dagrun)
	m.recordQueryMetrics("select", "tasks", start, err)
	return result, err
}

func (m *MetricsSqliteDAGManager) MarkTaskAsStarted(ctx context.Context, runId, taskId int) (int, error) {
	start := time.Now()
	result, err := m.sqliteDAGManager.MarkTaskAsStarted(ctx, runId, taskId)
	m.recordQueryMetrics("insert", "task_runs", start, err)
	return result, err
}

func (m *MetricsSqliteDAGManager) IncrementAttempts(ctx context.Context, taskRunId int) error {
	start := time.Now()
	err := m.sqliteDAGManager.IncrementAttempts(ctx, taskRunId)
	m.recordQueryMetrics("update", "task_runs", start, err)
	return err
}

func (m *MetricsSqliteDAGManager) MarkSuccessAndGetNextTasks(ctx context.Context, taskRunId int) ([]Task, error) {
	start := time.Now()
	result, err := m.sqliteDAGManager.MarkSuccessAndGetNextTasks(ctx, taskRunId)
	m.recordTransactionMetrics("mark_success_and_get_next_tasks", start, err)
	return result, err
}

func (m *MetricsSqliteDAGManager) MarkDAGRunOutcome(ctx context.Context, dagRunId int, outcome string) error {
	start := time.Now()
	err := m.sqliteDAGManager.MarkDAGRunOutcome(ctx, dagRunId, outcome)
	m.recordQueryMetrics("update", "dag_runs", start, err)
	return err
}

func (m *MetricsSqliteDAGManager) GetDagParameters(ctx context.Context, dagName string) (map[string]*Parameter, error) {
	start := time.Now()
	result, err := m.sqliteDAGManager.GetDagParameters(ctx, dagName)
	m.recordQueryMetrics("select", "dag_parameters", start, err)
	return result, err
}

func (m *MetricsSqliteDAGManager) DagExists(ctx context.Context, dagName string) (bool, int, error) {
	start := time.Now()
	exists, id, err := m.sqliteDAGManager.DagExists(ctx, dagName)
	m.recordQueryMetrics("select", "dags", start, err)
	return exists, id, err
}

func (m *MetricsSqliteDAGManager) ShouldRerun(ctx context.Context, taskRunid int, exitCode int32) (bool, error) {
	start := time.Now()
	result, err := m.sqliteDAGManager.ShouldRerun(ctx, taskRunid, exitCode)
	m.recordQueryMetrics("select", "task_runs", start, err)
	return result, err
}

func (m *MetricsSqliteDAGManager) MarkTaskAsFailed(ctx context.Context, taskRunId int) error {
	start := time.Now()
	err := m.sqliteDAGManager.MarkTaskAsFailed(ctx, taskRunId)
	m.recordQueryMetrics("update", "task_runs", start, err)
	return err
}

func (m *MetricsSqliteDAGManager) MarkPodStatus(ctx context.Context, podUid types.UID, name string, taskRunID int, status v1.PodPhase, tStamp time.Time, exitCode *int32, namespace string) error {
	start := time.Now()
	err := m.sqliteDAGManager.MarkPodStatus(ctx, podUid, name, taskRunID, status, tStamp, exitCode, namespace)
	m.recordQueryMetrics("upsert", "task_pods", start, err)
	return err
}

func (m *MetricsSqliteDAGManager) DeleteDAG(ctx context.Context, name string, namespace string) ([]string, error) {
	start := time.Now()
	result, err := m.sqliteDAGManager.DeleteDAG(ctx, name, namespace)
	m.recordTransactionMetrics("delete_dag", start, err)
	return result, err
}

func (m *MetricsSqliteDAGManager) FindExistingDAGRun(ctx context.Context, name string) (bool, error) {
	start := time.Now()
	result, err := m.sqliteDAGManager.FindExistingDAGRun(ctx, name)
	m.recordQueryMetrics("select", "dag_runs", start, err)
	return result, err
}

func (m *MetricsSqliteDAGManager) GetTaskScriptAndInjectorImage(ctx context.Context, taskId int) (*string, *string, error) {
	start := time.Now()
	script, injectorImage, err := m.sqliteDAGManager.GetTaskScriptAndInjectorImage(ctx, taskId)
	m.recordQueryMetrics("select", "tasks", start, err)
	return script, injectorImage, err
}

func (m *MetricsSqliteDAGManager) AddTask(ctx context.Context, task *v1alpha1.DagTask, namespace string) error {
	start := time.Now()
	err := m.sqliteDAGManager.AddTask(ctx, task, namespace)
	m.recordTransactionMetrics("add_task", start, err)
	return err
}

func (m *MetricsSqliteDAGManager) DeleteTask(ctx context.Context, taskName string, namespace string) error {
	start := time.Now()
	err := m.sqliteDAGManager.DeleteTask(ctx, taskName, namespace)
	m.recordTransactionMetrics("delete_task", start, err)
	return err
}

func (m *MetricsSqliteDAGManager) GetTaskRefsParameters(ctx context.Context, taskRefs []v1alpha1.TaskRef) (map[v1alpha1.TaskRef][]string, error) {
	start := time.Now()
	result, err := m.sqliteDAGManager.GetTaskRefsParameters(ctx, taskRefs)
	m.recordQueryMetrics("select", "tasks", start, err)
	return result, err
}

func (m *MetricsSqliteDAGManager) GetWebhookDetails(ctx context.Context, dagRunID int) (*v1alpha1.Webhook, error) {
	start := time.Now()
	result, err := m.sqliteDAGManager.GetWebhookDetails(ctx, dagRunID)
	m.recordQueryMetrics("select", "webhooks", start, err)
	return result, err
}

func (m *MetricsSqliteDAGManager) GetWorkspacePVCTemplate(ctx context.Context, dagId int) (*v1alpha1.PVC, error) {
	start := time.Now()
	result, err := m.sqliteDAGManager.GetWorkspacePVCTemplate(ctx, dagId)
	m.recordQueryMetrics("select", "workspace", start, err)
	return result, err
}

func (m *MetricsSqliteDAGManager) CheckIfAllTasksDone(ctx context.Context, dagRunID int) (bool, error) {
	start := time.Now()
	result, err := m.sqliteDAGManager.CheckIfAllTasksDone(ctx, dagRunID)
	m.recordQueryMetrics("select", "dag_runs", start, err)
	return result, err
}

func (m *MetricsSqliteDAGManager) MarkConnectingTasksAsSuspended(ctx context.Context, dagRunID, taskRunId int) ([]string, error) {
	start := time.Now()
	result, err := m.sqliteDAGManager.MarkConnectingTasksAsSuspended(ctx, dagRunID, taskRunId)
	m.recordTransactionMetrics("mark_connecting_tasks_suspended", start, err)
	return result, err
}

func (m *MetricsSqliteDAGManager) AddPodDuration(ctx context.Context, taskRunId int, durationSec int64) error {
	start := time.Now()
	err := m.sqliteDAGManager.AddPodDuration(ctx, taskRunId, durationSec)
	m.recordQueryMetrics("update", "task_pods", start, err)
	return err
}

func (m *MetricsSqliteDAGManager) SuspendDagRun(ctx context.Context, dagRunId int) ([]RunningPodInfo, error) {
	start := time.Now()
	result, err := m.sqliteDAGManager.SuspendDagRun(ctx, dagRunId)
	m.recordTransactionMetrics("suspend_dag_run", start, err)
	return result, err
}

func (m *MetricsSqliteDAGManager) DeleteDagRun(ctx context.Context, dagRunId int) error {
	start := time.Now()
	err := m.sqliteDAGManager.DeleteDagRun(ctx, dagRunId)
	m.recordTransactionMetrics("delete_dag_run", start, err)
	return err
}

func (m *MetricsSqliteDAGManager) DagrunExists(ctx context.Context, dagrunId int) (bool, error) {
	start := time.Now()
	result, err := m.sqliteDAGManager.DagrunExists(ctx, dagrunId)
	m.recordQueryMetrics("select", "dag_runs", start, err)
	return result, err
}
