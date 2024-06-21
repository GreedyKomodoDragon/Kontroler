package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"k8s.io/apimachinery/pkg/types"
)

type postgresManager struct {
	conn *pgxpool.Pool
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
		conn: pool,
	}, nil

}

func (p *postgresManager) GetAllCronJobs(ctx context.Context) ([]*CronJob, error) {
	rows, err := p.conn.Query(ctx, `SELECT uid, schedule, imageName, command, args, backoffLimit, retryCodes, conditionalEnabled FROM schedules`)
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
	rows, err := p.conn.Query(ctx, `
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
	rows, err := p.conn.Query(ctx, `
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

// CREATE TABLE IF NOT EXISTS Tasks (
// 	task_id SERIAL PRIMARY KEY,
//     name VARCHAR(255) NOT NULL,
//     command TEXT[] NOT NULL,
//     args TEXT[] NOT NULL,
//     image VARCHAR(255) NOT NULL
// );

// CREATE TABLE IF NOT EXISTS Dependencies (
//     task_id INTEGER NOT NULL,
//     depends_on_task_id INTEGER NOT NULL,
//     FOREIGN KEY (task_id) REFERENCES Tasks(task_id),
//     FOREIGN KEY (depends_on_task_id) REFERENCES Tasks(task_id)
// );

// CREATE TABLE IF NOT EXISTS DAG_Tasks (
//     dag_id INTEGER NOT NULL,
//     task_id INTEGER NOT NULL,
//     FOREIGN KEY (dag_id) REFERENCES DAGs(dag_id),
//     FOREIGN KEY (task_id) REFERENCES Tasks(task_id)
// );

// CREATE TABLE IF NOT EXISTS DAG_Runs (
// 	run_id SERIAL PRIMARY KEY,
//     dag_id INTEGER NOT NULL,
// 	status VARCHAR(255) NOT NULL,
//     FOREIGN KEY (dag_id) REFERENCES DAGs(dag_id)
// );

// CREATE TABLE IF NOT EXISTS Task_Runs (
// 	task_run_id SERIAL PRIMARY KEY,
// 	run_id INTEGER NOT NULL,
//     task_id INTEGER NOT NULL,
// 	status VARCHAR(255) NOT NULL,
//     FOREIGN KEY (task_id) REFERENCES Tasks(task_id),
// 	FOREIGN KEY (run_id) REFERENCES DAG_Runs(run_id)
// );

func (p *postgresManager) GetAllDagMetaData(ctx context.Context, limit int, offset int) ([]*DAGMetaData, error) {
	rows, err := p.conn.Query(ctx, `
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

		metas = append(metas, &meta)
	}

	return metas, nil
}

func (p *postgresManager) Close() {
	p.conn.Close()
}
