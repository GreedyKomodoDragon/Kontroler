package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/GreedyKomodoDragon/Kontroler/operator/api/v1alpha1"
	"github.com/jackc/pgx/v5"
	cron "github.com/robfig/cron/v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	_ "github.com/mattn/go-sqlite3"
)

// sqliteDAGManager manages the SQLite database connection and interactions.
type sqliteDAGManager struct {
	db     *sql.DB
	parser *cron.Parser
}

func NewSqliteManager(ctx context.Context, dbPath string, parser *cron.Parser) (DBDAGManager, error) {
	if parser == nil {
		return nil, fmt.Errorf("missing parser")
	}

	// Open a connection to the SQLite database file.
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	// Check the connection to ensure the database is accessible.
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to SQLite database: %w", err)
	}

	return &sqliteDAGManager{
		db:     db,
		parser: parser,
	}, nil
}

func (s *sqliteDAGManager) InitaliseDatabase(ctx context.Context) error {
	initScript := `
-- SQLite does not support UUID generation directly. Use TEXT with UNIQUE constraint.
CREATE TABLE IF NOT EXISTS IdTable (
    unique_id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6))))
);

-- SQLite does not have SERIAL, so we use INTEGER PRIMARY KEY with AUTOINCREMENT.
CREATE TABLE IF NOT EXISTS DAGs (
    dag_id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(255) NOT NULL,
    version INTEGER NOT NULL,
    hash VARCHAR(64) NOT NULL,
    schedule VARCHAR(255) NOT NULL,
    namespace VARCHAR(255) NOT NULL,
    active BOOLEAN NOT NULL,
    taskCount INTEGER NOT NULL,
    nexttime TIMESTAMP,
    UNIQUE(name, version)
);

-- DAG_Parameters table
CREATE TABLE IF NOT EXISTS DAG_Parameters (
    parameter_id INTEGER PRIMARY KEY AUTOINCREMENT,
    dag_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    isSecret BOOLEAN NOT NULL,
    defaultValue VARCHAR(255) NOT NULL,
    FOREIGN KEY (dag_id) REFERENCES DAGs(dag_id)
);

-- Tasks table
CREATE TABLE IF NOT EXISTS Tasks (
    task_id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(255) NOT NULL,
    command TEXT,  -- SQLite doesn’t support arrays; store as TEXT or use JSON format
    args TEXT,
    image VARCHAR(255) NOT NULL,
    parameters TEXT,
    backoffLimit BIGINT NOT NULL,
    isConditional BOOLEAN NOT NULL,
    podTemplate TEXT,  -- Replace JSONB with TEXT or consider JSON1 extension for JSON data
    retryCodes TEXT,   -- Store as TEXT or JSON if array data is needed
    script TEXT NOT NULL,
    scriptInjectorImage TEXT
);

-- Dependencies table
CREATE TABLE IF NOT EXISTS Dependencies (
    task_id INTEGER NOT NULL,
    depends_on_task_id INTEGER NOT NULL,
    FOREIGN KEY (task_id) REFERENCES Tasks(task_id),
    FOREIGN KEY (depends_on_task_id) REFERENCES Tasks(task_id)
);

-- DAG_Tasks table
CREATE TABLE IF NOT EXISTS DAG_Tasks (
    dag_id INTEGER NOT NULL,
    task_id INTEGER NOT NULL,
    FOREIGN KEY (dag_id) REFERENCES DAGs(dag_id),
    FOREIGN KEY (task_id) REFERENCES Tasks(task_id)
);

-- DAG_Runs table
CREATE TABLE IF NOT EXISTS DAG_Runs (
    run_id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(255) NOT NULL,
    dag_id INTEGER NOT NULL,
    status VARCHAR(255) NOT NULL,
    successfulCount INTEGER NOT NULL,
    failedCount INTEGER NOT NULL,
    run_time TIMESTAMP NOT NULL,
    FOREIGN KEY (dag_id) REFERENCES DAGs(dag_id),
    UNIQUE(name)
);

-- DAG_Run_Parameters table
CREATE TABLE IF NOT EXISTS DAG_Run_Parameters (
    param_id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    value VARCHAR(255) NOT NULL,
    isSecret BOOLEAN NOT NULL,
    FOREIGN KEY (run_id) REFERENCES DAG_Runs(run_id)
);

-- Task_Runs table
CREATE TABLE IF NOT EXISTS Task_Runs (
    task_run_id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id INTEGER NOT NULL,
    task_id INTEGER NOT NULL,
    status VARCHAR(255) NOT NULL,
    attempts INTEGER NOT NULL,
    FOREIGN KEY (task_id) REFERENCES Tasks(task_id),
    FOREIGN KEY (run_id) REFERENCES DAG_Runs(run_id)
);

-- Task_Pods table
CREATE TABLE IF NOT EXISTS Task_Pods (
    Pod_UID VARCHAR(255) PRIMARY KEY,
    task_run_id INTEGER NOT NULL,
    exitCode INTEGER,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(255) NOT NULL,
    namespace TEXT NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (task_run_id) REFERENCES Task_Runs(task_run_id)
);
	`

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, initScript); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *sqliteDAGManager) GetID(ctx context.Context) (string, error) {
	return "", nil
}

