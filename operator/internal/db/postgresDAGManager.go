package db

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/GreedyKomodoDragon/KubeConductor/operator/api/v1alpha1"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	cron "github.com/robfig/cron/v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
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
	hash VARCHAR(64) NOT NULL,
    schedule VARCHAR(255) NOT NULL,
	namespace VARCHAR(255) NOT NULL,
	active BOOL NOT NULL,
	taskCount INTEGER NOT NULL,
	nexttime TIMESTAMP
);

CREATE TABLE IF NOT EXISTS DAG_Parameters (
	parameter_id SERIAL PRIMARY KEY,
    dag_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
	isSecret BOOL NOT NULL,
	defaultValue VARCHAR(255) NOT NULL,
	FOREIGN KEY (dag_id) REFERENCES DAGs(dag_id)
);

CREATE TABLE IF NOT EXISTS Tasks (
	task_id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    command TEXT[] NOT NULL,
    args TEXT[] NOT NULL,
    image VARCHAR(255) NOT NULL,
	parameters TEXT[],
	backoffLimit BIGINT NOT NULL,
	isConditional BOOL NOT NULL,
	podTemplate JSONB,
	retryCodes INTEGER[]
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
	name VARCHAR(255) NOT NULL,
    dag_id INTEGER NOT NULL,
	status VARCHAR(255) NOT NULL,
	successfulCount INTEGER NOT NULL,
	failedCount INTEGER NOT NULL,
	run_time TIMESTAMP NOT NULL,
    FOREIGN KEY (dag_id) REFERENCES DAGs(dag_id),
	UNIQUE(name)
);

CREATE TABLE IF NOT EXISTS DAG_Run_Parameters (
	param_id SERIAL PRIMARY KEY,
    run_id INTEGER NOT NULL,
	name VARCHAR(255) NOT NULL,
	value  VARCHAR(255) NOT NULL,
	isSecret BOOL NOT NULL,
    FOREIGN KEY (run_id) REFERENCES DAG_Runs(run_id)
);

CREATE TABLE IF NOT EXISTS Task_Runs (
	task_run_id SERIAL PRIMARY KEY,
	run_id INTEGER NOT NULL,
    task_id INTEGER NOT NULL,
	status VARCHAR(255) NOT NULL,
	attempts INTEGER NOT NULL,
    FOREIGN KEY (task_id) REFERENCES Tasks(task_id),
	FOREIGN KEY (run_id) REFERENCES DAG_Runs(run_id)
);

CREATE TABLE IF NOT EXISTS Task_Pods (
    Pod_UID VARCHAR(255) NOT NULL,
    task_run_id INTEGER NOT NULL,
    exitCode INTEGER,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(255) NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (Pod_UID),
    FOREIGN KEY (task_run_id) REFERENCES Task_Runs(task_run_id)
);

