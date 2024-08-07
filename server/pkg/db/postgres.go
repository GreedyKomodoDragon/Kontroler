package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"k8s.io/apimachinery/pkg/types"
)

type postgresManager struct {
	pool *pgxpool.Pool
}

func NewPostgresManager(ctx context.Context, config *pgxpool.Config) (DbManager, error) {
	if config == nil {
		return nil, fmt.Errorf("missing config")
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	return &postgresManager{
		pool: pool,
	}, nil

}

func (p *postgresManager) GetAllCronJobs(ctx context.Context) ([]*CronJob, error) {
	rows, err := p.pool.Query(ctx, `SELECT uid, schedule, imageName, command, args, backoffLimit, retryCodes, conditionalEnabled FROM schedules`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cronJobs []*CronJob
	for rows.Next() {
		var (
			id                 string
			schedule           string
			imageName          string
			command            []string
			args               []string
			backoffLimit       uint64
			retryCodes         []int32
			conditionalEnabled bool
		)
		if err := rows.Scan(&id, &schedule, &imageName, &command, &args, &backoffLimit, &retryCodes, &conditionalEnabled); err != nil {
			return nil, err
		}
		cronJobs = append(cronJobs, &CronJob{
			Id:        types.UID(id),
			Schedule:  schedule,
			ImageName: imageName,
			Command:   command,
			Args:      args,
			ConditionalRetry: ConditionalRetry{
				RetryCodes: retryCodes,
				Enabled:    conditionalEnabled,
			},
		})
	}

	return cronJobs, nil
}

func (p *postgresManager) GetAllRuns(ctx context.Context, limit int, offset int) ([]*Run, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT runuid, jobuid, numberofattempts, status
		FROM runs
		ORDER BY starttime DESC
		LIMIT $1 OFFSET $2
		`, limit, offset)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := []*Run{}
	for rows.Next() {
		var (
			runId            string
			jobId            string
			numberOfAttempts int64
			status           string
		)
		if err := rows.Scan(&runId, &jobId, &numberOfAttempts, &status); err != nil {
			return nil, err
		}

		runs = append(runs, &Run{
			RunId:            types.UID(runId),
			JobUid:           types.UID(jobId),
			NumberOfAttempts: numberOfAttempts,
			Status:           status,
		})
	}

	return runs, nil
}

func (p *postgresManager) GetRunsPods(ctx context.Context, runId types.UID) ([]*PodWithExitCode, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT podName, exitcode
		FROM runPods
		WHERE runUid = $1`, runId)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := []*PodWithExitCode{}
	for rows.Next() {
		var (
			name     string
			exitCode int32
		)
		if err := rows.Scan(&name, &exitCode); err != nil {
			return nil, err
		}

		runs = append(runs, &PodWithExitCode{
			Name:     name,
			ExitCode: exitCode,
		})
	}

	return runs, nil
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
