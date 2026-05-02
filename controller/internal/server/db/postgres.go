package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
)

type postgresManager struct {
	pool *pgxpool.Pool
}

func NewPostgresManager(ctx context.Context, pool *pgxpool.Pool) (DbManager, error) {
	return &postgresManager{
		pool: pool,
	}, nil
}

const ALL_DAG_METADATA_QUERY = `
SELECT dag_id, name, namespace, version, schedule, active, nexttime, suspended
FROM DAGs
WHERE active = TRUE
ORDER BY dag_id DESC
LIMIT $1 OFFSET $2;
`

func (p *postgresManager) GetAllDagMetaData(ctx context.Context, limit int, offset int) ([]*DAGMetaData, error) {
	rows, err := p.pool.Query(ctx, ALL_DAG_METADATA_QUERY, limit, offset)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	metas := []*DAGMetaData{}
	for rows.Next() {
		var meta DAGMetaData
		if err := rows.Scan(&meta.DagId, &meta.Name, &meta.Namespace, &meta.Version,
			&meta.Schedule, &meta.Active, &meta.NextTime, &meta.IsSuspended); err != nil {
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

	if err := row.Scan(&dagId); err != nil {
		return nil, err
	}

	// Get the connections
	connections, err := p.getDagConnections(ctx, dagId)
	if err != nil {
		return nil, err
	}

	rows, err := p.pool.Query(ctx, `
	SELECT
		d.dag_task_id,
		d.name,
		COALESCE(r.status, 'pending') AS status
	FROM DAG_Tasks d
	JOIN tasks t ON d.task_id = t.task_id
	LEFT JOIN Task_Runs r ON r.task_id = d.dag_task_id AND r.run_id = $1
	WHERE d.dag_id = $2;`, dagRunId, dagId)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	taskInfo := map[int]TaskInfo{}
	for rows.Next() {
		var taskId int
		task := TaskInfo{}
		if err := rows.Scan(&taskId, &task.Name, &task.Status); err != nil {
			return nil, err
		}

		taskInfo[taskId] = task
	}

	return &DagRun{
		Connections: connections,
		TaskInfo:    taskInfo,
	}, nil
}

func (p *postgresManager) GetDagRuns(ctx context.Context, limit int, offset int) ([]*DagRunMeta, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT run_id, d.dag_id, status, successfulcount, failedcount, d.namespace, r.name
		FROM DAG_Runs r
		JOIN DAGs d ON r.dag_id = d.dag_id
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
		if err := rows.Scan(&meta.Id, &meta.DagId, &meta.Status, &meta.SuccessfulCount, &meta.FailedCount, &meta.Namespace, &meta.Name); err != nil {
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
		dt.dag_task_id,
		CASE 
			WHEN array_agg(td.depends_on_task_id) = ARRAY[NULL]::INTEGER[] THEN ARRAY[]::INTEGER[]
			ELSE COALESCE(array_agg(td.depends_on_task_id), ARRAY[]::INTEGER[])
		END AS dependencies
	FROM 
		Tasks t
	LEFT JOIN 
		DAG_Tasks dt ON t.task_id = dt.task_id
	LEFT JOIN 
		Dependencies td ON dt.dag_task_id = td.task_id
	WHERE dt.dag_id = $1
	GROUP BY dt.dag_task_id;`, dagId)

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

	// Get the current status of each task - modified query to use LEFT JOIN
	rows, err := p.pool.Query(ctx, `
	SELECT
		d.dag_task_id,
		d.name,
		COALESCE(r.status, 'pending') AS status
	FROM DAG_Tasks d
	JOIN tasks t ON d.task_id = t.task_id
	LEFT JOIN Task_Runs r ON r.task_id = d.dag_task_id AND r.run_id = $1
	WHERE d.dag_id = $2;`, dagRunId, meta.DagId)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	taskInfo := map[int]TaskInfo{}
	for rows.Next() {
		var taskId int
		task := TaskInfo{}
		if err := rows.Scan(&taskId, &task.Name, &task.Status); err != nil {
			return nil, err
		}

		taskInfo[taskId] = task
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
	SELECT Pod_UID, exitCode, name, status, duration
	FROM Task_Pods
	WHERE task_run_id = $1;`, task.Id)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	task.Pods = []*TaskPod{}

	for rows.Next() {
		pod := &TaskPod{}
		var duration sql.NullInt64
		if err := rows.Scan(&pod.PodUID, &pod.ExitCode, &pod.Name, &pod.Status, &duration); err != nil {
			return nil, err
		}

		if duration.Valid {
			pod.Duration = &duration.Int64
		}

		task.Pods = append(task.Pods, pod)
	}

	return task, nil

}

func (p *postgresManager) GetTaskDetails(ctx context.Context, taskId int) (*TaskDetails, error) {
	var taskDetails TaskDetails
	var podTemplateJSON sql.NullString
	var parameters []string

	// Query for the task details from the Tasks table
	queryTask := `
		SELECT t.task_id, dat.name, t.command, t.args, t.image, t.backoffLimit, t.isConditional, t.podTemplate, t.retryCodes, t.script, t.parameters
		FROM Tasks t
		LEFT JOIN DAG_Tasks dat ON dat.task_id = t.task_id
		WHERE dat.dag_task_id = $1;
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
		&taskDetails.Script,
		&parameters,
	); err != nil {
		return nil, fmt.Errorf("failed to query task details: %w", err)
	}

	// Handle the nullable value from podTemplateJSON
	if podTemplateJSON.Valid {
		taskDetails.PodTemplate = podTemplateJSON.String
	} else {
		taskDetails.PodTemplate = "" // Or any default value
	}

	// Query for the parameters related to the task from the DAG_Parameters table
	queryParameters := `
			SELECT parameter_id, name, isSecret, defaultValue
			FROM DAG_Parameters
			WHERE dag_id in (SELECT dag_id
							 FROM DAG_Tasks
							 WHERE task_id = $1)
				  AND name = ANY($2);
		`
	rows, err := p.pool.Query(ctx, queryParameters, taskDetails.ID, parameters)
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

	// use go error groups
	eGroup := errgroup.Group{}

	eGroup.Go(func() error {
		dagCountsQuery := `
			SELECT 
				COUNT(dag_id) AS dag_count
			FROM DAGs
			WHERE active = TRUE;
		`
		return p.pool.QueryRow(ctx, dagCountsQuery).Scan(&stats.DAGCount)
	})

	eGroup.Go(func() error {
		dagRunsQuery := `
			SELECT 
				COUNT(CASE WHEN status = 'success' AND run_time >= NOW() - INTERVAL '30 days' THEN 1 END) AS successful_dag_runs,
				COUNT(CASE WHEN status = 'failed' AND run_time >= NOW() - INTERVAL '30 days' THEN 1 END) AS failed_dag_runs,
				COUNT(CASE WHEN run_time >= NOW() - INTERVAL '30 days' THEN 1 END) AS total_dag_runs,
				COUNT(CASE WHEN status = 'running' THEN 1 END) AS active_dag_runs
			FROM DAG_Runs;
		`
		return p.pool.QueryRow(ctx, dagRunsQuery).
			Scan(&stats.SuccessfulDagRuns, &stats.FailedDagRuns, &stats.TotalDagRuns, &stats.ActiveDagRuns)
	})

	// Goroutine for Task Outcomes
	eGroup.Go(func() error {
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
		if err := p.pool.QueryRow(ctx, taskOutcomesQuery).Scan(&completedTasks, &failedTasks); err != nil {
			return fmt.Errorf("failed to execute taskOutcomesQuery: %w", err)
		}

		// Set task outcomes
		stats.TaskOutcomes = map[string]int{
			"Completed": completedTasks,
			"Failed":    failedTasks,
		}

		return nil
	})

	// Goroutine for DAG Type Counts
	eGroup.Go(func() error {
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
			return err
		}
		defer rows.Close()

		stats.DAGTypeCounts = make(map[string]int)
		for rows.Next() {
			var dagType string
			var count int
			if err := rows.Scan(&dagType, &count); err != nil {
				return fmt.Errorf("failed to scan row for dagTypeCountsQuery: %w", err)
			}
			stats.DAGTypeCounts[dagType] = count
		}

		return nil
	})

	// Goroutine for Daily DAG Run Counts
	eGroup.Go(func() error {
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
			return fmt.Errorf("failed to execute dailyDagRunCountsQuery: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var dailyCount DailyDagRunCount
			if err := rows.Scan(&dailyCount.Day, &dailyCount.SuccessfulCount, &dailyCount.FailedCount); err != nil {
				return fmt.Errorf("failed to scan row for dailyDagRunCountsQuery: %w", err)
			}

			stats.DailyDagRunCounts = append(stats.DailyDagRunCounts, dailyCount)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("failed to iterate over dailyDagRunCounts rows: %w", err)
		}

		return nil
	})

	if err := eGroup.Wait(); err != nil {
		return nil, err
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
		);`

	rows, err := p.pool.Query(ctx, query, dagName)
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
			return nil, fmt.Errorf("parameter '%s' does not exist", paramName)
		}
	}

	return results, nil
}