`

	if _, err := p.pool.Exec(ctx, initSQL); err != nil {
		return err
	}

	return nil
}

func hashDagSpec(s *v1alpha1.DAGSpec) []byte {
	// Convert the DAGSpec to JSON
	data, err := json.Marshal(s)
	if err != nil {
		// Handle the error appropriately
		return nil
	}

	// Hash the JSON bytes
	hash := sha256.New()
	hash.Write(data)

	return hash.Sum(nil)
}

func (p *postgresDAGManager) InsertDAG(ctx context.Context, dag *v1alpha1.DAG, namespace string) error {
	// Check if the DAG already exists
	// Begin transaction
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}

	// Rollback transaction if not committed
	defer tx.Rollback(ctx)

	var existingDAGID int
	var version int
	var hash string

	err = tx.QueryRow(ctx, `
	SELECT dag_id, version, hash
	FROM DAGs
	WHERE name = $1 AND namespace = $2
	ORDER BY version DESC;`, dag.Name, namespace).Scan(&existingDAGID, &version, &hash)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}

	hashBytes := hashDagSpec(&dag.Spec)
	if hashBytes == nil {
		return fmt.Errorf("failed to create hash")
	}

	hashValue := fmt.Sprintf("%x", hashBytes)
	if hash == hashValue {
		return fmt.Errorf("applying the same dag")
	}

	if existingDAGID != 0 {
		version++
	}

	// DAG does not exist, insert it
	if err := p.insertDAG(ctx, tx, dag, version, namespace, hashValue); err != nil {
		return err
	}

	// SET previous version to false - allows version but stops multiple versions running
	if err := p.setInactive(ctx, tx, dag.Name, namespace, version-1); err != nil {
		return err
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}

// insertDAG inserts a new DAG object into the database.
func (p *postgresDAGManager) insertDAG(ctx context.Context, tx pgx.Tx, dag *v1alpha1.DAG, version int, namespace string, hash string) error {

	// Parse the cron expression
	var nextTime *time.Time

	// Could be an event driven only score
	if dag.Spec.Schedule != "" {
		sched, err := p.parser.Parse(dag.Spec.Schedule)
		if err != nil {
			return err
		}

		// Get the next occurrence of the scheduled time
		t := sched.Next(time.Now())
		nextTime = &t
	}

	var dagID int
	if err := tx.QueryRow(ctx, `
	INSERT INTO DAGs (name, version, hash, schedule, namespace, active, nexttime, taskCount) 
	VALUES ($1, $2, $3, $4, $5, TRUE, $6, $7)
	RETURNING dag_id`, dag.Name, version, hash, dag.Spec.Schedule, namespace, nextTime, len(dag.Spec.Task)).Scan(&dagID); err != nil {
		return err
	}

	// Insert tasks and map them to the DAG
	for _, task := range dag.Spec.Task {
		if err := p.insertTask(ctx, tx, dagID, &task); err != nil {
			return err
		}
	}

	// Insert parameters and map them to the DAG
	for _, parameter := range dag.Spec.Parameters {
		if err := p.insertParameter(ctx, tx, dagID, &parameter); err != nil {
			return err
		}
	}

	return nil
}

func (p *postgresDAGManager) insertParameter(ctx context.Context, tx pgx.Tx, dagID int, parameter *v1alpha1.DagParameterSpec) error {
	value := parameter.DefaultFromSecret
	isSecret := parameter.DefaultValue == ""
	if !isSecret {
		value = parameter.DefaultValue
	}

	// Map the task to the DAG
	if _, err := tx.Exec(ctx, `
	INSERT INTO DAG_Parameters (dag_id, name, isSecret, defaultValue) 
	VALUES ($1, $2, $3, $4)`, dagID, parameter.Name, isSecret, value); err != nil {
		return err
	}

	return nil
}

func (p *postgresDAGManager) insertTask(ctx context.Context, tx pgx.Tx, dagID int, task *v1alpha1.TaskSpec) error {
	var jsonValue *string
	if task.PodTemplate != nil {
		json, err := task.PodTemplate.Serialize()
		if err != nil {
			return err
		}

		jsonValue = &json
	}

	// Insert the task
	var taskId int
	if err := tx.QueryRow(ctx, `
	INSERT INTO Tasks (name, command, args, image, parameters, backoffLimit, isConditional, retryCodes, podTemplate) 
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) 
	RETURNING task_id`,
		task.Name, task.Command, task.Args, task.Image, task.Parameters, task.Backoff.Limit,
		task.Conditional.Enabled, task.Conditional.RetryCodes, jsonValue).Scan(&taskId); err != nil {
		return err
	}

	// Map the task to the DAG
	if _, err := tx.Exec(ctx, `
	INSERT INTO DAG_Tasks (dag_id, task_id)
	 VALUES ($1, $2)`, dagID, taskId); err != nil {
		return err
	}

	// Insert task dependencies
	for _, dependency := range task.RunAfter {
		var depId int
		err := tx.QueryRow(ctx, `
		SELECT task_id FROM tasks
		WHERE task_id in (SELECT task_id FROM DAG_Tasks WHERE dag_id = $1)
			AND name = $2`, dagID, dependency).Scan(&depId)
		if err != nil && err != pgx.ErrNoRows {
			return err
		}

		if _, err = tx.Exec(ctx, `
		INSERT INTO Dependencies (task_id, depends_on_task_id) 
		VALUES ($1, $2)`, taskId, depId); err != nil {
			return err
		}
	}

	return nil
}

func (p *postgresDAGManager) CreateDAGRun(ctx context.Context, name string, dag *v1alpha1.DagRunSpec, parameters map[string]v1alpha1.ParameterSpec) (int, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}

	defer tx.Rollback(ctx)

	// Map the task to the DAG
	var dagRunID int
	if err := tx.QueryRow(ctx, `
	INSERT INTO DAG_Runs (dag_id, name, status, successfulCount, failedCount, run_time) 
	VALUES ($1, $2, 'running', 0, 0, NOW()) 
	RETURNING run_id`, dag.DagId, name).Scan(&dagRunID); err != nil {
		return 0, err
	}

	for _, param := range parameters {
		value := param.Value
		if param.FromSecret != "" {
			value = param.FromSecret
		}

		if _, err := tx.Exec(ctx, "INSERT INTO DAG_Run_Parameters (run_id, name, value, isSecret) VALUES ($1, $2, $3, $4);", dagRunID, param.Name, value, param.FromSecret != ""); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	return dagRunID, nil
}

func (p *postgresDAGManager) GetStartingTasks(ctx context.Context, dagId int) ([]Task, error) {
	rows, err := p.pool.Query(ctx, `
	SELECT t.task_id, t.name, t.image, t.command, t.args, t.parameters, t.podtemplate
	FROM Tasks t
	LEFT JOIN Dependencies d ON t.task_id = d.task_id
	JOIN DAG_Tasks dt ON t.task_id = dt.task_id
	WHERE d.depends_on_task_id IS NULL AND dt.dag_id = $1;
	`, dagId)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	tasks := []Task{}
	for rows.Next() {
		var task Task
		var parameters []string
		var podTemplateJSON string
		if err := rows.Scan(&task.Id, &task.Name, &task.Image, &task.Command, &task.Args, &parameters, &podTemplateJSON); err != nil {
			return nil, err
		}

		task.Parameters = []Parameter{}
		for _, parameter := range parameters {
			param := Parameter{
				Name: parameter,
			}

			if err := p.pool.QueryRow(ctx, `
			SELECT isSecret, defaultValue
			FROM DAG_Parameters
			WHERE dag_id = $1 and name = $2;
			`, dagId, parameter).Scan(&param.IsSecret, &param.Value); err != nil {
				return nil, err
			}

			task.Parameters = append(task.Parameters, param)
		}

		var podTemplate *v1alpha1.PodTemplateSpec
		if podTemplateJSON != "" {
			if err := json.Unmarshal([]byte(podTemplateJSON), &podTemplate); err != nil {
				return nil, err
			}
		}

		task.PodTemplate = podTemplate

		tasks = append(tasks, task)

	}

	return tasks, nil
}

func (p *postgresDAGManager) MarkDAGRunOutcome(ctx context.Context, dagRunId int, outcome string) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "UPDATE DAG_Runs SET status = $1 WHERE run_id = $2;", outcome, dagRunId); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}

func (p *postgresDAGManager) MarkSuccessAndGetNextTasks(ctx context.Context, taskRunId int) ([]Task, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}

	defer tx.Rollback(ctx)

	var taskId int
	var runId int
	err = tx.QueryRow(ctx, "UPDATE Task_Runs SET status = 'success' WHERE task_run_id = $1 RETURNING task_id, run_id", taskRunId).Scan(&taskId, &runId)
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
			UPDATE DAG_Runs 
			SET successfulCount = successfulCount + 1
			WHERE run_id = $1;`, runId); err != nil {
		return nil, err
	}

	var status string
	err = tx.QueryRow(ctx, `
		UPDATE DAG_Runs
		SET status = 'success'
		FROM DAGs
		WHERE DAG_Runs.dag_id = DAGs.dag_id
		AND DAGs.taskCount = DAG_Runs.successfulCount
		AND DAG_Runs.run_id = $1
		RETURNING DAG_Runs.status;
	`, runId).Scan(&status)

	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}

	if status == "success" {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}

		return []Task{}, nil
	}

	var dagId int
	err = tx.QueryRow(ctx, `
		SELECT dag_id
		FROM dag_runs
		WHERE run_id = $1
	`, runId).Scan(&dagId)

	if err != nil {
		return nil, err
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
			LEFT JOIN Task_Runs tr ON tr.task_id = d.depends_on_task_id
			LEFT JOIN DependCount dc ON d.task_id = dc.task_id
			WHERE tr.status = 'success' AND task_run_id = $1
			GROUP BY d.task_id, dc.DependCount
			HAVING COUNT(*) = dc.DependCount
		)
		SELECT t.task_id, t.name, t.image, t.command, t.args, t.parameters
		FROM Tasks t
		WHERE t.task_id in (SELECT task_id FROM RunnableTask)
    `, taskRunId)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	tasks := []Task{}
	parameters := [][]string{}
	for rows.Next() {
		var task Task
		var params []string
		if err := rows.Scan(&task.Id, &task.Name, &task.Image, &task.Command, &task.Args, &params); err != nil {
			return nil, err
		}

		// can be a null return
		if params == nil {
			params = []string{}
		}

		parameters = append(parameters, params)
		tasks = append(tasks, task)
	}

	for i := 0; i < len(tasks); i++ {
		tasks[i].Parameters = []Parameter{}
		for _, parameter := range parameters[i] {
			param := Parameter{
				Name: parameter,
			}

			err := tx.QueryRow(ctx, `
			SELECT isSecret, defaultValue
			FROM DAG_Parameters
			WHERE dag_id = $1 and name = $2;
			`, dagId, parameter).Scan(&param.IsSecret, &param.Value)

			if err == pgx.ErrNoRows {
				continue
			}

			if err != nil {
				return nil, err
			}

			tasks[i].Parameters = append(tasks[i].Parameters, param)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (p *postgresDAGManager) IncrementAttempts(ctx context.Context, taskRunId int) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
	UPDATE Task_Runs 
	SET attempts = attempts + 1
	WHERE task_run_id = $1 
	`, taskRunId); err != nil {
		return err
	}

	// TODO: Mark pod as failed

	return tx.Commit(ctx)
}

