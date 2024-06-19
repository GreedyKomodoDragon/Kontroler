package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	cron "github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/types"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

type postgresManager struct {
	pool   *pgxpool.Pool
	parser *cron.Parser
}

func NewPostgresSchedulerManager(ctx context.Context, pool *pgxpool.Pool, parser *cron.Parser) (DBSchedulerManager, error) {
	if parser == nil {
		return nil, fmt.Errorf("missing parser")
	}

	return &postgresManager{
		pool:   pool,
		parser: parser,
	}, nil

}

func (p *postgresManager) InitaliseDatabase(ctx context.Context) error {
	// TODO: work out size of each column
	fmt.Println("here:", p.pool)
	_, err := p.pool.Exec(ctx, `
		BEGIN;

        CREATE TABLE IF NOT EXISTS schedules (
            uid VARCHAR(255) PRIMARY KEY,
            schedule VARCHAR(255),
            imageName VARCHAR(255),
			nextTime TIMESTAMP,
			command TEXT[],
			args TEXT[],
			backoffLimit BIGINT,
			retryCodes INTEGER[],
			conditionalEnabled BOOL,
			namespace VARCHAR(255)
        );

		CREATE TABLE IF NOT EXISTS runs (
			runUid VARCHAR(255) PRIMARY KEY,
			jobUid VARCHAR(255),
			numberOfAttempts BIGINT,
			status VARCHAR(20),
			startTime TIMESTAMP
        );

        CREATE TABLE IF NOT EXISTS runPods (
			podName VARCHAR(255) PRIMARY KEY,
			runUid VARCHAR(255),
			exitcode INTEGER,
			startTime TIMESTAMP
        );

		COMMIT;
    `)

	fmt.Println("here 2")

	return err
}

func (p *postgresManager) UpsertCronJob(ctx context.Context, cronJob *CronJob) error {
	if cronJob == nil {
		return fmt.Errorf("missing cronjob, nil pointer")
	}

	// Parse the cron expression
	sched, err := p.parser.Parse(cronJob.Schedule)
	if err != nil {
		return err
	}

	// Get the next occurrence of the scheduled time
	nextTime := sched.Next(time.Now())

	// Insert or update data into the table
	_, err = p.pool.Exec(ctx, `
	INSERT INTO schedules (uid, schedule, imageName, nextTime, command, args, backoffLimit, retryCodes, conditionalEnabled, namespace)
	VALUES ($1, $2, $3, to_timestamp($4), $5, $6, $7, $8, $9, $10)
	ON CONFLICT (uid)
	DO UPDATE SET schedule = EXCLUDED.schedule, imageName = EXCLUDED.imageName, nextTime = EXCLUDED.nextTime,
	command = EXCLUDED.command, args = EXCLUDED.args, backoffLimit = EXCLUDED.backoffLimit, retryCodes = EXCLUDED.retryCodes,
	conditionalEnabled = EXCLUDED.conditionalEnabled, namespace = EXCLUDED.conditionalEnabled
	`, cronJob.Id, cronJob.Schedule, cronJob.ImageName, nextTime.Unix(), cronJob.Command, cronJob.Args, cronJob.BackoffLimit,
		cronJob.ConditionalRetry.RetryCodes, cronJob.ConditionalRetry.Enabled, cronJob.Namespace)

	return err
}

func (p *postgresManager) DeleteCronJob(ctx context.Context, id types.UID) error {
	if _, err := p.pool.Exec(ctx, `
        DELETE FROM schedules
        WHERE uid = $1
    `, id); err != nil {
		log.Log.Error(err, "failed to delete cronjob")
		return err
	}

	return nil
}

func (p *postgresManager) GetAllCronJobs(ctx context.Context) ([]*CronJob, error) {
	rows, err := p.pool.Query(ctx, `SELECT uid, schedule, imageName, command, args, backoffLimit, retryCodes, conditionalEnabled, namespace FROM schedules`)
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
			namespace          string
		)
		if err := rows.Scan(&id, &schedule, &imageName, &command, &args, &backoffLimit, &retryCodes, &conditionalEnabled, &namespace); err != nil {
			return nil, err
		}
		cronJobs = append(cronJobs, &CronJob{
			Id:        types.UID(id),
			Schedule:  schedule,
			ImageName: imageName,
			Command:   command,
			Args:      args,
			Namespace: namespace,
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

func (p *postgresManager) GetCronJobsToStart(ctx context.Context) ([]*CronJob, error) {
	// Maybe able to cut this down
	rows, err := p.pool.Query(ctx, `
        SELECT uid, schedule, imageName, command, args, backoffLimit, namespace
        FROM schedules
        WHERE nextTime <= NOW()
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cronJobs []*CronJob
	for rows.Next() {
		var job CronJob
		if err := rows.Scan(&job.Id, &job.Schedule, &job.ImageName, &job.Command, &job.Args, &job.BackoffLimit, &job.Namespace); err != nil {
			return nil, err
		}

		cronJobs = append(cronJobs, &job)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return cronJobs, nil
}

func (p *postgresManager) UpdateNextTime(ctx context.Context, uid types.UID, schedule string) error {
	// Parse the cron expression
	sched, err := p.parser.Parse(schedule)
	if err != nil {
		return err
	}

	// Get the next occurrence of the scheduled time
	nextTime := sched.Next(time.Now())

	// Insert or update data into the table
	_, err = p.pool.Exec(ctx, `
	UPDATE schedules
	SET nextTime = to_timestamp($1)
	WHERE uid = $2
	`, nextTime.Unix(), uid)

	return err
}

func (p *postgresManager) StartRun(ctx context.Context, jobId, runID types.UID) error {
	_, err := p.pool.Exec(ctx, `
	INSERT INTO runs (runUid, jobUid, numberOfAttempts, status, starttime)
	VALUES ($1, $2, 1, 'running', NOW());
	`, runID, jobId)

	return err
}

func (p *postgresManager) IncrementRunCount(ctx context.Context, runID types.UID) error {
	_, err := p.pool.Exec(ctx, `
	UPDATE runs
	SET numberOfAttempts = numberOfAttempts + 1
	WHERE runUid = $1;
	`, runID)

	return err
}

func (p *postgresManager) ShouldRerun(ctx context.Context, runID types.UID, exitCode int32) (bool, error) {
	// Query to check if rerun is needed based on join and conditions
	query := `
	SELECT s.backoffLimit, r.numberOfAttempts
	FROM schedules s
	INNER JOIN runs r ON s.uid = r.jobUid
	WHERE r.runUid = $1 AND r.numberOfAttempts <= s.backoffLimit AND (s.conditionalEnabled = FALSE or $2 = ANY(s.retryCodes));
    `

	rows, err := p.pool.Query(ctx, query, runID, exitCode)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	if !rows.Next() {
		// No rows returned, so rerun is not needed
		return false, nil
	}

	// At least one row returned, so rerun may be needed
	return true, nil
}

func (p *postgresManager) MarkRunOutcome(ctx context.Context, runID types.UID, status string) error {
	_, err := p.pool.Exec(ctx, `
	UPDATE runs
	SET status = $2
	WHERE runUid = $1;
	`, runID, status)

	return err
}

func (p *postgresManager) AddPodToRun(ctx context.Context, podName string, runID types.UID, exitCode int32) error {
	_, err := p.pool.Exec(ctx, `
	INSERT INTO runPods (podName, runUid, exitcode, startTime)
	VALUES ($1, $2, $3, NOW());
	`, podName, runID, exitCode)

	return err
}
