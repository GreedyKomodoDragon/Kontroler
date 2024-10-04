package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresManager struct {
	pool *pgxpool.Pool
}

func NewPostgresManager(ctx context.Context, pool *pgxpool.Pool) (DbManager, error) {
	return &postgresManager{
		pool: pool,
	}, nil

}

func (p *postgresManager) GetAllDagMetaData(ctx context.Context, limit int, offset int) ([]*DAGMetaData, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT dag_id, name, version, schedule, active, nexttime
		FROM DAGs
		ORDER BY dag_id DESC
		LIMIT $1 OFFSET $2
		`, limit, offset)

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

		// Get the connections
		meta.Connections, err = p.getDagConnections(ctx, meta.DagId)
		if err != nil {
			return nil, err
		}
		metas = append(metas, &meta)
	}

	return metas, nil
}

func (p *postgresManager) GetDagRun(ctx context.Context, dagRunId int) (*DagRun, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}

	defer tx.Rollback(ctx)

	var dagId int
	row := tx.QueryRow(ctx, `
	SELECT dag_id
	FROM DAG_Runs
	WHERE run_id = $1`, dagRunId)

	if err != nil {
		return nil, err
	}

	if err := row.Scan(&dagId); err != nil {
		return nil, err
	}

	// Get the connections
	connections, err := p.getDagConnections(ctx, dagId)
	if err != nil {
		return nil, err
	}

	// Get the current status of each task
	rows, err := tx.Query(ctx, `
	SELECT task_id, status
	FROM Task_Runs
	WHERE run_id = $1;`, dagRunId)

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

	// fill in the blanks - avoids having to do complex queries to get the missing values
	for key := range connections {
		if _, ok := taskInfo[key]; !ok {
			taskInfo[key] = TaskInfo{
				Status: "pending",
			}
		}
	}

	return &DagRun{
		Connections: connections,
		TaskInfo:    taskInfo,
	}, nil
}

func (p *postgresManager) GetDagRuns(ctx context.Context, limit int, offset int) ([]*DagRunMeta, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT run_id, dag_id, status, successfulcount, failedcount
		FROM dag_runs
		ORDER BY run_id DESC
		LIMIT $1 OFFSET $2
		`, limit, offset)

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

func (p *postgresManager) Close() {
	p.pool.Close()
}

func (p *postgresManager) getDagConnections(ctx context.Context, dagId int) (map[int][]int, error) {
	// Get the connections
	rows, err := p.pool.Query(ctx, `
	SELECT 
		t.task_id, 
		CASE 
			WHEN array_agg(td.depends_on_task_id) = ARRAY[NULL]::INTEGER[] THEN ARRAY[]::INTEGER[]
			ELSE COALESCE(array_agg(td.depends_on_task_id), ARRAY[]::INTEGER[])
    	END AS dependencies
	FROM 
		Tasks t
	LEFT JOIN 
		Dependencies td ON t.task_id = td.task_id
	WHERE t.task_id in (
		SELECT task_id
		FROM DAG_Tasks
		WHERE dag_id = $1
	)
	GROUP BY 
		t.task_id;`, dagId)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	connections := map[int][]int{}
	for rows.Next() {
		var taskId int
		var taskDeps []int
		if err := rows.Scan(&taskId, &taskDeps); err != nil {
			return nil, err
		}

		connections[taskId] = taskDeps
	}

	return connections, nil
}

func (p *postgresManager) GetDagRunAll(ctx context.Context, dagRunId int) (*DagRunAll, error) {
	meta := &DagRunAll{
		Id: dagRunId,
	}

	if err := p.pool.QueryRow(ctx, `
	SELECT dag_id, status, successfulCount, failedCount
	FROM DAG_Runs
	WHERE run_id = $1;
	`, dagRunId).Scan(&meta.DagId, &meta.Status, &meta.SuccessfulCount, &meta.FailedCount); err != nil {
		return nil, err
	}

	// Get the connections
	connections, err := p.getDagConnections(ctx, meta.DagId)
	if err != nil {
		return nil, err
	}

	// Get the current status of each task
	rows, err := p.pool.Query(ctx, `
	SELECT task_id, status
	FROM Task_Runs
	WHERE run_id = $1;`, dagRunId)

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
			taskInfo[key] = TaskInfo{
				Status: "pending",
			}
		}
	}

	meta.Connections = connections
	meta.TaskInfo = taskInfo

	return meta, nil
}