func (p *postgresDAGManager) MarkTaskAsStarted(ctx context.Context, runId int, taskId int) (int, error) {
	var taskRunId int
	if err := p.pool.QueryRow(ctx, `
	INSERT INTO Task_Runs (run_id, task_id, status, attempts) 
	VALUES ($1, $2, 'running', 1) 
	RETURNING task_run_id`,
		runId, taskId).Scan(&taskRunId); err != nil {
		return 0, err
	}

	return taskRunId, nil
}

func (p *postgresDAGManager) GetDAGsToStartAndUpdate(ctx context.Context) ([]int, []string, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}

	defer tx.Rollback(ctx)

	// Maybe able to cut this down
	rows, err := tx.Query(ctx, `
        SELECT dag_id, schedule, namespace
        FROM DAGs
        WHERE nexttime <= NOW() AND schedule != '' AND active = TRUE;
    `)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	ids := []int{}
	namespaces := []string{}

	schedules := map[int]string{}
	for rows.Next() {
		var id int
		var schedule string
		var namespace string
		if err := rows.Scan(&id, &schedule, &namespace); err != nil {
			return nil, nil, err
		}

		schedules[id] = schedule
		ids = append(ids, id)
		namespaces = append(namespaces, namespace)
	}

	if err = rows.Err(); err != nil {
		return nil, nil, err
	}

	for id, schedule := range schedules {
		// Parse the cron expression
		sched, err := p.parser.Parse(schedule)
		if err != nil {
			return nil, nil, err
		}

		// Get the next occurrence of the scheduled time
		nextTime := sched.Next(time.Now())

		if _, err := tx.Exec(ctx, `
		UPDATE DAGs 
		SET nextTime = $1 
		WHERE dag_id = $2;`, nextTime, id); err != nil {
			return nil, nil, err
		}

	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}

	return ids, namespaces, nil
}

