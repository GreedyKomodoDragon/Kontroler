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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return cronJobs, nil
}

func (p *postgresManager) GetAllRuns(ctx context.Context) ([]*Run, error) {
	rows, err := p.conn.Query(ctx, `SELECT runuid, jobuid, numberofattempts, status FROM runs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []*Run
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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return runs, nil
}

func (p *postgresManager) Close() {
	p.conn.Close()
}
