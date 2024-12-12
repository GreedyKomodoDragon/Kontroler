package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type sqliteManager struct {
	db *sql.DB
}

// SQLiteReadOnlyConfig holds the configurable for ReadOnly SQLite settings
type SQLiteReadOnlyConfig struct {
	DBPath      string
	Synchronous string // e.g., "NORMAL" or "FULL"
	CacheSize   int    // e.g., -2000 (for KB, negative to use memory size in KB)
}

// Creates Read Only SQLite Connection
func NewSQLiteReadOnlyManager(ctx context.Context, config *SQLiteReadOnlyConfig) (DbManager, error) {
	// Open a read-only connection to the SQLite database file.
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s", config.DBPath))
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database in read-only mode at '%s': %w", config.DBPath, err)
	}

	// Optional performance settings for read-only access
	if config.CacheSize != 0 {
		if _, err := db.Exec(fmt.Sprintf("PRAGMA cache_size=%d;", config.CacheSize)); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set cache size: %w", err)
		}
	}

	// Check the connection to ensure the database is accessible.
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to SQLite database at '%s': %w", config.DBPath, err)
	}

	return &sqliteManager{
		db: db,
	}, nil
}

func (s *sqliteManager) GetAllDagMetaData(ctx context.Context, limit int, offset int) ([]*DAGMetaData, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT dag_id, name, version, schedule, active, nexttime
		FROM DAGs
		WHERE active = TRUE
		ORDER BY dag_id DESC
		LIMIT ? OFFSET ?`, limit, offset)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	metas := []*DAGMetaData{}
	for rows.Next() {
		var meta DAGMetaData
		if err := rows.Scan(&meta.DagId, &meta.Name, &meta.Version, &meta.Schedule, &meta.Active, &meta.NextTime); err != nil {
			return nil, err
		}

		meta.Connections, err = s.getDagConnections(ctx, meta.DagId)
		if err != nil {
			return nil, err
		}
		metas = append(metas, &meta)
	}

	return metas, nil
}

func (s *sqliteManager) GetDagRun(ctx context.Context, dagRunId int) (*DagRun, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var dagId int
	row := tx.QueryRowContext(ctx, `
		SELECT dag_id
		FROM DAG_Runs
		WHERE run_id = ?`, dagRunId)

	if err := row.Scan(&dagId); err != nil {
		return nil, err
	}

	connections, err := s.getDagConnections(ctx, dagId)
	if err != nil {
		return nil, err
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT task_id, status
		FROM Task_Runs
		WHERE run_id = ?`, dagRunId)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	taskInfo := map[int]TaskInfo{}
	for rows.Next() {
		var taskId int
		taskStatus := TaskInfo{}
		if err := rows.Scan(&taskId, &taskStatus.Status); err != nil {
			return nil, err
		}
		taskInfo[taskId] = taskStatus
	}

	for key := range connections {
		if _, ok := taskInfo[key]; !ok {
			taskInfo[key] = TaskInfo{Status: "pending"}
		}
	}

	return &DagRun{
		Connections: connections,
		TaskInfo:    taskInfo,
	}, nil
}