func (s *sqliteDAGManager) GetDAGsToStartAndUpdate(ctx context.Context) ([]*DagInfo, error) {
	return nil, nil
}

func (s *sqliteDAGManager) InsertDAG(ctx context.Context, dag *v1alpha1.DAG, namespace string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var existingDAGID int
	var version int
	var hash string

	err = tx.QueryRow(`
	SELECT dag_id, version, hash
	FROM DAGs
	WHERE name = ? AND namespace = ?
	ORDER BY version DESC;`, dag.Name, namespace).Scan(&existingDAGID, &version, &hash)
	if err != nil && err != sql.ErrNoRows {
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
	if err := s.insertDAG(tx, dag, version, namespace, hashValue); err != nil {
		return err
	}

	// SET previous version to false - allows version but stops multiple versions running
	if err := s.setInactive(tx, dag.Name, namespace, version-1); err != nil {
		return err
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

// insertDAG inserts a new DAG object into the database.
func (s *sqliteDAGManager) insertDAG(tx *sql.Tx, dag *v1alpha1.DAG, version int, namespace string, hash string) error {

	// Parse the cron expression
	var nextTime *time.Time

	// Could be an event driven only score
	if dag.Spec.Schedule != "" {
		sched, err := s.parser.Parse(dag.Spec.Schedule)
		if err != nil {
			return err
		}

		// Get the next occurrence of the scheduled time
		t := sched.Next(time.Now())
		nextTime = &t
	}

	var dagID int
	if err := tx.QueryRow(`
	INSERT INTO DAGs (name, version, hash, schedule, namespace, active, nexttime, taskCount) 
	VALUES (?, ?, ?, ?, ?, TRUE, ?, ?)
	RETURNING dag_id`, dag.Name, version, hash, dag.Spec.Schedule, namespace, nextTime, len(dag.Spec.Task)).Scan(&dagID); err != nil {
		return err
	}

	// Insert tasks and map them to the DAG
	for _, task := range dag.Spec.Task {
		if err := s.insertTask(tx, dagID, &task); err != nil {
			return err
		}
	}

	// Insert parameters and map them to the DAG
	for _, parameter := range dag.Spec.Parameters {
		if err := s.insertParameter(tx, dagID, &parameter); err != nil {
			return err
		}
	}

	return nil
}

func (s *sqliteDAGManager) insertTask(tx *sql.Tx, dagID int, task *v1alpha1.TaskSpec) error {
	var jsonValue *string
	if task.PodTemplate != nil {
		json, err := task.PodTemplate.Serialize()
		if err != nil {
			return err
		}

		jsonValue = &json
	}

	// SQLite has no slice/array type so we need to convert it to a JSON string
	commandJson, err := json.Marshal(task.Command)
	if err != nil {
		return err
	}

	argsJson, err := json.Marshal(task.Args)
	if err != nil {
		return err
	}

	paramsJson, err := json.Marshal(task.Parameters)
	if err != nil {
		return err
	}

	retryCodesJson, err := json.Marshal(task.Conditional.RetryCodes)
	if err != nil {
		return err
	}

	// Insert the task
	var taskId int
	if err := tx.QueryRow(`
	INSERT INTO Tasks (name, command, args, image, parameters, backoffLimit, isConditional, retryCodes, podTemplate, script, scriptInjectorImage) 
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) 
	RETURNING task_id`,
		task.Name, commandJson, argsJson, task.Image, paramsJson, task.Backoff.Limit,
		task.Conditional.Enabled, retryCodesJson, jsonValue, task.Script, task.ScriptInjectorImage).Scan(&taskId); err != nil {
		return err
	}

	// Map the task to the DAG
	if _, err := tx.Exec(`
	INSERT INTO DAG_Tasks (dag_id, task_id)
	 VALUES (?, ?)`, dagID, taskId); err != nil {
		return err
	}

	// Insert task dependencies
	for _, dependency := range task.RunAfter {
		var depId int
		err := tx.QueryRow(`
		SELECT task_id FROM tasks
		WHERE task_id in (SELECT task_id FROM DAG_Tasks WHERE dag_id = ?)
			AND name = ?`, dagID, dependency).Scan(&depId)
		if err != nil && err != pgx.ErrNoRows {
			return err
		}

		if _, err = tx.Exec(`
		INSERT INTO Dependencies (task_id, depends_on_task_id) 
		VALUES (?, ?)`, taskId, depId); err != nil {
			return err
		}
	}

	return nil
}

func (s *sqliteDAGManager) insertParameter(tx *sql.Tx, dagID int, parameter *v1alpha1.DagParameterSpec) error {
	value := parameter.DefaultFromSecret
	isSecret := parameter.DefaultValue == ""
	if !isSecret {
		value = parameter.DefaultValue
	}

	// Map the task to the DAG
	if _, err := tx.Exec(`
	INSERT INTO DAG_Parameters (dag_id, name, isSecret, defaultValue) 
	VALUES (?, ?, ?, ?)`, dagID, parameter.Name, isSecret, value); err != nil {
		return err
	}

	return nil
}

func (s *sqliteDAGManager) setInactive(tx *sql.Tx, name string, namespace string, prevVersion int) error {
	if _, err := tx.Exec(`
	UPDATE DAGs 
	SET active = FALSE 
	WHERE name = ? AND namespace = ? AND version = ?`, name, namespace, prevVersion); err != nil {
		return err
	}

	return nil
}

func (s *sqliteDAGManager) CreateDAGRun(ctx context.Context, name string, dag *v1alpha1.DagRunSpec, parameters map[string]v1alpha1.ParameterSpec) (int, error) {
	dagId, err := s.dagNameToDagId(dag.DagName)
	if err != nil {
		return 0, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}

	defer tx.Rollback()

	// Map the task to the DAG
	var dagRunID int
	if err := tx.QueryRow(`
	INSERT INTO DAG_Runs (dag_id, name, status, successfulCount, failedCount, run_time) 
	VALUES (?, ?, 'running', 0, 0, datetime('now')) 
	RETURNING run_id`, dagId, name).Scan(&dagRunID); err != nil {
		return 0, err
	}

	for _, param := range parameters {
		value := param.Value
		if param.FromSecret != "" {
			value = param.FromSecret
		}

		if _, err := tx.Exec("INSERT INTO DAG_Run_Parameters (run_id, name, value, isSecret) VALUES (?, ?, ?, ?);", dagRunID, param.Name, value, param.FromSecret != ""); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return dagRunID, nil
}

func (s *sqliteDAGManager) dagNameToDagId(dagName string) (int, error) {
	dagId := -1
	if err := s.db.QueryRow(`
		SELECT dag_id
		FROM DAGs
		WHERE name = ?
		ORDER BY version DESC
		LIMIT 1;
	`, dagName).Scan(&dagId); err != nil {
		return -1, err
	}

	if dagId == -1 {
		return -1, fmt.Errorf("could not find dag")
	}

	return dagId, nil
}

func (s *sqliteDAGManager) GetStartingTasks(ctx context.Context, dagName string) ([]Task, error) {
	rows, err := s.db.Query(`
	SELECT t.task_id, t.name, t.image, t.command, t.args, t.parameters, t.podtemplate, dt.dag_id, t.script
	FROM Tasks t
	LEFT JOIN Dependencies d ON t.task_id = d.task_id
	JOIN DAG_Tasks dt ON t.task_id = dt.task_id
	WHERE d.depends_on_task_id IS NULL
	AND dt.dag_id IN (
		SELECT dag_id
		FROM DAGs
		WHERE name = ?
		ORDER BY version DESC
		LIMIT 1
  	);
	`, dagName)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	tasks := []Task{}
	for rows.Next() {
		task := Task{}
		var podTemplateJSON *string
		var dagId int

		// Needed as stored as TEXT and not []TEXT
		var commandJSON string
		var argsJSON string
		var paramJSON string

		if err := rows.Scan(&task.Id, &task.Name, &task.Image, &commandJSON, &argsJSON, &paramJSON, &podTemplateJSON, &dagId, &task.Script); err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(commandJSON), &task.Command); err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(argsJSON), &task.Args); err != nil {
			return nil, err
		}

		parameters := []string{}
		if err := json.Unmarshal([]byte(paramJSON), &parameters); err != nil {
			return nil, err
		}

		task.Parameters = []Parameter{}
		for _, parameter := range parameters {
			param := Parameter{
				Name: parameter,
			}

			if err := s.db.QueryRow(`
			SELECT isSecret, defaultValue
			FROM DAG_Parameters
			WHERE dag_id = ? and name = ?;
			`, dagId, parameter).Scan(&param.IsSecret, &param.Value); err != nil {
				return nil, err
			}

			task.Parameters = append(task.Parameters, param)
		}

		var podTemplate *v1alpha1.PodTemplateSpec
		if podTemplateJSON != nil {
			if err := json.Unmarshal([]byte(*podTemplateJSON), &podTemplate); err != nil {
				return nil, err
			}
		}

		task.PodTemplate = podTemplate

		tasks = append(tasks, task)

	}

	return tasks, nil
}

func (s *sqliteDAGManager) MarkTaskAsStarted(ctx context.Context, runId, taskId int) (int, error) {
	var taskRunId int

	if err := s.db.QueryRow(`
	INSERT INTO Task_Runs (run_id, task_id, status, attempts) 
	VALUES (?, ?, 'running', 1) 
	RETURNING task_run_id`,
		runId, taskId).Scan(&taskRunId); err != nil {
		return 0, err
	}

	return taskRunId, nil
}

func (s *sqliteDAGManager) IncrementAttempts(ctx context.Context, taskRunId int) error {
	return nil
}

func (s *sqliteDAGManager) MarkSuccessAndGetNextTasks(ctx context.Context, taskRunId int) ([]Task, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var taskId int
	var runId int
	err = tx.QueryRow("UPDATE Task_Runs SET status = 'success' WHERE task_run_id = ? RETURNING task_id, run_id", taskRunId).Scan(&taskId, &runId)
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}

	if _, err := tx.Exec(`
			UPDATE DAG_Runs 
			SET successfulCount = successfulCount + 1
			WHERE run_id = ?;`, runId); err != nil {
		return nil, err
	}

	var status string
	err = tx.QueryRow(`
		UPDATE DAG_Runs
		SET status = 'success'
		FROM DAGs
		WHERE DAG_Runs.dag_id = DAGs.dag_id
		AND DAGs.taskCount = DAG_Runs.successfulCount
		AND DAG_Runs.run_id = ?
		RETURNING DAG_Runs.status;
	`, runId).Scan(&status)

	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	if status == "success" {
		if err := tx.Commit(); err != nil {
			return nil, err
		}

		return []Task{}, nil
	}

	var dagId int
	err = tx.QueryRow(`
		SELECT dag_id
		FROM dag_runs
		WHERE run_id = ?
	`, runId).Scan(&dagId)

	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(`
		WITH CompletedTask AS (
			SELECT run_id, task_id 
			FROM Task_Runs 
			WHERE task_run_id = ?
		),
		DependCount AS (
			SELECT d.task_id, COUNT(*) AS DependCount
			FROM Dependencies d
			JOIN Dependencies dep ON d.task_id = dep.task_id
			JOIN CompletedTask ct ON dep.depends_on_task_id = ct.task_id
			GROUP BY d.task_id
		),
		RunnableTask as (
			SELECT d.task_id
			FROM Dependencies d
			LEFT JOIN Task_Runs tr ON tr.task_id = d.depends_on_task_id
			LEFT JOIN DependCount dc ON d.task_id = dc.task_id
			WHERE tr.status = 'success' AND task_run_id = ?
			GROUP BY d.task_id, dc.DependCount
			HAVING COUNT(*) = dc.DependCount
		)
		SELECT t.task_id, t.name, t.image, t.command, t.args, t.parameters, t.ScriptInjectorImage
		FROM Tasks t
		WHERE 
			t.task_id in (SELECT task_id FROM RunnableTask) 
			AND t.task_id not in (select task_id FROM task_runs WHERE run_id = ?)
    `, taskRunId, taskRunId, runId)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	tasks := []Task{}
	parameters := [][]string{}
	for rows.Next() {
		var task Task

		var commandJSON string
		var argsJSON string
		var paramsJson string

		if err := rows.Scan(&task.Id, &task.Name, &task.Image, &commandJSON, &argsJSON, &paramsJson, &task.ScriptInjectorImage); err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(commandJSON), &task.Command); err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(argsJSON), &task.Args); err != nil {
			return nil, err
		}

		params := []string{}
		if err := json.Unmarshal([]byte(paramsJson), &params); err != nil {
			return nil, err
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

			err := tx.QueryRow(`
			SELECT isSecret, defaultValue
			FROM DAG_Parameters
			WHERE dag_id = ? and name = ?;
			`, dagId, parameter).Scan(&param.IsSecret, &param.Value)

			if err == sql.ErrNoRows {
				continue
			}

			if err != nil {
				return nil, err
			}

			tasks[i].Parameters = append(tasks[i].Parameters, param)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (s *sqliteDAGManager) MarkDAGRunOutcome(ctx context.Context, dagRunId int, outcome string) error {
	return nil
}

func (s *sqliteDAGManager) GetDagParameters(ctx context.Context, dagName string) (map[string]*Parameter, error) {
	return nil, nil
}

func (s *sqliteDAGManager) DagExists(ctx context.Context, dagName string) (bool, error) {
	return false, nil
}

func (s *sqliteDAGManager) ShouldRerun(ctx context.Context, taskRunid int, exitCode int32) (bool, error) {
	return false, nil
}

func (s *sqliteDAGManager) MarkTaskAsFailed(ctx context.Context, taskRunId int) error {
	return nil
}

func (s *sqliteDAGManager) MarkPodStatus(ctx context.Context, podUid types.UID, name string, taskRunID int, status v1.PodPhase, tStamp time.Time, exitCode *int32, namespace string) error {
	return nil
}

func (s *sqliteDAGManager) SoftDeleteDAG(ctx context.Context, name string, namespace string) error {
	return nil
}

func (s *sqliteDAGManager) FindExistingDAGRun(ctx context.Context, name string) (bool, error) {
	return false, nil
}
