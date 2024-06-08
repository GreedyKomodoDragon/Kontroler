package db

import (
	"context"
	"fmt"

	"github.com/GreedyKomodoDragon/KubeConductor/operator/api/v1alpha1"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	cron "github.com/robfig/cron/v3"
)

type postgresDAGManager struct {
	pool   *pgxpool.Pool
	parser *cron.Parser
}

func NewPostgresDAGManager(ctx context.Context, pool *pgxpool.Pool, parser *cron.Parser) (DBDAGManager, error) {
	if parser == nil {
		return nil, fmt.Errorf("missing parser")
	}

	return &postgresDAGManager{
		pool:   pool,
		parser: parser,
	}, nil
}

func (p *postgresDAGManager) InitaliseDatabase(ctx context.Context) error {
	// Initialize the database schema
	initSQL := `
CREATE TABLE IF NOT EXISTS DAGs (
    dag_id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
	version INTEGER NOT NULL,
    schedule VARCHAR(255) NOT NULL,
	active BOOL NOT NULL
);

CREATE TABLE IF NOT EXISTS Tasks (
	task_id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    command TEXT[] NOT NULL,
    args TEXT[] NOT NULL,
    image VARCHAR(255) NOT NULL
);

CREATE TABLE IF NOT EXISTS Dependencies (
    task_id INTEGER NOT NULL,
    depends_on_task_id INTEGER NOT NULL,
    FOREIGN KEY (task_id) REFERENCES Tasks(task_id),
    FOREIGN KEY (depends_on_task_id) REFERENCES Tasks(task_id)
);

CREATE TABLE IF NOT EXISTS DAG_Tasks (
    dag_id INTEGER NOT NULL,
    task_id INTEGER NOT NULL,
    FOREIGN KEY (dag_id) REFERENCES DAGs(dag_id),
    FOREIGN KEY (task_id) REFERENCES Tasks(task_id)
);
`

	if _, err := p.pool.Exec(ctx, initSQL); err != nil {
		return err
	}

	return nil
}

func (p *postgresDAGManager) UpsertDAG(ctx context.Context, dag *v1alpha1.DAG) error {
	// Check if the DAG already exists
	// Begin transaction
	tx, err := p.pool.Begin(context.Background())
	if err != nil {
		return err
	}

	var existingDAGID int
	var version int
	err = tx.QueryRow(ctx, "SELECT dag_id, version FROM DAGs WHERE name = $1", dag.Name).Scan(&existingDAGID, &version)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}

	// Rollback transaction if not committed
	defer tx.Rollback(ctx)

	if existingDAGID != 0 {
		version++
	}

	// DAG does not exist, insert it
	if err := p.insertDAG(ctx, tx, dag, version); err != nil {
		return err
	}

	// SET previous version to false - allows version but stops multiple versions running
	if err := p.setInactive(ctx, tx, dag.Name, version-1); err != nil {
		return err
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}

func (p *postgresDAGManager) setInactive(ctx context.Context, tx pgx.Tx, name string, prevVersion int) error {
	_, err := tx.Exec(ctx, "UPDATE DAGs SET active = FALSE WHERE name = $1 and version = $2", name, prevVersion)
	if err != nil {
		return err
	}

	return nil
}

// insertDAG inserts a new DAG object into the database.
func (p *postgresDAGManager) insertDAG(ctx context.Context, tx pgx.Tx, dag *v1alpha1.DAG, version int) error {
	var dagID int
	err := tx.QueryRow(ctx, "INSERT INTO DAGs (name, version, schedule, active) VALUES ($1, $2, $3, TRUE) RETURNING dag_id", dag.Name, version, dag.Spec.Schedule).Scan(&dagID)
	if err != nil {
		return err
	}

	// Insert tasks and map them to the DAG
	for _, task := range dag.Spec.Task {
		if err = p.insertTask(ctx, tx, dagID, &task); err != nil {
			return err
		}
	}

	return nil
}

func (p *postgresDAGManager) insertTask(ctx context.Context, tx pgx.Tx, dagID int, task *v1alpha1.TaskSpec) error {
	// Insert the task
	var taskId int
	err := tx.QueryRow(ctx, "INSERT INTO Tasks (name, command, args, image) VALUES ($1, $2, $3, $4) RETURNING task_id", task.Name, task.Command, task.Args, task.Image).Scan(&taskId)
	if err != nil {
		return err
	}

	// Map the task to the DAG
	_, err = tx.Exec(ctx, "INSERT INTO DAG_Tasks (dag_id, task_id) VALUES ($1, $2)", dagID, taskId)
	if err != nil {
		return err
	}

	// Insert task dependencies
	for _, dependency := range task.RunAfter {
		var depId int
		err = tx.QueryRow(ctx, "SELECT task_id FROM tasks WHERE task_id in (SELECT task_id FROM DAG_Tasks WHERE dag_id = $1) and name = $2", dagID, dependency).Scan(&depId)
		if err != nil && err != pgx.ErrNoRows {
			return err
		}
		_, err = tx.Exec(ctx, "INSERT INTO Dependencies (task_id, depends_on_task_id) VALUES ($1, $2)", taskId, depId)
		if err != nil {
			return err
		}
	}

	return nil
}
