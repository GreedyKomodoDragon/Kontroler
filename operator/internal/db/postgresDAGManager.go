package db

import (
	"context"
	"fmt"
	"time"

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
	// TODO: Right-size the columns + select correct types
	initSQL := `
CREATE TABLE IF NOT EXISTS DAGs (
    dag_id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
	version INTEGER NOT NULL,
    schedule VARCHAR(255) NOT NULL,
	active BOOL NOT NULL,
	nexttime TIMESTAMP NOT NULL
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

CREATE TABLE IF NOT EXISTS DAG_Runs (
	run_id SERIAL PRIMARY KEY,
    dag_id INTEGER NOT NULL,
	status VARCHAR(255) NOT NULL,
    FOREIGN KEY (dag_id) REFERENCES DAGs(dag_id)
);

CREATE TABLE IF NOT EXISTS Task_Runs (
	task_run_id SERIAL PRIMARY KEY,
	run_id INTEGER NOT NULL,
    task_id INTEGER NOT NULL,
	status VARCHAR(255) NOT NULL,
    FOREIGN KEY (task_id) REFERENCES Tasks(task_id),
	FOREIGN KEY (run_id) REFERENCES DAG_Runs(run_id)
);
`

	if _, err := p.pool.Exec(ctx, initSQL); err != nil {
		return err
	}

	return nil
}

func (p *postgresDAGManager) InsertDAG(ctx context.Context, dag *v1alpha1.DAG) error {
	// Check if the DAG already exists
	// Begin transaction
	tx, err := p.pool.Begin(context.Background())
	if err != nil {
		return err
	}

	// Rollback transaction if not committed
	defer tx.Rollback(ctx)

	var existingDAGID int
	var version int
	err = tx.QueryRow(ctx, "SELECT dag_id, version FROM DAGs WHERE name = $1", dag.Name).Scan(&existingDAGID, &version)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}

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

	// Parse the cron expression
	sched, err := p.parser.Parse(dag.Spec.Schedule)
	if err != nil {
		return err
	}

	// Get the next occurrence of the scheduled time
	nextTime := sched.Next(time.Now())

	var dagID int
	if err = tx.QueryRow(ctx, `
	INSERT INTO DAGs (name, version, schedule, active, nexttime) 
	VALUES ($1, $2, $3, TRUE, $4) 
	RETURNING dag_id`, dag.Name, version, dag.Spec.Schedule, nextTime).Scan(&dagID); err != nil {
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

func (p *postgresDAGManager) CreateDAGRun(ctx context.Context, dagId int) (int, error) {
	// Map the task to the DAG
	var dagRunID int
	if err := p.pool.QueryRow(ctx, "INSERT INTO DAG_Runs (dag_id, status) VALUES ($1, 'running') RETURNING run_id", dagId).Scan(&dagRunID); err != nil {
		return 0, err
	}

	return dagRunID, nil
}

func (p *postgresDAGManager) GetStartingTasks(ctx context.Context, dagId int) ([]Task, error) {
	rows, err := p.pool.Query(ctx, `
	SELECT t.task_id, t.name, t.image, t.command, t.args
	FROM Tasks t
	LEFT JOIN Dependencies d ON t.task_id = d.task_id
	JOIN DAG_Tasks dt ON t.task_id = dt.task_id
	WHERE d.depends_on_task_id IS NULL AND dt.dag_id = $1;`, dagId)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	tasks := []Task{}
	for rows.Next() {
		var task Task
		if err := rows.Scan(&task.Id, &task.Name, &task.Image, &task.Command, &task.Args); err != nil {
			return nil, err
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

func (p *postgresDAGManager) MarkDAGRunOutcome(ctx context.Context, dagRunId int, outcome string) error {
	if _, err := p.pool.Exec(ctx, "UPDATE DAG_Runs SET status = $1 WHERE run_id = $2;", outcome, dagRunId); err != nil {
		return err
	}

	return nil
}

func (p *postgresDAGManager) MarkOutcomeAndGetNextTasks(ctx context.Context, taskRunId int, outcome string) ([]Task, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}

	defer tx.Rollback(ctx)

	var taskId int
	err = tx.QueryRow(ctx, "UPDATE Task_Runs SET status = $1 WHERE task_run_id = $2 RETURNING task_id", outcome, taskRunId).Scan(&taskId)
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}

	if outcome != "success" {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return []Task{}, nil
	}

	rows, err := tx.Query(ctx, `
		WITH CompletedTask AS (
			SELECT run_id, task_id 
			FROM Task_Runs 
			WHERE task_run_id = $1
		),
		DependCount AS (
			SELECT d.task_id, COUNT(*) as DependCount
			FROM Dependencies d
			WHERE d.task_id IN (
				SELECT DISTINCT d.task_id
				FROM Dependencies d
				WHERE d.depends_on_task_id IN (SELECT task_id FROM CompletedTask)
			)
			GROUP BY d.task_id
		),
		RunnableTask as (
			SELECT d.task_id
			FROM Dependencies d
			LEFT JOIN Task_Runs tr ON d.depends_on_task_id = tr.task_id AND tr.status = 'success'
			LEFT JOIN DependCount dc ON d.task_id = dc.task_id
			WHERE tr.status = 'success'
			GROUP BY d.task_id, dc.DependCount
			HAVING COUNT(*) = dc.DependCount
		)
		SELECT t.task_id, t.name, t.image, t.command, t.args
		FROM Tasks t
		WHERE t.task_id in (SELECT task_id FROM RunnableTask)
    `, taskRunId)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	tasks := []Task{}
	for rows.Next() {
		var task Task
		if err := rows.Scan(&task.Id, &task.Name, &task.Image, &task.Command, &task.Args); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (p *postgresDAGManager) MarkOutcome(ctx context.Context, taskRunId int, outcome string) error {
	_, err := p.pool.Exec(ctx, "UPDATE Task_Runs SET status = $1 WHERE task_run_id = $2;", outcome, taskRunId)
	return err
}

func (p *postgresDAGManager) MarkTaskAsStarted(ctx context.Context, runId int, taskId int) (int, error) {
	var taskRunId int
	err := p.pool.QueryRow(ctx, "INSERT INTO Task_Runs (run_id, task_id, status) VALUES ($1, $2, 'running') RETURNING task_run_id", runId, taskId).Scan(&taskRunId)
	if err != nil {
		return 0, err
	}

	return taskRunId, nil
}

func (p *postgresDAGManager) GetDAGsToStartAndUpdate(ctx context.Context) ([]int, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}

	defer tx.Rollback(ctx)

	// Maybe able to cut this down
	rows, err := tx.Query(ctx, `
        SELECT dag_id, schedule
        FROM DAGs
        WHERE nexttime <= NOW()
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int
	schedules := map[int]string{}
	for rows.Next() {
		var id int
		var schedule string
		if err := rows.Scan(&id, &schedule); err != nil {
			return nil, err
		}

		schedules[id] = schedule
		ids = append(ids, id)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	for id, schedule := range schedules {
		// Parse the cron expression
		sched, err := p.parser.Parse(schedule)
		if err != nil {
			return nil, err
		}

		// Get the next occurrence of the scheduled time
		nextTime := sched.Next(time.Now())

		if _, err := tx.Exec(ctx, `
		UPDATE DAGs 
		SET nextTime = $1 
		WHERE dag_id = $2;`, nextTime, id); err != nil {
			return nil, err
		}

	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return ids, nil
}
