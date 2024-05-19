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
	conn   *pgxpool.Pool
	parser *cron.Parser
}

func NewPostgresManager(ctx context.Context, config *pgxpool.Config, parser *cron.Parser) (DbManager, error) {
	if config == nil {
		return nil, fmt.Errorf("missing config")
	}

	if parser == nil {
		return nil, fmt.Errorf("missing parser")
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	return &postgresManager{
		conn:   pool,
		parser: parser,
	}, nil

}

func (p *postgresManager) InitaliseDatabase(ctx context.Context) error {
	// TODO: work out size of each column
	_, err := p.conn.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS schedules (
            uid VARCHAR(255) PRIMARY KEY,
            schedule VARCHAR(255),
            imageName VARCHAR(255),
			nextTime TIMESTAMP,
			command TEXT[],
			args TEXT[]
        )
    `)

	return err
}

func (p *postgresManager) UpsertCronJob(ctx context.Context, id types.UID, schedule string, imageName string, command []string, args []string) error {
	// Parse the cron expression
	sched, err := p.parser.Parse(schedule)
	if err != nil {
		return err
	}

	// Get the next occurrence of the scheduled time
	nextTime := sched.Next(time.Now())

	// Insert or update data into the table
	_, err = p.conn.Exec(ctx, `
	INSERT INTO schedules (uid, schedule, imageName, nextTime, command, args)
	VALUES ($1, $2, $3, to_timestamp($4), $5, $6)
	ON CONFLICT (uid)
	DO UPDATE SET schedule = EXCLUDED.schedule, imageName = EXCLUDED.imageName, nextTime = EXCLUDED.nextTime, command = EXCLUDED.command, args = EXCLUDED.args
	`, id, schedule, imageName, nextTime.Unix(), command, args)

	return err
}

func (p *postgresManager) DeleteCronJob(ctx context.Context, id types.UID) error {
	if _, err := p.conn.Exec(ctx, `
        DELETE FROM schedules
        WHERE uid = $1
    `, id); err != nil {
		log.Log.Error(err, "failed to delete cronjob")
		return err
	}

	return nil
}

func (p *postgresManager) GetAllCronJobs(ctx context.Context) ([]*CronJob, error) {
	rows, err := p.conn.Query(ctx, `SELECT uid, schedule, imageName, command, args FROM schedules`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cronJobs []*CronJob
	for rows.Next() {
		var (
			id        string
			schedule  string
			imageName string
			command   []string
			args      []string
		)
		if err := rows.Scan(&id, &schedule, &imageName, &command, &args); err != nil {
			return nil, err
		}
		cronJobs = append(cronJobs, &CronJob{
			Id:        types.UID(id),
			Schedule:  schedule,
			ImageName: imageName,
			Command:   command,
			Args:      args,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return cronJobs, nil
}

func (p *postgresManager) GetCronJobsToStart(ctx context.Context) ([]CronJob, error) {
	rows, err := p.conn.Query(ctx, `
        SELECT uid, schedule, imageName, command, args
        FROM schedules
        WHERE nextTime <= NOW()
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cronJobs []CronJob
	for rows.Next() {
		var job CronJob
		if err := rows.Scan(&job.Id, &job.Schedule, &job.ImageName, &job.Command, &job.Args); err != nil {
			return nil, err
		}

		cronJobs = append(cronJobs, job)
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
	_, err = p.conn.Exec(ctx, `
	UPDATE schedules
	SET nextTime = to_timestamp($1)
	WHERE uid = $2
	`, nextTime.Unix(), uid)

	return err
}

func (p *postgresManager) Close() {
	p.conn.Close()
}
