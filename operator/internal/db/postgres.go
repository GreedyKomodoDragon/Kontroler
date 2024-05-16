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

func (p *postgresManager) InitaliseDatabase(ctx context.Context) error {
	// TODO: work out size of each column
	_, err := p.conn.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS schedules (
            uid VARCHAR(255) PRIMARY KEY,
            schedule VARCHAR(255),
            imageName VARCHAR(255)
        )
    `)

	return err
}

func (p *postgresManager) UpsertCronJob(ctx context.Context, id types.UID, schedule string, imageName string) error {
	// Insert or update data into the table
	_, err := p.conn.Exec(ctx, `
        INSERT INTO schedules (uid, schedule, imageName)
        VALUES ($1, $2, $3)
        ON CONFLICT (uid)
        DO UPDATE SET schedule = EXCLUDED.schedule, imageName = EXCLUDED.imageName
    `, id, schedule, imageName)

	return err
}

func (p *postgresManager) DeleteCronJob(ctx context.Context, id types.UID) error {
	_, err := p.conn.Exec(ctx, `
        DELETE FROM schedules
        WHERE uid = $1
    `, id)

	return err
}

func (p *postgresManager) GetAllCronJobs(ctx context.Context) ([]*CronJob, error) {
	rows, err := p.conn.Query(ctx, `SELECT uid, schedule, imageName FROM schedules`)
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
		)
		if err := rows.Scan(&id, &schedule, &imageName); err != nil {
			return nil, err
		}
		cronJobs = append(cronJobs, &CronJob{
			Id:        id,
			Schedule:  schedule,
			ImageName: imageName,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return cronJobs, nil
}

func (p *postgresManager) Close() {
	p.conn.Close()
}