func (s *sqliteManager) GetDagRuns(ctx context.Context, limit int, offset int) ([]*DagRunMeta, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT run_id, dag_id, status, successfulcount, failedcount
		FROM dag_runs
		ORDER BY run_id DESC
		LIMIT ? OFFSET ?`, limit, offset)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	metas := []*DagRunMeta{}
	for rows.Next() {
		var meta DagRunMeta
		if err := rows.Scan(&meta.Id, &meta.DagId, &meta.Status, &meta.SuccessfulCount, &meta.FailedCount); err != nil {
			return nil, err
		}
		metas = append(metas, &meta)
	}

	return metas, nil
}

func (s *sqliteManager) Close() {
	s.db.Close()
}

func (s *sqliteManager) getDagConnections(ctx context.Context, dagId int) (map[int][]int, error) {
	// Query for the task connections
	rows, err := s.db.QueryContext(ctx, `
		SELECT 
			dt.dag_task_id,
			COALESCE(GROUP_CONCAT(td.depends_on_task_id), '') AS dependencies
		FROM 
			Tasks t
		LEFT JOIN 
		DAG_Tasks dt ON t.task_id = dt.task_id
		LEFT JOIN 
			Dependencies td ON dt.dag_task_id = td.task_id
		WHERE dt.dag_id = ?
		GROUP BY dt.dag_task_id;
	`, dagId)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	connections := map[int][]int{}
	for rows.Next() {
		var taskId int
		var depsString string

		// Scan the task_id and dependencies (as a comma-separated string)
		if err := rows.Scan(&taskId, &depsString); err != nil {
			return nil, err
		}

		// Parse the comma-separated string into an int slice
		var taskDeps []int
		if depsString == "" {
			connections[taskId] = []int{}
			continue
		}

		for _, dep := range strings.Split(depsString, ",") {
			depInt, err := strconv.Atoi(dep)
			if err != nil {
				return nil, fmt.Errorf("failed to parse dependency '%s' for task %d: %w", dep, taskId, err)
			}
			taskDeps = append(taskDeps, depInt)
		}

		connections[taskId] = taskDeps
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating task connections: %w", rows.Err())
	}

	return connections, nil
}

func (s *sqliteManager) GetDagRunAll(ctx context.Context, dagRunId int) (*DagRunAll, error) {
	meta := &DagRunAll{Id: dagRunId}

	if err := s.db.QueryRowContext(ctx, `
		SELECT dag_id, status, successfulCount, failedCount
		FROM DAG_Runs
		WHERE run_id = ?`, dagRunId).Scan(&meta.DagId, &meta.Status, &meta.SuccessfulCount, &meta.FailedCount); err != nil {
		return nil, err
	}

	connections, err := s.getDagConnections(ctx, meta.DagId)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT task_id, status
		FROM Task_Runs
		WHERE run_id = ?`, dagRunId)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	taskInfo := map[int]TaskInfo{}
	for rows.Next() {
		var taskId int
		taskStatus := TaskInfo{}
		if err := rows.Scan(&taskId, &taskStatus.Status); err != nil {
			return nil, err
		}
		taskInfo[taskId] = taskStatus
	}

	for key := range connections {
		if _, ok := taskInfo[key]; !ok {
			taskInfo[key] = TaskInfo{Status: "pending"}
		}
	}

	meta.Connections = connections
	meta.TaskInfo = taskInfo

	return meta, nil
}

func (s *sqliteManager) GetDagNames(ctx context.Context, term string, limit int) ([]*string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT name
		FROM DAGs
		WHERE name LIKE ?
		LIMIT ?`, "%"+term+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := []*string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, &name)
	}
	return names, nil
}

func (s *sqliteManager) GetDagPageCount(ctx context.Context, limit int) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM DAGs`).Scan(&count); err != nil {
		return 0, err
	}
	return (count + limit - 1) / limit, nil
}

func (s *sqliteManager) GetDagParameters(ctx context.Context, dagName string) ([]*Parameter, error) {
	rows, err := s.db.QueryContext(ctx, `
	SELECT parameter_id, name, isSecret, defaultValue
	FROM DAG_Parameters
	WHERE dag_id IN (
		SELECT dag_id
		FROM DAGs
		WHERE name = ?
		ORDER BY version DESC
		LIMIT 1
  	);`, dagName)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	params := []*Parameter{}

	for rows.Next() {
		var param Parameter
		if err := rows.Scan(&param.ID, &param.Name, &param.IsSecret, &param.DefaultValue); err != nil {
			return nil, err
		}
		params = append(params, &param)
	}

	return params, nil
}