func (p *postgresDAGManager) GetDagParameters(ctx context.Context, dagId int) (map[string]*Parameter, error) {
	rows, err := p.pool.Query(ctx, `
	SELECT name, isSecret, defaultValue
	FROM DAG_Parameters
	WHERE dag_id = $1;
	`, dagId)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	parameters := map[string]*Parameter{}
	for rows.Next() {
		var parameter Parameter
		if err := rows.Scan(&parameter.Name, &parameter.IsSecret, &parameter.Value); err != nil {
			return nil, err
		}

		parameters[parameter.Name] = &parameter
	}

	return parameters, nil
}

func (p *postgresDAGManager) DagExists(ctx context.Context, dagId int) (bool, error) {
	var name string
	err := p.pool.QueryRow(ctx, `
	SELECT name
	FROM DAG_Parameters
	WHERE dag_id = $1;
	`, dagId).Scan(&name)

	if err != nil && err != pgx.ErrNoRows {
		return false, err
	}

	return name != "", nil
}

func (p *postgresDAGManager) ShouldRerun(ctx context.Context, taskRunid int, exitCode int32) (bool, error) {
	// Query to check if rerun is needed based on join and conditions
	query := `
	SELECT t.backoffLimit, r.attempts
	FROM tasks t
	INNER JOIN Task_Runs r ON t.task_id = r.task_id
	WHERE r.task_run_id = $1 AND r.attempts <= t.backoffLimit AND (t.isConditional = FALSE or $2 = ANY(t.retryCodes));
    `

	rows, err := p.pool.Query(ctx, query, taskRunid, exitCode)
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

func (p *postgresDAGManager) MarkTaskAsFailed(ctx context.Context, taskRunId int) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE Task_Runs 
		SET status = 'failed' 
		WHERE task_run_id = $1 ;
	`, taskRunId); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
	    UPDATE DAG_Runs
	    SET
	        failedCount = failedCount + 1,
	        status = 'failed'
	    WHERE run_id in (
			SELECT run_id
			FROM Task_Runs
			WHERE task_run_id = $1
		);`, taskRunId); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (p *postgresDAGManager) MarkPodStatus(ctx context.Context, podUid types.UID, name string, taskRunID int, status v1.PodPhase, tStamp time.Time, exitCode *int32) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)

	// Get the current status and timestamp from the database
	var currentStatus v1.PodPhase
	var currentTimestamp time.Time
	err = tx.QueryRow(ctx, `
        SELECT status, updated_at FROM Task_Pods WHERE Pod_UID = $1 AND task_run_id = $2
    `, podUid, taskRunID).Scan(&currentStatus, &currentTimestamp)

	if err != nil && err != pgx.ErrNoRows {
		return err
	}

	// Compare timestamps and skip the update if the current status is newer
	if currentTimestamp.After(tStamp) {
		return nil // The database already has a newer status, so skip this update
	}

	// Insert the new status with the current timestamp
	if _, err = tx.Exec(ctx, `
        INSERT INTO Task_Pods (Pod_UID, task_run_id, name, status, updated_at, exitCode)
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (Pod_UID) 
        DO UPDATE SET status = EXCLUDED.status, updated_at = EXCLUDED.updated_at, exitCode = EXCLUDED.exitCode
        WHERE Task_Pods.updated_at < EXCLUDED.updated_at;
    `, podUid, taskRunID, name, status, tStamp, exitCode); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (p *postgresDAGManager) SoftDeleteDAG(ctx context.Context, name string, namespace string) error {
	// Check if the DAG already exists
	// Begin transaction
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}

	// Rollback transaction if not committed
	defer tx.Rollback(ctx)

	var version int
	if err := tx.QueryRow(ctx, `
	SELECT version
	FROM DAGs
	WHERE name = $1 AND namespace = $2
	ORDER BY version DESC;`, name, namespace).Scan(&version); err != nil {
		return err
	}

	if err := p.setInactive(ctx, tx, name, namespace, version); err != nil {
		return err
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}

func (p *postgresDAGManager) setInactive(ctx context.Context, tx pgx.Tx, name string, namespace string, prevVersion int) error {
	if _, err := tx.Exec(ctx, `
	UPDATE DAGs 
	SET active = FALSE 
	WHERE name = $1 AND namespace = $2 AND version = $3`, name, namespace, prevVersion); err != nil {
		return err
	}

	return nil
}