func (p *postgresManager) GetTaskRunDetails(ctx context.Context, dagRunId, taskId int) (*TaskRunDetails, error) {
	task := &TaskRunDetails{}

	if err := p.pool.QueryRow(ctx, `
	SELECT task_run_id, status, attempts
	FROM Task_Runs
	WHERE run_id = $1 AND task_id = $2;
	`, dagRunId, taskId).Scan(&task.Id, &task.Status, &task.Attempts); err != nil {
		return nil, err
	}

	// Get the current status of each task
	rows, err := p.pool.Query(ctx, `
	SELECT Pod_UID, exitCode, name, status
	FROM Task_Pods
	WHERE task_run_id = $1;`, task.Id)

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

func (p *postgresManager) GetTaskDetails(ctx context.Context, taskId int) (*TaskDetails, error) {
	var taskDetails TaskDetails
	var podTemplateJSON json.RawMessage

	// Query for the task details from the Tasks table
	queryTask := `
			SELECT task_id, name, command, args, image, backoffLimit, isConditional, podTemplate, retryCodes
			FROM Tasks
			WHERE task_id = $1
		`

	if err := p.pool.QueryRow(ctx, queryTask, taskId).Scan(
		&taskDetails.ID,
		&taskDetails.Name,
		&taskDetails.Command,
		&taskDetails.Args,
		&taskDetails.Image,
		&taskDetails.BackOffLimit,
		&taskDetails.IsConditional,
		&podTemplateJSON,
		&taskDetails.RetryCodes,
	); err != nil {
		return nil, fmt.Errorf("failed to query task details: %w", err)
	}

	// Convert JSONB field to string
	taskDetails.PodTemplate = string(podTemplateJSON)

	// Query for the parameters related to the task from the DAG_Parameters table
	queryParameters := `
			SELECT parameter_id, name, isSecret, defaultValue
			FROM DAG_Parameters
			WHERE dag_id = $1
		`

	rows, err := p.pool.Query(ctx, queryParameters, taskDetails.ID)
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

func (p *postgresManager) GetDashboardStats(ctx context.Context) (*DashboardStats, error) {
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
			WHERE active = TRUE;
		`
		row := p.pool.QueryRow(ctx, dagCountsQuery)
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
				COUNT(CASE WHEN status = 'success' AND run_time >= NOW() - INTERVAL '30 days' THEN 1 END) AS successful_dag_runs,
				COUNT(CASE WHEN status = 'failed' AND run_time >= NOW() - INTERVAL '30 days' THEN 1 END) AS failed_dag_runs,
				COUNT(CASE WHEN run_time >= NOW() - INTERVAL '30 days' THEN 1 END) AS total_dag_runs,
				COUNT(CASE WHEN status = 'running' THEN 1 END) AS active_dag_runs
			FROM DAG_Runs;
		`
		row := p.pool.QueryRow(ctx, dagRunsQuery)
		if err := row.Scan(&stats.SuccessfulDagRuns, &stats.FailedDagRuns, &stats.TotalDagRuns, &stats.ActiveDagRuns); err != nil {
			errChan <- fmt.Errorf("failed to execute dagRunsQuery: %w", err)
			return
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
			WHERE tp.max_updated_at >= NOW() - INTERVAL '30 days';
		`
		var completedTasks, failedTasks int
		row := p.pool.QueryRow(ctx, taskOutcomesQuery)
		if err := row.Scan(&completedTasks, &failedTasks); err != nil {
			errChan <- fmt.Errorf("failed to execute taskOutcomesQuery: %w", err)
			return
		}

		// Set task outcomes
		stats.TaskOutcomes = map[string]int{
			"Completed": completedTasks,
			"Failed":    failedTasks,
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
		rows, err := p.pool.Query(ctx, dagTypeCountsQuery)
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
				date_trunc('day', run_time) AS day,
				COUNT(*) FILTER (WHERE status = 'success') AS successful_count,
				COUNT(*) FILTER (WHERE status = 'failed') AS failed_count
			FROM DAG_Runs
			WHERE run_time >= NOW() - INTERVAL '30 days'
			GROUP BY day
			ORDER BY day;
		`
		rows, err := p.pool.Query(ctx, dailyDagRunCountsQuery)
		if err != nil {
			errChan <- fmt.Errorf("failed to execute dailyDagRunCountsQuery: %w", err)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var dailyCount DailyDagRunCount
			if err := rows.Scan(&dailyCount.Day, &dailyCount.SuccessfulCount, &dailyCount.FailedCount); err != nil {
				errChan <- fmt.Errorf("failed to scan row for dailyDagRunCountsQuery: %w", err)
				return
			}
			stats.DailyDagRunCounts = append(stats.DailyDagRunCounts, dailyCount)
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

func (p *postgresManager) GetDagRunPageCount(ctx context.Context, limit int) (int, error) {
	var pageCount int

	if err := p.pool.QueryRow(ctx, `
	SELECT COUNT(*)
	FROM DAG_Runs;
	`).Scan(&pageCount); err != nil {
		return 0, err
	}

	pages := pageCount / limit
	if pageCount%limit > 0 {
		pages++
	}

	return pages, nil
}

func (p *postgresManager) GetDagPageCount(ctx context.Context, limit int) (int, error) {
	var pageCount int

	if err := p.pool.QueryRow(ctx, `
	SELECT COUNT(*)
	FROM DAGs;
	`).Scan(&pageCount); err != nil {
		return 0, err
	}

	pages := pageCount / limit
	if pageCount%limit > 0 {
		pages++
	}

	return pages, nil
}

func (p *postgresManager) GetDagNames(ctx context.Context, term string, limit int) ([]*string, error) {
	rows, err := p.pool.Query(ctx, `
	SELECT name
	FROM DAGs
	WHERE name ILIKE $1
	LIMIT $2;`, "%"+term+"%", limit)

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

func (p *postgresManager) GetDagParameters(ctx context.Context, dagName string) ([]*Parameter, error) {
	rows, err := p.pool.Query(ctx, `
	SELECT parameter_id, name, isSecret, defaultValue
	FROM DAG_Parameters
	WHERE dag_id IN (
		SELECT dag_id
		FROM DAGs
		WHERE name = $1
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

func (p *postgresManager) GetIsSecrets(ctx context.Context, dagName string, parameterNames []string) (map[string]bool, error) {
	query := `
		SELECT name, isSecret 
		FROM DAG_Parameters 
		WHERE dag_id IN (
			SELECT dag_id
			FROM DAGs
			WHERE name = $1
			ORDER BY version DESC
			LIMIT 1
		) AND name = ANY($2)`

	rows, err := p.pool.Query(ctx, query, dagName, parameterNames)
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