func (s *sqliteManager) GetDagRunPageCount(ctx context.Context, limit int) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM DAG_Runs`).Scan(&count); err != nil {
		return 0, err
	}

	pages := count / limit
	if count%limit > 0 {
		pages++
	}

	return pages, nil
}

func (s *sqliteManager) GetDashboardStats(ctx context.Context) (*DashboardStats, error) {
	stats := DashboardStats{}
	errChan := make(chan error, 5)
	var wg sync.WaitGroup

	wg.Add(5)
	// Goroutine for DAG Count and DAG Type Counts
	go func() {
		defer wg.Done()
		dagCountsQuery := `
			SELECT 
				COUNT(dag_id) AS dag_count
			FROM DAGs
			WHERE active = 1;
		`
		row := s.db.QueryRowContext(ctx, dagCountsQuery)
		if err := row.Scan(&stats.DAGCount); err != nil {
			errChan <- fmt.Errorf("failed to execute dagCountsQuery: %w", err)
			return
		}
	}()

	// Goroutine for DAG Runs Stats
	go func() {
		defer wg.Done()
		dagRunsQuery := `
			SELECT 
				SUM(CASE WHEN status = 'success' AND run_time >= datetime('now', '-30 days') THEN 1 ELSE 0 END) AS successful_dag_runs,
				SUM(CASE WHEN status = 'failed' AND run_time >= datetime('now', '-30 days') THEN 1 ELSE 0 END) AS failed_dag_runs,
				SUM(CASE WHEN run_time >= datetime('now', '-30 days') THEN 1 ELSE 0 END) AS total_dag_runs,
				SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END) AS active_dag_runs
			FROM DAG_Runs;
		`
		var successfulDagRuns, failedDagRuns, totalDagRuns, activeDagRuns sql.NullInt64
		row := s.db.QueryRowContext(ctx, dagRunsQuery)
		if err := row.Scan(&successfulDagRuns, &failedDagRuns, &totalDagRuns, &activeDagRuns); err != nil {
			errChan <- fmt.Errorf("failed to execute dagRunsQuery: %w", err)
			return
		}

		if successfulDagRuns.Valid {
			stats.SuccessfulDagRuns = int(successfulDagRuns.Int64)
		} else {
			stats.SuccessfulDagRuns = 0
		}

		if failedDagRuns.Valid {
			stats.FailedDagRuns = int(failedDagRuns.Int64)
		} else {
			stats.FailedDagRuns = 0
		}

		if totalDagRuns.Valid {
			stats.TotalDagRuns = int(totalDagRuns.Int64)
		} else {
			stats.TotalDagRuns = 0
		}

		if activeDagRuns.Valid {
			stats.ActiveDagRuns = int(activeDagRuns.Int64)
		} else {
			stats.ActiveDagRuns = 0
		}

	}()

	// Goroutine for Task Outcomes
	go func() {
		defer wg.Done()
		taskOutcomesQuery := `
			SELECT 
				SUM(CASE WHEN tr.status = 'success' THEN 1 ELSE 0 END) AS completed_tasks,
				SUM(CASE WHEN tr.status = 'failed' THEN 1 ELSE 0 END) AS failed_tasks
			FROM Task_Runs tr
			JOIN (
				SELECT task_run_id, MAX(updated_at) AS max_updated_at
				FROM Task_Pods
				GROUP BY task_run_id
			) tp ON tr.task_run_id = tp.task_run_id
			WHERE tp.max_updated_at >= datetime('now', '-30 days');
		`
		var completedTasks, failedTasks sql.NullInt64
		row := s.db.QueryRowContext(ctx, taskOutcomesQuery)
		if err := row.Scan(&completedTasks, &failedTasks); err != nil {
			errChan <- fmt.Errorf("failed to execute taskOutcomesQuery: %w", err)
			return
		}

		stats.TaskOutcomes = map[string]int{}

		if completedTasks.Valid {
			stats.TaskOutcomes["Completed"] = int(completedTasks.Int64)
		} else {
			stats.TaskOutcomes["Completed"] = 0
		}

		if failedTasks.Valid {
			stats.TaskOutcomes["Failed"] = int(failedTasks.Int64)
		} else {
			stats.TaskOutcomes["Failed"] = 0
		}
	}()

	// Goroutine for DAG Type Counts
	go func() {
		defer wg.Done()
		dagTypeCountsQuery := `
			SELECT 
				CASE 
					WHEN schedule = '' THEN 'Event Driven'
					ELSE 'Scheduled'
				END AS dag_type, 
				COUNT(*) AS count
			FROM DAGs
			GROUP BY dag_type;
		`
		rows, err := s.db.QueryContext(ctx, dagTypeCountsQuery)
		if err != nil {
			errChan <- fmt.Errorf("failed to execute dagTypeCountsQuery: %w", err)
			return
		}
		defer rows.Close()

		stats.DAGTypeCounts = make(map[string]int)
		for rows.Next() {
			var dagType string
			var count int
			if err := rows.Scan(&dagType, &count); err != nil {
				errChan <- fmt.Errorf("failed to scan row for dagTypeCountsQuery: %w", err)
				return
			}
			stats.DAGTypeCounts[dagType] = count
		}
	}()

	// Goroutine for Daily DAG Run Counts
	go func() {
		defer wg.Done()
		dailyDagRunCountsQuery := `
			SELECT
				strftime('%Y-%m-%d', run_time) AS day,
				SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) AS successful_count,
				SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) AS failed_count
			FROM DAG_Runs
			WHERE run_time >= datetime('now', '-30 days')
			GROUP BY day
			ORDER BY day;
		`
		rows, err := s.db.QueryContext(ctx, dailyDagRunCountsQuery)
		if err != nil {
			errChan <- fmt.Errorf("failed to execute dailyDagRunCountsQuery: %w", err)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var dayStr string
			var successfulCount, failedCount int
			if err := rows.Scan(&dayStr, &successfulCount, &failedCount); err != nil {
				errChan <- fmt.Errorf("failed to scan row for dailyDagRunCountsQuery: %w", err)
				return
			}

			// Parse dayStr to time.Time
			day, err := time.Parse("2006-01-02", dayStr)
			if err != nil {
				errChan <- fmt.Errorf("failed to parse day string: %w", err)
				return
			}

			stats.DailyDagRunCounts = append(stats.DailyDagRunCounts, DailyDagRunCount{
				Day:             day,
				SuccessfulCount: successfulCount,
				FailedCount:     failedCount,
			})
		}

		if err := rows.Err(); err != nil {
			errChan <- fmt.Errorf("failed to iterate over dailyDagRunCounts rows: %w", err)
		}
	}()

	// Wait for all goroutines to finish
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Check for errors
	for err := range errChan {
		if err != nil {
			return nil, err
		}
	}

	return &stats, nil
}

func (s *sqliteManager) GetIsSecrets(ctx context.Context, dagName string, parameterNames []string) (map[string]bool, error) {
	placeholders := strings.Join(make([]string, len(parameterNames)), "?")

	// Final query with placeholders
	query := fmt.Sprintf(`
		SELECT name, isSecret 
		FROM DAG_Parameters 
		WHERE dag_id = (
			SELECT dag_id
			FROM DAGs
			WHERE name = ?
			ORDER BY version DESC
			LIMIT 1
		) AND name IN (%s)`, placeholders)

	args := make([]interface{}, 0, len(parameterNames)+1)
	args = append(args, dagName)
	for _, name := range parameterNames {
		args = append(args, name)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make(map[string]bool)

	for rows.Next() {
		var name string
		var isSecret bool
		if err := rows.Scan(&name, &isSecret); err != nil {
			return nil, err
		}
		results[name] = isSecret
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	for _, paramName := range parameterNames {
		if _, exists := results[paramName]; !exists {
			return nil, errors.New(fmt.Sprintf("parameter '%s' does not exist", paramName))
		}
	}

	return results, nil
}

func (s *sqliteManager) GetTaskDetails(ctx context.Context, taskId int) (*TaskDetails, error) {
	var taskDetails TaskDetails
	var podTemplateJSON *string
	var commandJSON string
	var argsJSON string
	var retryJSON string
	var paramsJson string

	// Query for the task details from the Tasks table
	queryTask := `
			SELECT t.task_id, dat.name, t.command, t.args, t.image, t.backoffLimit, t.isConditional, t.podTemplate, t.retryCodes, t.script, t.parameters
			FROM Tasks t
			LEFT JOIN DAG_Tasks dat ON dat.task_id = t.task_id
			WHERE dat.dag_task_id = ?;
		`

	if err := s.db.QueryRowContext(ctx, queryTask, taskId).Scan(
		&taskDetails.ID,
		&taskDetails.Name,
		&commandJSON,
		&argsJSON,
		&taskDetails.Image,
		&taskDetails.BackOffLimit,
		&taskDetails.IsConditional,
		&podTemplateJSON,
		&retryJSON,
		&taskDetails.Script,
		&paramsJson,
	); err != nil {
		return nil, fmt.Errorf("failed to query task details: %w", err)
	}

	if err := json.Unmarshal([]byte(commandJSON), &taskDetails.Command); err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(retryJSON), &taskDetails.RetryCodes); err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(argsJSON), &taskDetails.Args); err != nil {
		return nil, err
	}

	if podTemplateJSON != nil {
		taskDetails.PodTemplate = string(*podTemplateJSON)
	} else {
		taskDetails.PodTemplate = ""
	}

	params := []string{}
	if err := json.Unmarshal([]byte(paramsJson), &params); err != nil {
		return nil, err
	}

	placeholders := generateQuestionMarks(params)

	queryParameters := fmt.Sprintf(`
		SELECT parameter_id, name, isSecret, defaultValue
		FROM DAG_Parameters
		WHERE dag_id = (
			SELECT dag_id
			FROM DAG_Tasks
			WHERE task_id = ?
		) AND name IN (%s)
	`, placeholders)

	args := make([]interface{}, len(params)+1)
	args[0] = taskDetails.ID
	for i, param := range params {
		args[i+1] = param
	}

	rows, err := s.db.QueryContext(ctx, queryParameters, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query parameters: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var param Parameter
		if err := rows.Scan(&param.ID, &param.Name, &param.IsSecret, &param.DefaultValue); err != nil {
			return nil, fmt.Errorf("failed to scan parameter row: %w", err)
		}
		taskDetails.Parameters = append(taskDetails.Parameters, param)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating parameter rows: %w", rows.Err())
	}

	return &taskDetails, nil
}

// GetTaskRunDetails implements DbManager.
func (s *sqliteManager) GetTaskRunDetails(ctx context.Context, dagRunId int, taskId int) (*TaskRunDetails, error) {
	task := &TaskRunDetails{}

	if err := s.db.QueryRowContext(ctx, `
	SELECT task_run_id, status, attempts
	FROM Task_Runs
	WHERE run_id = ? AND task_id = ?;
	`, dagRunId, taskId).Scan(&task.Id, &task.Status, &task.Attempts); err != nil {
		return nil, err
	}

	// Get the current status of each task
	rows, err := s.db.QueryContext(ctx, `
	SELECT Pod_UID, exitCode, name, status
	FROM Task_Pods
	WHERE task_run_id = ?;`, task.Id)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	task.Pods = []*TaskPod{}

	for rows.Next() {
		pod := &TaskPod{}
		if err := rows.Scan(&pod.PodUID, &pod.ExitCode, &pod.Name, &pod.Status); err != nil {
			return nil, err
		}
		task.Pods = append(task.Pods, pod)
	}

	return task, nil
}

func (s *sqliteManager) GetDagTasks(ctx context.Context, limit int, offset int) ([]*DagTaskDetails, error) {
	// Query for the task details from the Tasks table
	queryTask := `
		SELECT t.task_id, t.name, t.command, t.args, t.image, t.backoffLimit, t.isConditional, t.podTemplate, t.retryCodes, t.script, t.parameters
		FROM Tasks t
		WHERE t.inline = FALSE		
		LIMIT ? OFFSET ?;
	`

	rows, err := s.db.QueryContext(ctx, queryTask, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query task details in GetDagTasks: %w", err)
	}

	defer rows.Close()

	taskDetails := []*DagTaskDetails{}
	for rows.Next() {
		var taskDetail DagTaskDetails
		var podTemplateJSON *string
		var commandJSON string
		var argsJSON string
		var retryJSON string
		var paramsJson string

		if err := rows.Scan(
			&taskDetail.ID,
			&taskDetail.Name,
			&commandJSON,
			&argsJSON,
			&taskDetail.Image,
			&taskDetail.BackOffLimit,
			&taskDetail.IsConditional,
			&podTemplateJSON,
			&retryJSON,
			&taskDetail.Script,
			&paramsJson,
		); err != nil {
			return nil, fmt.Errorf("failed to query task details: %w", err)
		}

		if err := json.Unmarshal([]byte(commandJSON), &taskDetail.Command); err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(retryJSON), &taskDetail.RetryCodes); err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(argsJSON), &taskDetail.Args); err != nil {
			return nil, err
		}

		if podTemplateJSON != nil {
			taskDetail.PodTemplate = string(*podTemplateJSON)
		} else {
			taskDetail.PodTemplate = ""
		}

		params := []string{}
		if err := json.Unmarshal([]byte(paramsJson), &params); err != nil {
			return nil, err
		}

		taskDetail.Parameters = params
		taskDetails = append(taskDetails, &taskDetail)
	}

	return taskDetails, nil
}

func (s *sqliteManager) GetDagTaskPageCount(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		return 0, fmt.Errorf("limit must be greater than zero")
	}

	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM Tasks
		WHERE inline = FALSE;`).Scan(&count); err != nil {
		return 0, err
	}

	pages := count / limit
	if count%limit > 0 {
		pages++
	}

	return pages, nil
}

func generateQuestionMarks(slice []string) string {
	length := len(slice)

	if length == 0 {
		return ""
	}

	questionMarks := make([]string, length)

	for i := range questionMarks {
		questionMarks[i] = "?"
	}

	return strings.Join(questionMarks, ", ")
}