func (p *postgresManager) GetDagTasks(ctx context.Context, limit int, offset int) ([]*DagTaskDetails, error) {
	// Query for the task details from the Tasks table
	queryTask := `
		SELECT t.task_id, t.name, t.command, t.args, t.image, t.backoffLimit, t.isConditional, t.podTemplate, t.retryCodes, t.script, t.parameters
		FROM Tasks t
		WHERE t.inline = FALSE		
		LIMIT $1 OFFSET $2;
	`
	rows, err := p.pool.Query(ctx, queryTask, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query task details in GetDagTasks: %w", err)
	}
	defer rows.Close()
	taskDetails := []*DagTaskDetails{}
	for rows.Next() {
		var taskDetail DagTaskDetails
		var podTemplateJSON sql.NullString
		if err := rows.Scan(
			&taskDetail.ID,
			&taskDetail.Name,
			&taskDetail.Command,
			&taskDetail.Args,
			&taskDetail.Image,
			&taskDetail.BackOffLimit,
			&taskDetail.IsConditional,
			&podTemplateJSON,
			&taskDetail.RetryCodes,
			&taskDetail.Script,
			&taskDetail.Parameters,
		); err != nil {
			return nil, err
		}
		// Handle the nullable value from podTemplateJSON
		if podTemplateJSON.Valid {
			taskDetail.PodTemplate = podTemplateJSON.String
		} else {
			taskDetail.PodTemplate = "" // Or any default value
		}

		taskDetails = append(taskDetails, &taskDetail)
	}

	return taskDetails, nil
}

func (p *postgresManager) GetDagTaskPageCount(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		return 0, fmt.Errorf("limit must be greater than zero")
	}

	var pageCount int

	if err := p.pool.QueryRow(ctx, `
	SELECT COUNT(*)
	FROM Tasks
	WHERE inline = FALSE;
	`).Scan(&pageCount); err != nil {
		return 0, err
	}

	pages := pageCount / limit
	if pageCount%limit > 0 {
		pages++
	}

	return pages, nil
}

func (p *postgresManager) PodExists(ctx context.Context, podUID string) (bool, error) {
	// Check if the pod exists
	var exists bool
	if err := p.pool.QueryRow(ctx, `
	SELECT EXISTS(
		SELECT 1
		FROM Task_Pods
		WHERE Pod_UID = $1
	);`, podUID).Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}

func (p *postgresManager) GetPodNameAndNamespace(ctx context.Context, podUID string) (string, string, error) {
	var namespace string
	var name string
	if err := p.pool.QueryRow(ctx, `
	SELECT namespace, name
	FROM Task_Pods
	WHERE pod_uid = $1;
	`, podUID).Scan(&namespace, &name); err != nil {
		return "", "", err
	}

	return namespace, name, nil
}
