package db

import (
	"context"
	"time"

	"kontroler-controller/api/v1alpha1"
	"kontroler-controller/internal/metrics"

	"github.com/go-logr/logr"
	"github.com/jackc/pgx/v5/pgxpool"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// metricsPostgresDAGManager wraps postgresDAGManager with metrics
type metricsPostgresDAGManager struct {
	*postgresDAGManager
	pool   *pgxpool.Pool
	ctx    context.Context
	cancel context.CancelFunc
}

// NewMetricsPostgresDAGManager creates a new PostgreSQL DAG manager with metrics
func NewMetricsPostgresDAGManager(manager *postgresDAGManager, pool *pgxpool.Pool) DBDAGManager {
	ctx, cancel := context.WithCancel(context.Background())
	wrapped := &metricsPostgresDAGManager{
		postgresDAGManager: manager,
		pool:               pool,
		ctx:                ctx,
		cancel:             cancel,
	}

	// Start background metrics collection
	go wrapped.collectConnectionMetrics()
	go wrapped.collectContentMetrics()

	return wrapped
}

// collectConnectionMetrics periodically collects connection pool metrics
func (m *metricsPostgresDAGManager) collectConnectionMetrics() {
	logger := log.FromContext(m.ctx).WithName("metrics").WithValues("type", "connection")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Collect initial metrics
	stats := m.pool.Stat()
	metrics.UpdateConnectionMetrics(
		"postgresql",
		int(stats.AcquiredConns()),
		int(stats.IdleConns()),
		int(stats.MaxConns()),
	)
	log.Log.Info("Initial connection metrics collected",
		"active", stats.AcquiredConns(),
		"idle", stats.IdleConns(),
		"max", stats.MaxConns())

	for {
		select {
		case <-m.ctx.Done():
			logger.Info("Connection metrics collection stopped")
			return
		case <-ticker.C:
			stats := m.pool.Stat()
			metrics.UpdateConnectionMetrics(
				"postgresql",
				int(stats.AcquiredConns()),
				int(stats.IdleConns()),
				int(stats.MaxConns()),
			)
			log.Log.Info("Connection metrics updated",
				"active", stats.AcquiredConns(),
				"idle", stats.IdleConns(),
				"max", stats.MaxConns())
		}
	}
}

// collectContentMetrics periodically collects database content metrics
func (m *metricsPostgresDAGManager) collectContentMetrics() {
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
func (m *metricsPostgresDAGManager) updateContentMetrics(logger logr.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Collect DAG metrics grouped by namespace
	rows, err := m.pool.Query(ctx, `
		SELECT 
			namespace,
			COUNT(CASE WHEN active = true AND suspended = false THEN 1 END) as active,
			COUNT(CASE WHEN active = true AND suspended = true THEN 1 END) as suspended,
			COUNT(CASE WHEN active = false THEN 1 END) as inactive
		FROM DAGs
		GROUP BY namespace
	`)

	if err != nil {
		logger.Error(err, "Failed to collect DAG metrics")
		metrics.RecordErrorMetrics("postgresql", "collect_content_metrics", "dag_counts_error")
		return
	}
	defer rows.Close()

	// Process each namespace's DAG counts
	for rows.Next() {
		var namespace string
		var active, suspended, inactive int

		if err := rows.Scan(&namespace, &active, &suspended, &inactive); err != nil {
			logger.Error(err, "Failed to scan DAG metrics row")
			metrics.RecordErrorMetrics("postgresql", "collect_content_metrics", "dag_scan_error")
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
		dagRunErr := m.pool.QueryRow(ctx, `
			SELECT 
				COUNT(CASE WHEN dr.status = 'running' THEN 1 END) as running,
				COUNT(CASE WHEN dr.status = 'success' THEN 1 END) as success,
				COUNT(CASE WHEN dr.status = 'failed' THEN 1 END) as failed,
				COUNT(CASE WHEN dr.status = 'pending' THEN 1 END) as pending
			FROM DAG_Runs dr
			JOIN DAGs d ON dr.dag_id = d.dag_id
			WHERE d.namespace = $1
		`, namespace).Scan(&runningRuns, &successRuns, &failedRuns, &pendingRuns)

		if dagRunErr != nil {
			logger.Error(dagRunErr, "Failed to collect DAG run metrics", "namespace", namespace)
			metrics.RecordErrorMetrics("postgresql", "collect_content_metrics", "dag_run_counts_error")
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
		taskRunErr := m.pool.QueryRow(ctx, `
			SELECT 
				COUNT(CASE WHEN tr.status = 'running' THEN 1 END) as running,
				COUNT(CASE WHEN tr.status = 'success' THEN 1 END) as success,
				COUNT(CASE WHEN tr.status = 'failed' THEN 1 END) as failed,
				COUNT(CASE WHEN tr.status = 'pending' THEN 1 END) as pending
			FROM Task_Runs tr
			JOIN DAG_Runs dr ON tr.run_id = dr.run_id
			JOIN DAGs d ON dr.dag_id = d.dag_id
			WHERE d.namespace = $1
		`, namespace).Scan(&runningTasks, &successTasks, &failedTasks, &pendingTasks)

		if taskRunErr != nil {
			logger.Error(taskRunErr, "Failed to collect task run metrics", "namespace", namespace)
			metrics.RecordErrorMetrics("postgresql", "collect_content_metrics", "task_run_counts_error")
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
		metrics.UpdateContentMetrics("postgresql", namespace, dagCounts, dagRunCounts, taskRunCounts)
		log.Log.Info("Content metrics updated successfully", "namespace", namespace)
	}
}

// recordQueryMetrics is a helper to record query metrics
func (m *metricsPostgresDAGManager) recordQueryMetrics(operation, table string, start time.Time, err error) {
	duration := time.Since(start).Seconds()
	status := "success"
	if err != nil {
		status = "error"
		metrics.RecordErrorMetrics("postgresql", operation, "query_error")
	}
	metrics.RecordQueryMetrics("postgresql", operation, table, status, duration)
}

// recordTransactionMetrics is a helper to record transaction metrics
func (m *metricsPostgresDAGManager) recordTransactionMetrics(operation string, start time.Time, err error) {
	duration := time.Since(start).Seconds()
	status := "success"
	if err != nil {
		status = "error"
		metrics.RecordErrorMetrics("postgresql", operation, "transaction_error")
	}
	metrics.RecordTransactionMetrics("postgresql", operation, status, duration)
}

// Stop gracefully stops the metrics collection
func (m *metricsPostgresDAGManager) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

// Implement the interface methods with metrics wrapper

func (m *metricsPostgresDAGManager) InitaliseDatabase(ctx context.Context) error {
	start := time.Now()
	err := m.postgresDAGManager.InitaliseDatabase(ctx)
	m.recordTransactionMetrics("initialize_database", start, err)
	return err
}

func (m *metricsPostgresDAGManager) GetID(ctx context.Context) (string, error) {
	start := time.Now()
	result, err := m.postgresDAGManager.GetID(ctx)
	m.recordQueryMetrics("select", "idtable", start, err)
	return result, err
}

func (m *metricsPostgresDAGManager) GetDAGsToStartAndUpdate(ctx context.Context, tm time.Time) ([]*DagInfo, error) {
	start := time.Now()
	result, err := m.postgresDAGManager.GetDAGsToStartAndUpdate(ctx, tm)
	m.recordTransactionMetrics("get_dags_to_start_and_update", start, err)
	return result, err
}

func (m *metricsPostgresDAGManager) InsertDAG(ctx context.Context, dag *v1alpha1.DAG, namespace string) error {
	start := time.Now()
	err := m.postgresDAGManager.InsertDAG(ctx, dag, namespace)
	m.recordTransactionMetrics("insert_dag", start, err)
	return err
}

func (m *metricsPostgresDAGManager) CreateDAGRun(ctx context.Context, name string, dag *v1alpha1.DagRunSpec, parameters map[string]v1alpha1.ParameterSpec, pvcName *string) (int, error) {
	start := time.Now()
	result, err := m.postgresDAGManager.CreateDAGRun(ctx, name, dag, parameters, pvcName)
	m.recordTransactionMetrics("create_dag_run", start, err)
	return result, err
}

func (m *metricsPostgresDAGManager) GetStartingTasks(ctx context.Context, dagName string, dagrun int) ([]Task, error) {
	start := time.Now()
	result, err := m.postgresDAGManager.GetStartingTasks(ctx, dagName, dagrun)
	m.recordQueryMetrics("select", "tasks", start, err)
	return result, err
}

func (m *metricsPostgresDAGManager) MarkTaskAsStarted(ctx context.Context, runId, taskId int) (int, error) {
	start := time.Now()
	result, err := m.postgresDAGManager.MarkTaskAsStarted(ctx, runId, taskId)
	m.recordQueryMetrics("insert", "task_runs", start, err)
	return result, err
}

func (m *metricsPostgresDAGManager) IncrementAttempts(ctx context.Context, taskRunId int) error {
	start := time.Now()
	err := m.postgresDAGManager.IncrementAttempts(ctx, taskRunId)
	m.recordQueryMetrics("update", "task_runs", start, err)
	return err
}

func (m *metricsPostgresDAGManager) MarkSuccessAndGetNextTasks(ctx context.Context, taskRunId int) ([]Task, error) {
	start := time.Now()
	result, err := m.postgresDAGManager.MarkSuccessAndGetNextTasks(ctx, taskRunId)
	m.recordTransactionMetrics("mark_success_and_get_next_tasks", start, err)
	return result, err
}

func (m *metricsPostgresDAGManager) MarkDAGRunOutcome(ctx context.Context, dagRunId int, outcome string) error {
	start := time.Now()
	err := m.postgresDAGManager.MarkDAGRunOutcome(ctx, dagRunId, outcome)
	m.recordQueryMetrics("update", "dag_runs", start, err)
	return err
}

func (m *metricsPostgresDAGManager) GetDagParameters(ctx context.Context, dagName string) (map[string]*Parameter, error) {
	start := time.Now()
	result, err := m.postgresDAGManager.GetDagParameters(ctx, dagName)
	m.recordQueryMetrics("select", "dag_parameters", start, err)
	return result, err
}

func (m *metricsPostgresDAGManager) DagExists(ctx context.Context, dagName string) (bool, int, error) {
	start := time.Now()
	exists, id, err := m.postgresDAGManager.DagExists(ctx, dagName)
	m.recordQueryMetrics("select", "dags", start, err)
	return exists, id, err
}

func (m *metricsPostgresDAGManager) ShouldRerun(ctx context.Context, taskRunid int, exitCode int32) (bool, error) {
	start := time.Now()
	result, err := m.postgresDAGManager.ShouldRerun(ctx, taskRunid, exitCode)
	m.recordQueryMetrics("select", "task_runs", start, err)
	return result, err
}

func (m *metricsPostgresDAGManager) MarkTaskAsFailed(ctx context.Context, taskRunId int) error {
	start := time.Now()
	err := m.postgresDAGManager.MarkTaskAsFailed(ctx, taskRunId)
	m.recordQueryMetrics("update", "task_runs", start, err)
	return err
}

func (m *metricsPostgresDAGManager) MarkPodStatus(ctx context.Context, podUid types.UID, name string, taskRunID int, status v1.PodPhase, tStamp time.Time, exitCode *int32, namespace string) error {
	start := time.Now()
	err := m.postgresDAGManager.MarkPodStatus(ctx, podUid, name, taskRunID, status, tStamp, exitCode, namespace)
	m.recordQueryMetrics("upsert", "task_pods", start, err)
	return err
}

func (m *metricsPostgresDAGManager) DeleteDAG(ctx context.Context, name string, namespace string) ([]string, error) {
	start := time.Now()
	result, err := m.postgresDAGManager.DeleteDAG(ctx, name, namespace)
	m.recordTransactionMetrics("delete_dag", start, err)
	return result, err
}

func (m *metricsPostgresDAGManager) FindExistingDAGRun(ctx context.Context, name string) (bool, error) {
	start := time.Now()
	result, err := m.postgresDAGManager.FindExistingDAGRun(ctx, name)
	m.recordQueryMetrics("select", "dag_runs", start, err)
	return result, err
}

func (m *metricsPostgresDAGManager) GetTaskScriptAndInjectorImage(ctx context.Context, taskId int) (*string, *string, error) {
	start := time.Now()
	script, injectorImage, err := m.postgresDAGManager.GetTaskScriptAndInjectorImage(ctx, taskId)
	m.recordQueryMetrics("select", "tasks", start, err)
	return script, injectorImage, err
}

func (m *metricsPostgresDAGManager) AddTask(ctx context.Context, task *v1alpha1.DagTask, namespace string) error {
	start := time.Now()
	err := m.postgresDAGManager.AddTask(ctx, task, namespace)
	m.recordTransactionMetrics("add_task", start, err)
	return err
}

func (m *metricsPostgresDAGManager) DeleteTask(ctx context.Context, taskName string, namespace string) error {
	start := time.Now()
	err := m.postgresDAGManager.DeleteTask(ctx, taskName, namespace)
	m.recordTransactionMetrics("delete_task", start, err)
	return err
}

func (m *metricsPostgresDAGManager) GetTaskRefsParameters(ctx context.Context, taskRefs []v1alpha1.TaskRef) (map[v1alpha1.TaskRef][]string, error) {
	start := time.Now()
	result, err := m.postgresDAGManager.GetTaskRefsParameters(ctx, taskRefs)
	m.recordQueryMetrics("select", "tasks", start, err)
	return result, err
}

func (m *metricsPostgresDAGManager) GetWebhookDetails(ctx context.Context, dagRunID int) (*v1alpha1.Webhook, error) {
	start := time.Now()
	result, err := m.postgresDAGManager.GetWebhookDetails(ctx, dagRunID)
	m.recordQueryMetrics("select", "webhooks", start, err)
	return result, err
}

func (m *metricsPostgresDAGManager) GetWorkspacePVCTemplate(ctx context.Context, dagId int) (*v1alpha1.PVC, error) {
	start := time.Now()
	result, err := m.postgresDAGManager.GetWorkspacePVCTemplate(ctx, dagId)
	m.recordQueryMetrics("select", "workspace", start, err)
	return result, err
}

func (m *metricsPostgresDAGManager) CheckIfAllTasksDone(ctx context.Context, dagRunID int) (bool, error) {
	start := time.Now()
	result, err := m.postgresDAGManager.CheckIfAllTasksDone(ctx, dagRunID)
	m.recordQueryMetrics("select", "dag_runs", start, err)
	return result, err
}

func (m *metricsPostgresDAGManager) MarkConnectingTasksAsSuspended(ctx context.Context, dagRunID, taskRunId int) ([]string, error) {
	start := time.Now()
	result, err := m.postgresDAGManager.MarkConnectingTasksAsSuspended(ctx, dagRunID, taskRunId)
	m.recordTransactionMetrics("mark_connecting_tasks_suspended", start, err)
	return result, err
}

func (m *metricsPostgresDAGManager) AddPodDuration(ctx context.Context, taskRunId int, durationSec int64) error {
	start := time.Now()
	err := m.postgresDAGManager.AddPodDuration(ctx, taskRunId, durationSec)
	m.recordQueryMetrics("update", "task_pods", start, err)
	return err
}

func (m *metricsPostgresDAGManager) SuspendDagRun(ctx context.Context, dagRunId int) ([]RunningPodInfo, error) {
	start := time.Now()
	result, err := m.postgresDAGManager.SuspendDagRun(ctx, dagRunId)
	m.recordTransactionMetrics("suspend_dag_run", start, err)
	return result, err
}

func (m *metricsPostgresDAGManager) DeleteDagRun(ctx context.Context, dagRunId int) error {
	start := time.Now()
	err := m.postgresDAGManager.DeleteDagRun(ctx, dagRunId)
	m.recordTransactionMetrics("delete_dag_run", start, err)
	return err
}

func (m *metricsPostgresDAGManager) DagrunExists(ctx context.Context, dagrunId int) (bool, error) {
	start := time.Now()
	result, err := m.postgresDAGManager.DagrunExists(ctx, dagrunId)
	m.recordQueryMetrics("select", "dag_runs", start, err)
	return result, err
}

func (m *metricsPostgresDAGManager) GetTaskRunInfo(ctx context.Context, taskRunId int) (dagName, taskName, namespace string, err error) {
	start := time.Now()
	dagName, taskName, namespace, err = m.postgresDAGManager.GetTaskRunInfo(ctx, taskRunId)
	m.recordQueryMetrics("select", "task_runs", start, err)
	return dagName, taskName, namespace, err
}
