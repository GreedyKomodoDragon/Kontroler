package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"kontroler-controller/api/v1alpha1"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	cron "github.com/robfig/cron/v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	log "sigs.k8s.io/controller-runtime/pkg/log"

	_ "modernc.org/sqlite"
)

// sqliteDAGManager manages the SQLite database connection and interactions.
type sqliteDAGManager struct {
	db     *sql.DB
	parser *cron.Parser
}

// SQLiteConfig holds the configurable SQLite settings
type SQLiteConfig struct {
	DBPath      string
	JournalMode string // e.g., "WAL"
	Synchronous string // e.g., "NORMAL" or "FULL"
	CacheSize   int    // e.g., -2000 (for KB, negative to use memory size in KB)
	TempStore   string // e.g., "MEMORY"
}

// NewSqliteManager creates a new SQLite manager with configurable settings
func NewSqliteManager(ctx context.Context, parser *cron.Parser, config *SQLiteConfig) (DBDAGManager, *sql.DB, error) {
	if parser == nil {
		return nil, nil, fmt.Errorf("missing parser")
	}

	// Open a connection to the SQLite database file.
	db, err := sql.Open("sqlite", config.DBPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	// Apply the configurable settings if provided
	if config.JournalMode != "" {
		if _, err := db.Exec(fmt.Sprintf("PRAGMA journal_mode=%s;", config.JournalMode)); err != nil {
			db.Close()
			return nil, nil, fmt.Errorf("failed to set journal mode: %w", err)
		}
	}

	if config.Synchronous != "" {
		if _, err := db.Exec(fmt.Sprintf("PRAGMA synchronous=%s;", config.Synchronous)); err != nil {
			db.Close()
			return nil, nil, fmt.Errorf("failed to set synchronous mode: %w", err)
		}
	}

	if config.CacheSize != 0 {
		if _, err := db.Exec(fmt.Sprintf("PRAGMA cache_size=%d;", config.CacheSize)); err != nil {
			db.Close()
			return nil, nil, fmt.Errorf("failed to set cache size: %w", err)
		}
	}

	if config.TempStore != "" {
		if _, err := db.Exec(fmt.Sprintf("PRAGMA temp_store=%s;", config.TempStore)); err != nil {
			db.Close()
			return nil, nil, fmt.Errorf("failed to set temp store: %w", err)
		}
	}

	// Check the connection to ensure the database is accessible.
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("failed to connect to SQLite database: %w", err)
	}

	return &sqliteDAGManager{
		db:     db,
		parser: parser,
	}, db, nil
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
    namespace VARCHAR(63) NOT NULL,
    active BOOLEAN NOT NULL,
    taskCount INTEGER NOT NULL,
    nexttime TIMESTAMP,
	webhookUrl VARCHAR(255),
	sslVerification BOOL,
	workspaceEnabled BOOL,
    UNIQUE(name, version, namespace)
);

CREATE TABLE IF NOT EXISTS DAG_Workspaces (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    dag_id INTEGER NOT NULL,
    accessModes TEXT,
    selector TEXT,
    resources TEXT,
    storageClassName TEXT,
    volumeMode TEXT,
	FOREIGN KEY (dag_id) REFERENCES DAGs(dag_id)
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
    command TEXT,  -- SQLite does not support arrays; store as TEXT
    args TEXT,
    image VARCHAR(255) NOT NULL,
    parameters TEXT,
    backoffLimit BIGINT NOT NULL,
    isConditional BOOLEAN NOT NULL,
    podTemplate TEXT,  -- Replace JSONB with TEXT or consider JSON1 extension for JSON data
    retryCodes TEXT,   -- Store as TEXT or JSON if array data is needed
    script TEXT NOT NULL,
    scriptInjectorImage TEXT,
	inline BOOL NOT NULL,
	namespace VARCHAR(63) NOT NULL,
	version INTEGER NOT NULL,
	hash VARCHAR(64),
	UNIQUE(name, version, namespace)
);

-- DAG_Tasks table
CREATE TABLE IF NOT EXISTS DAG_Tasks (
	dag_task_id INTEGER PRIMARY KEY AUTOINCREMENT,
    dag_id INTEGER NOT NULL,
    task_id INTEGER NOT NULL,
	name VARCHAR(255) NOT NULL,
	version INTEGER NOT NULL,
    FOREIGN KEY (dag_id) REFERENCES DAGs(dag_id),
    FOREIGN KEY (task_id) REFERENCES Tasks(task_id)
);

-- Dependencies table
CREATE TABLE IF NOT EXISTS Dependencies (
    task_id INTEGER NOT NULL,
    depends_on_task_id INTEGER NOT NULL,
    FOREIGN KEY (task_id) REFERENCES DAG_Tasks(dag_task_id),
    FOREIGN KEY (depends_on_task_id) REFERENCES DAG_Tasks(dag_task_id)
);

-- DAG_Runs table
CREATE TABLE IF NOT EXISTS DAG_Runs (
    run_id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(255) NOT NULL,
    dag_id INTEGER NOT NULL,
    status VARCHAR(255) NOT NULL,
    successfulCount INTEGER NOT NULL,
    failedCount INTEGER NOT NULL,
	suspendedCount INTEGER NOT NULL,
    run_time TIMESTAMP NOT NULL,
	pvcName VARCHAR(255),
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
    FOREIGN KEY (task_id) REFERENCES DAG_Tasks(dag_task_id),
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
    duration INTEGER,
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
	var uniqueID string

	// Try to get an existing unique_id
	err := s.db.QueryRowContext(ctx, "SELECT unique_id FROM IdTable LIMIT 1").Scan(&uniqueID)
	if err == nil {
		return uniqueID, nil
	}

	if err == sql.ErrNoRows {
		newUUID := uuid.New().String()

		_, err = s.db.Exec("INSERT INTO IdTable (unique_id) VALUES (?)", newUUID)
		if err != nil {
			return "", fmt.Errorf("failed to insert new unique_id: %w", err)
		}
		return newUUID, nil
	}

	return "", fmt.Errorf("failed to query IdTable: %w", err)
}

func (s *sqliteDAGManager) GetDAGsToStartAndUpdate(ctx context.Context) ([]*DagInfo, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.Query(`
        SELECT dag_id, name, schedule, namespace, workspaceEnabled
        FROM DAGs
        WHERE nexttime <= datetime('now') AND schedule != '' AND active = 1;
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Collect DAG info and schedules
	namespaces := []*DagInfo{}
	schedules := []string{}
	dagIds := []int{}
	for rows.Next() {
		var dagId int
		var name, schedule, namespace string
		var workEnabled bool

		if err := rows.Scan(&dagId, &name, &schedule, &namespace, &workEnabled); err != nil {
			return nil, err
		}

		namespaces = append(namespaces, &DagInfo{
			DagName:          name,
			Namespace:        namespace,
			WorkspaceEnabled: workEnabled,
		})
		schedules = append(schedules, schedule)
		dagIds = append(dagIds, dagId)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	// TODO: bath update nexttime for all DAGs
	for i, schedule := range schedules {
		// Parse the cron expression
		sched, err := s.parser.Parse(schedule)
		if err != nil {
			return nil, err
		}

		// Calculate the next occurrence
		nextTime := sched.Next(time.Now())

		// Update the nextTime for each DAG
		_, err = tx.Exec(`
			UPDATE DAGs 
			SET nexttime = ? 
			WHERE dag_id = ?;
		`, nextTime, dagIds[i])
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return namespaces, nil
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

	err = tx.QueryRowContext(ctx, `
	SELECT dag_id, version, hash
	FROM DAGs
	WHERE name = ? AND namespace = ?
	ORDER BY version DESC;`, dag.Name, namespace).Scan(&existingDAGID, &version, &hash)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	hashBytes, err := hashDagSpec(&dag.Spec)
	if err != nil {
		return err
	}

	hashValue := fmt.Sprintf("%x", hashBytes)
	if hash == hashValue {
		return fmt.Errorf("applying the same dag")
	}

	if existingDAGID != 0 {
		version++
	}

	// DAG does not exist, insert it
	if err := s.insertDAG(ctx, tx, dag, version, namespace, hashValue); err != nil {
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
func (s *sqliteDAGManager) insertDAG(ctx context.Context, tx *sql.Tx, dag *v1alpha1.DAG, version int, namespace string, hash string) error {

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
	if err := tx.QueryRowContext(ctx, `
	INSERT INTO DAGs (name, version, hash, schedule, namespace, active, nexttime, taskCount, webhookUrl, sslVerification) 
	VALUES (?, ?, ?, ?, ?, TRUE, ?, ?, ?, ?)
	RETURNING dag_id`, dag.Name, version, hash, dag.Spec.Schedule, namespace, nextTime, len(dag.Spec.Task), dag.Spec.Webhook.URL, dag.Spec.Webhook.VerifySSL).Scan(&dagID); err != nil {
		return err
	}

	// only insert workspace if enabled
	if dag.Spec.Workspace.Enabled {
		if err := s.insertWorkspace(ctx, tx, dagID, &dag.Spec.Workspace.PvcSpec); err != nil {
			return fmt.Errorf("failed to insert workspace: %w", err)
		}
	}

	// Insert tasks and map them to the DAG
	for _, task := range dag.Spec.Task {
		version := getTaskVersion(&task)

		if err := s.insertTask(ctx, tx, dagID, &task, namespace, version); err != nil {
			return err
		}
	}

	// After all tasks are inserted, handle dependencies
	for _, task := range dag.Spec.Task {
		version := getTaskVersion(&task)

		if err := s.createDependencyConnection(ctx, tx, dagID, &task, version); err != nil {
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

func (s *sqliteDAGManager) insertWorkspace(ctx context.Context, tx *sql.Tx, dagID int, workspace *v1alpha1.PVC) error {
	accessModesJSON, err := json.Marshal(workspace.AccessModes)
	if err != nil {
		return err
	}

	selectorJSON, err := json.Marshal(workspace.Selector)
	if err != nil {
		return err
	}

	resourcesJSON, err := json.Marshal(workspace.Resources)
	if err != nil {
		return err
	}

	volumeModeJSON, err := json.Marshal(workspace.VolumeMode)
	if err != nil {
		return err
	}

	// Insert the workspace
	if _, err := tx.ExecContext(ctx, `
	INSERT INTO DAG_Workspaces (dag_id, accessModes, selector, resources, storageClassName, volumeMode) 
	VALUES (?, ?, ?, ?, ?, ?);`, dagID, accessModesJSON, selectorJSON, resourcesJSON, workspace.StorageClassName, volumeModeJSON); err != nil {
		return err
	}

	return nil
}

func (s *sqliteDAGManager) insertTask(ctx context.Context, tx *sql.Tx, dagID int, task *v1alpha1.TaskSpec, namespace string, version int) error {
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

	var taskId int
	inline := task.TaskRef == nil
	if !inline {
		err := tx.QueryRowContext(ctx, `
		SELECT task_id FROM Tasks
		WHERE name = ? AND inline = FALSE and version = ?;`, task.TaskRef.Name, task.TaskRef.Version).Scan(&taskId)
		if err != nil {
			return err
		}

	} else {
		// must provide a unique name - name is used not used for in-line and must just be unique
		newUUID := uuid.New()

		if err := tx.QueryRowContext(ctx, `
		INSERT INTO Tasks (name, command, args, image, parameters, backoffLimit, isConditional, retryCodes, podTemplate, script, scriptInjectorImage, inline, namespace, version) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, TRUE, ?, ?) 
		RETURNING task_id;`,
			newUUID.String(), commandJson, argsJson, task.Image, paramsJson, task.Backoff.Limit,
			task.Conditional.Enabled, retryCodesJson, jsonValue, task.Script, task.ScriptInjectorImage, namespace, version).Scan(&taskId); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO DAG_Tasks (dag_id, task_id, name, version)
		VALUES (?, ?, ?, ?);`, dagID, taskId, task.Name, version); err != nil {
		return err
	}

	return nil
}

func (s *sqliteDAGManager) createDependencyConnection(ctx context.Context, tx *sql.Tx, dagID int, task *v1alpha1.TaskSpec, version int) error {
	for _, dependency := range task.RunAfter {
		var taskId, depId int

		err := tx.QueryRowContext(ctx, `
		SELECT dag_task_id
		FROM DAG_Tasks 
		WHERE dag_id = ? AND name = ? AND version = ?;
		`, dagID, task.Name, version).Scan(&taskId)
		if err != nil {
			return fmt.Errorf("task: %s not found for version %d", task.Name, version)
		}

		err = tx.QueryRowContext(ctx, `
		SELECT dag_task_id
		FROM DAG_Tasks 
		WHERE dag_id = ? AND name = ?
		ORDER BY version DESC
		LIMIT 1;
		;`, dagID, dependency).Scan(&depId)
		if err != nil {
			if err == pgx.ErrNoRows {
				return fmt.Errorf("dependency task %s not found for version %d", dependency, version)
			}
			return err
		}

		if _, err := tx.ExecContext(ctx, `
		INSERT INTO Dependencies (task_id, depends_on_task_id) 
		VALUES (?, ?);`, taskId, depId); err != nil {
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

func (s *sqliteDAGManager) CreateDAGRun(ctx context.Context, name string, dag *v1alpha1.DagRunSpec, parameters map[string]v1alpha1.ParameterSpec, pvcName *string) (int, error) {
	dagId, err := s.dagNameToDagId(ctx, dag.DagName)
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
	if err := tx.QueryRowContext(ctx, `
	INSERT INTO DAG_Runs (dag_id, name, status, successfulCount, failedCount, suspendedCount, run_time, pvcName) 
	VALUES (?, ?, 'running', 0, 0, 0, datetime('now'), ?) 
	RETURNING run_id`, dagId, name, pvcName).Scan(&dagRunID); err != nil {
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

func (s *sqliteDAGManager) dagNameToDagId(ctx context.Context, dagName string) (int, error) {
	dagId := -1
	if err := s.db.QueryRowContext(ctx, `
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

func (s *sqliteDAGManager) GetStartingTasks(ctx context.Context, dagName string, dagrun int) ([]Task, error) {
	rows, err := s.db.Query(`
	SELECT 
		dt.dag_task_id,
		dt.name, 
		t.image, 
		t.command, 
		t.args, 
		t.parameters, 
		t.podTemplate, 
		dt.dag_id, 
		t.script,
		dr.pvcName
	FROM 
		Tasks t
	JOIN 
		DAG_Tasks dt ON t.task_id = dt.task_id
	JOIN 
        DAG_Runs dr ON dt.dag_id = dr.dag_id
	LEFT JOIN 
		Dependencies d ON dt.dag_task_id = d.task_id
	LEFT JOIN 
		DAG_Tasks dat ON dat.dag_task_id = d.depends_on_task_id
	WHERE 
		d.depends_on_task_id IS NULL  -- Ensure tasks with no dependencies
		AND dt.dag_id = (
			SELECT dag_id
			FROM DAGs
			WHERE name = ?
			ORDER BY version DESC
			LIMIT 1
		)
		AND dr.run_id = ?;
	`, dagName, dagrun)

	if err != nil {
		return nil, fmt.Errorf("failed to get tasks: %v", err)
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Log.Error(err, "failed to close row")
		}
	}()

	tasks := []Task{}
	for rows.Next() {
		task := Task{}
		var podTemplateJSON *string
		var dagId int

		// Needed as stored as TEXT and not []TEXT
		var commandJSON string
		var argsJSON string
		var paramJSON string
		var pvcName sql.NullString

		if err := rows.Scan(&task.Id, &task.Name, &task.Image, &commandJSON, &argsJSON, &paramJSON, &podTemplateJSON, &dagId, &task.Script, &pvcName); err != nil {
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

			if err := s.db.QueryRowContext(ctx, `
			SELECT isSecret, defaultValue
			FROM DAG_Parameters
			WHERE dag_id = ? and name = ?;
			`, dagId, parameter).Scan(&param.IsSecret, &param.Value); err != nil {
				return nil, fmt.Errorf("failed to get parameter '%s': %v", parameter, err)
			}

			task.Parameters = append(task.Parameters, param)
		}

		var podTemplate *v1alpha1.PodTemplateSpec
		if podTemplateJSON != nil {
			if err := json.Unmarshal([]byte(*podTemplateJSON), &podTemplate); err != nil {
				return nil, err
			}
		} else {
			podTemplate = &v1alpha1.PodTemplateSpec{}
		}

		if pvcName.Valid {
			podTemplate.Volumes = append(podTemplate.Volumes, v1.Volume{
				Name: "workspace",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvcName.String,
					},
				},
			})

			podTemplate.VolumeMounts = append(podTemplate.VolumeMounts, v1.VolumeMount{
				Name:      "workspace",
				MountPath: "/workspace",
			})
		}

		task.PodTemplate = podTemplate

		tasks = append(tasks, task)

	}

	return tasks, nil
}

func (s *sqliteDAGManager) MarkTaskAsStarted(ctx context.Context, runId, taskId int) (int, error) {
	var taskRunId int

	if err := s.db.QueryRowContext(ctx, `
	INSERT INTO Task_Runs (run_id, task_id, status, attempts) 
	VALUES (?, ?, 'running', 1) 
	RETURNING task_run_id`,
		runId, taskId).Scan(&taskRunId); err != nil {
		return 0, err
	}

	return taskRunId, nil
}

func (s *sqliteDAGManager) IncrementAttempts(ctx context.Context, taskRunId int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	if _, err := tx.Exec(`
	UPDATE Task_Runs 
	SET attempts = attempts + 1
	WHERE task_run_id = ?
	`, taskRunId); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *sqliteDAGManager) MarkSuccessAndGetNextTasks(ctx context.Context, taskRunId int) ([]Task, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var runId int
	err = tx.QueryRowContext(ctx, `
	UPDATE Task_Runs 
	SET status = 'success' 
	WHERE task_run_id = ? 
	RETURNING run_id`, taskRunId).Scan(&runId)
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
	err = tx.QueryRowContext(ctx, `
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

	dagId, err := s.getDAGIdFromRun(ctx, tx, runId)
	if err != nil {
		return nil, err
	}

	tasks, parameters, err := s.getNextRunnableTasks(ctx, tx, taskRunId, runId, dagId)
	if err != nil {
		return nil, err
	}

	if err := s.fetchTaskParameters(ctx, tx, dagId, tasks, parameters); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (s *sqliteDAGManager) getDAGIdFromRun(ctx context.Context, tx *sql.Tx, runId int) (int, error) {
	var dagId int
	err := tx.QueryRowContext(ctx, `
		SELECT dag_id
		FROM dag_runs
		WHERE run_id = ?
	`, runId).Scan(&dagId)

	return dagId, err
}

func (s *sqliteDAGManager) getNextRunnableTasks(ctx context.Context, tx *sql.Tx, taskRunId, runId int, dagId int) ([]Task, [][]string, error) {
	dependencyCounts, err := s.getDependencyCounts(ctx, tx, dagId)
	if err != nil {
		return nil, nil, err
	}

	metDependencies, err := s.getMetDependencies(ctx, tx, dagId, runId)
	if err != nil {
		return nil, nil, err
	}

	runnableTasks, err := s.getRunnableTasks(ctx, tx, dependencyCounts, metDependencies, taskRunId)
	if err != nil {
		return nil, nil, err
	}

	return s.getTasksByIds(ctx, tx, runnableTasks, runId)
}

func (s *sqliteDAGManager) getTasksByIds(ctx context.Context, tx *sql.Tx, taskIds []int, runId int) ([]Task, [][]string, error) {
	params := make([]string, 0, len(taskIds))
	args := make([]interface{}, 0, len(taskIds)+1)
	args = append(args, runId)

	for _, id := range taskIds {
		params = append(params, "?")
		args = append(args, id)
	}
	query := fmt.Sprintf(`
		SELECT dat.dag_task_id, dat.name, t.image, t.command, t.args, t.parameters, t.scriptInjectorImage, t.script, t.podTemplate, dr.pvcName
		FROM Tasks t
		JOIN DAG_Tasks dat ON dat.task_id = t.task_id
		JOIN DAG_Runs dr ON dat.dag_id = dr.dag_id
		WHERE 
			dr.run_id = ?
		AND 
			dat.dag_task_id IN (%s)`, strings.Join(params, ","))

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	tasks := make([]Task, 0, len(taskIds))
	parameters := make([][]string, 0, len(taskIds))

	for rows.Next() {
		var task Task
		var commandJSON string
		var argsJSON string
		var paramsJson string
		var podTemplateJSON *string
		var pvcName sql.NullString

		if err := rows.Scan(&task.Id, &task.Name, &task.Image, &commandJSON, &argsJSON, &paramsJson, &task.ScriptInjectorImage, &task.Script, &podTemplateJSON, &pvcName); err != nil {
			return nil, nil, err
		}

		if err := json.Unmarshal([]byte(commandJSON), &task.Command); err != nil {
			return nil, nil, err
		}

		if err := json.Unmarshal([]byte(argsJSON), &task.Args); err != nil {
			return nil, nil, err
		}

		params := []string{}
		if err := json.Unmarshal([]byte(paramsJson), &params); err != nil {
			return nil, nil, err
		}

		parameters = append(parameters, params)

		var podTemplate *v1alpha1.PodTemplateSpec
		if podTemplateJSON != nil {
			if err := json.Unmarshal([]byte(*podTemplateJSON), &podTemplate); err != nil {
				return nil, nil, err
			}
		} else {
			podTemplate = &v1alpha1.PodTemplateSpec{}
		}

		if pvcName.Valid {
			podTemplate.Volumes = append(podTemplate.Volumes, v1.Volume{
				Name: "workspace",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvcName.String,
					},
				},
			})

			podTemplate.VolumeMounts = append(podTemplate.VolumeMounts, v1.VolumeMount{
				Name:      "workspace",
				MountPath: "/workspace",
			})
		}

		task.PodTemplate = podTemplate

		tasks = append(tasks, task)
	}
	return tasks, parameters, nil
}

func (s *sqliteDAGManager) getRunnableTasks(ctx context.Context, tx *sql.Tx, dependencyCounts, metDependencies map[int]int, taskRunId int) ([]int, error) {
	var runnableTasks []int

	for taskId, totalDeps := range dependencyCounts {
		metDeps := metDependencies[taskId]
		if totalDeps != metDeps {
			continue
		}
		var taskStatus string
		err := tx.QueryRowContext(ctx, `
                SELECT status
                FROM Task_Runs
                WHERE task_id = ? AND run_id = ?;
            `, taskId, taskRunId).Scan(&taskStatus)

		if err == sql.ErrNoRows {
			runnableTasks = append(runnableTasks, taskId)
			continue
		} else if err != nil {
			return nil, err
		}
	}

	return runnableTasks, nil
}

func (s *sqliteDAGManager) getMetDependencies(ctx context.Context, tx *sql.Tx, dagId, runID int) (map[int]int, error) {
	// Query to get the count of met dependencies for tasks in the same DAG and not already started/completed
	rows, err := tx.QueryContext(ctx, `
		SELECT d.task_id, COUNT(d.depends_on_task_id)
		FROM Dependencies d
		JOIN Task_Runs tr ON d.depends_on_task_id = tr.task_id
		WHERE tr.status = 'success'
		AND d.task_id IN (
			SELECT dag_task_id
			FROM DAG_Tasks 
			WHERE dag_id = ?
		)
		AND d.task_id NOT IN (
			SELECT task_id 
			FROM Task_Runs 
			WHERE
				status IN ('running', 'success', 'failed')
			AND run_id = ?
		)
		AND tr.run_id = ?
		GROUP BY d.task_id`, dagId, runID, runID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Map to store met dependency counts for each task
	metDependencies := make(map[int]int)
	for rows.Next() {
		var taskId, metDeps int
		if err := rows.Scan(&taskId, &metDeps); err != nil {
			return nil, err
		}
		metDependencies[taskId] = metDeps
	}

	return metDependencies, nil
}

func (s *sqliteDAGManager) getDependencyCounts(ctx context.Context, tx *sql.Tx, dagId int) (map[int]int, error) {
	// Query to get the total dependencies for tasks associated with the given DAG
	rows, err := tx.QueryContext(ctx, `
		SELECT d.task_id, COUNT(d.depends_on_task_id)
		FROM Dependencies d
		JOIN DAG_Tasks dt ON d.task_id = dt.dag_task_id
		WHERE dt.dag_id = ?
		GROUP BY d.task_id`, dagId)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Map to store dependency counts for each task
	dependencyCounts := make(map[int]int)
	for rows.Next() {
		var taskId, totalDependencies int
		if err := rows.Scan(&taskId, &totalDependencies); err != nil {
			return nil, err
		}
		dependencyCounts[taskId] = totalDependencies
	}

	return dependencyCounts, nil
}

func (s *sqliteDAGManager) fetchTaskParameters(ctx context.Context, tx *sql.Tx, dagId int, tasks []Task, parameters [][]string) error {
	for i := 0; i < len(tasks); i++ {
		tasks[i].Parameters = []Parameter{}
		for _, parameter := range parameters[i] {
			param := Parameter{Name: parameter}
			err := tx.QueryRowContext(ctx, `
				SELECT isSecret, defaultValue
				FROM DAG_Parameters
				WHERE dag_id = ? and name = ?;
			`, dagId, parameter).Scan(&param.IsSecret, &param.Value)

			if err == sql.ErrNoRows {
				continue
			}

			if err != nil {
				return err
			}

			tasks[i].Parameters = append(tasks[i].Parameters, param)
		}
	}
	return nil
}

func (s *sqliteDAGManager) MarkDAGRunOutcome(ctx context.Context, dagRunId int, outcome string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	if _, err := tx.Exec("UPDATE DAG_Runs SET status = ? WHERE run_id = ?;", outcome, dagRunId); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *sqliteDAGManager) GetDagParameters(ctx context.Context, dagName string) (map[string]*Parameter, error) {
	rows, err := s.db.Query(`
	SELECT name, isSecret, defaultValue
	FROM DAG_Parameters
	WHERE dag_id IN (
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

func (s *sqliteDAGManager) DagExists(ctx context.Context, dagName string) (bool, int, error) {
	dagId := -1
	if err := s.db.QueryRowContext(ctx, `
		SELECT dag_id
		FROM DAGs
		WHERE name = ?
	`, dagName).Scan(&dagId); err != nil && err != sql.ErrNoRows {
		return false, -1, err
	}

	return dagId != -1, dagId, nil
}

func (s *sqliteDAGManager) ShouldRerun(ctx context.Context, taskRunID int, exitCode int32) (bool, error) {
	// Due to the SQLite Driver not supporting JSON we need to check in the go code
	// Using non CGO driver but may have to convert over at some point

	query := `
	SELECT t.backoffLimit, t.isConditional, t.retryCodes, r.attempts
	FROM tasks t
	JOIN DAG_Tasks dt ON t.task_id = dt.task_id
    JOIN Task_Runs r ON dt.dag_task_id = r.task_id
	WHERE r.task_run_id = ?;
	`

	row := s.db.QueryRowContext(ctx, query, taskRunID)

	var backoffLimit int
	var isConditional bool
	var retryCodes string
	var attempts int

	// Scan the result into variables
	err := row.Scan(&backoffLimit, &isConditional, &retryCodes, &attempts)
	if err != nil {
		if err == sql.ErrNoRows {
			// No matching rows, rerun is not needed
			return false, nil
		}
		return false, fmt.Errorf("failed to execute query: %w", err)
	}

	// Perform the check in Go
	if attempts > backoffLimit {
		return false, nil
	}

	if isConditional {
		var codes []int32
		if err := json.Unmarshal([]byte(retryCodes), &codes); err != nil {
			return false, fmt.Errorf("failed to parse retry codes: %w", err)
		}

		for _, code := range codes {
			if code == exitCode {
				return true, nil
			}
		}
		return false, nil
	}

	return true, nil
}

func (s *sqliteDAGManager) MarkTaskAsFailed(ctx context.Context, taskRunId int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	if _, err := tx.Exec(`
		UPDATE Task_Runs 
		SET status = 'failed' 
		WHERE task_run_id = ? ;
	`, taskRunId); err != nil {
		return err
	}

	if _, err := tx.Exec(`
	    UPDATE DAG_Runs
	    SET
	        failedCount = failedCount + 1,
	        status = 'failed'
	    WHERE run_id in (
			SELECT run_id
			FROM Task_Runs
			WHERE task_run_id = ?
		);`, taskRunId); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *sqliteDAGManager) MarkPodStatus(ctx context.Context, podUid types.UID, name string, taskRunID int, status v1.PodPhase, tStamp time.Time, exitCode *int32, namespace string) error {
	// Begin a transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check for existing record and retrieve the current status and timestamp
	var currentTimestamp time.Time
	err = tx.QueryRowContext(ctx, `
        SELECT updated_at FROM Task_Pods WHERE Pod_UID = ? AND task_run_id = ?
    `, podUid, taskRunID).Scan(&currentTimestamp)

	if err != nil && err != sql.ErrNoRows {
		// Return if any error other than "no rows" occurs
		return err
	}

	// Decide whether to insert or update
	if err == sql.ErrNoRows {
		// No existing row, perform an INSERT
		_, err = tx.ExecContext(ctx, `
            INSERT INTO Task_Pods (Pod_UID, task_run_id, name, status, namespace, updated_at, exitCode)
            VALUES (?, ?, ?, ?, ?, ?, ?)
        `, podUid, taskRunID, name, status, namespace, tStamp, exitCode)
		if err != nil {
			return err
		}
	} else {
		// Existing row has an older timestamp, perform an UPDATE
		_, err = tx.ExecContext(ctx, `
            UPDATE Task_Pods 
            SET status = ?, updated_at = ?, exitCode = ?
            WHERE Pod_UID = ? AND task_run_id = ?
        `, status, tStamp, exitCode, podUid, taskRunID)
		if err != nil {
			return err
		}
	}

	// Commit the transaction
	return tx.Commit()
}

func (s *sqliteDAGManager) getTaskDeletionData(ctx context.Context, tx *sql.Tx, name, namespace string) ([]taskData, error) {
	// Check for tasks associated with the specified DAG
	rows, err := tx.QueryContext(ctx, `
	SELECT DISTINCT(t.task_id), t.name, t.namespace
	FROM Tasks t
	JOIN DAG_Tasks dt ON t.task_id = dt.task_id
	JOIN DAGs d ON d.dag_id = dt.dag_id
	WHERE d.name = ? AND d.namespace = ? AND t.inline = FALSE;
	`, name, namespace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	taskDatas := []taskData{}
	for rows.Next() {
		var taskID int
		var taskName, taskNamespace string
		if err := rows.Scan(&taskID, &taskName, &taskNamespace); err != nil {
			return nil, err
		}

		taskDatas = append(taskDatas, taskData{TaskID: taskID, TaskName: taskName, TaskNamespace: taskNamespace})
	}

	return taskDatas, nil
}

func (s *sqliteDAGManager) DeleteDAG(ctx context.Context, name string, namespace string) ([]string, error) {
	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	// Rollback transaction if not committed
	defer tx.Rollback()

	taskData, err := s.getTaskDeletionData(ctx, tx, name, namespace)
	if err != nil {
		return nil, err
	}

	// Check if each task is still associated with other DAGs
	var unusedTaskNames []string
	for _, task := range taskData {
		var count int
		err = tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM (
				SELECT DISTINCT d.name, d.namespace
				FROM DAG_Tasks dt
				JOIN DAGs d ON dt.dag_id = d.dag_id
				WHERE 
					dt.task_id in (
						SELECT task_id
						FROM tasks
						WHERE name = ? and namespace = ?
					) 
					AND NOT(d.name = ? AND d.namespace = ?)
			) AS distinct_combinations;
			`, task.TaskName, namespace, name, namespace).Scan(&count)
		if err != nil {
			return nil, err
		}

		// Add tasks that are no longer connected to any DAG
		if count == 0 {
			unusedTaskNames = append(unusedTaskNames, task.TaskName)
		}
	}

	// Delete the associated DAG_Run entries first
	_, err = tx.ExecContext(ctx, `
		DELETE FROM DAG_Run_Parameters
		WHERE run_id IN (
			SELECT run_id FROM DAG_Runs WHERE dag_id IN (SELECT dag_id FROM DAGs WHERE name = ? AND namespace = ?)
		);
		`, name, namespace)
	if err != nil {
		return nil, err
	}

	_, err = tx.ExecContext(ctx, `
		DELETE FROM Task_Runs
		WHERE run_id IN (
			SELECT run_id FROM DAG_Runs WHERE dag_id IN (SELECT dag_id FROM DAGs WHERE name = ? AND namespace = ?)
		);
		`, name, namespace)
	if err != nil {
		return nil, err
	}

	// Now delete any tasks associated with inline tasks in the DAG
	rowsTasks, err := tx.QueryContext(ctx, `
		SELECT t.task_id
		FROM Tasks t
		JOIN DAG_Tasks dt ON dt.task_id = t.task_id
		LEFT JOIN DAGs d ON dt.dag_id = d.dag_id
		WHERE d.name = ? AND t.inline = TRUE;
		`, name)
	if err != nil {
		return nil, err
	}
	defer rowsTasks.Close()

	taskIds := []interface{}{}
	placeholders := []string{}
	i := 0
	for rowsTasks.Next() {
		var taskId int
		if err := rowsTasks.Scan(&taskId); err != nil {
			return nil, err
		}

		taskIds = append(taskIds, taskId)
		placeholders = append(placeholders, "?")
		i++
	}

	_, err = tx.ExecContext(ctx, `
		DELETE FROM DAG_Runs
		WHERE dag_id IN (SELECT dag_id FROM DAGs WHERE name = ? AND namespace = ?)
		`, name, namespace)
	if err != nil {
		return nil, err
	}

	// Now, delete DAG_Tasks references to the DAG
	_, err = tx.ExecContext(ctx, `
		DELETE FROM DAG_Tasks
		WHERE dag_id IN (SELECT dag_id FROM DAGs WHERE name = ? AND namespace = ?)
		`, name, namespace)
	if err != nil {
		return nil, err
	}

	// Now, delete the DAG itself
	_, err = tx.ExecContext(ctx, `
		DELETE FROM DAGs
		WHERE name = ? AND namespace = ?;
		`, name, namespace)
	if err != nil {
		return nil, err
	}

	// Optionally delete tasks that are no longer in use
	if len(taskIds) > 0 {
		query := fmt.Sprintf(`
			DELETE FROM Tasks
			WHERE task_id IN (%s);`, strings.Join(placeholders, ","))
		if _, err := tx.ExecContext(ctx, query, taskIds...); err != nil {
			return nil, err
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return unusedTaskNames, nil

}

func (s *sqliteDAGManager) FindExistingDAGRun(ctx context.Context, name string) (bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `
    SELECT EXISTS (
        SELECT 1
        FROM DAG_Runs
        WHERE name = ?
    );
	`, name).Scan(&exists); err != nil && err != sql.ErrNoRows {
		return false, err
	}

	return exists, nil
}

func (s *sqliteDAGManager) GetTaskScriptAndInjectorImage(ctx context.Context, taskId int) (*string, *string, error) {
	var script *string
	var injectorImage *string

	if err := s.db.QueryRowContext(ctx, `
	SELECT t.script, t.scriptInjectorImage
	FROM Tasks t
	WHERE t.task_id = (
		SELECT task_id
		FROM DAG_Tasks
		WHERE dag_task_id = ?
		);
	`, taskId).Scan(&script, &injectorImage); err != nil {
		return nil, nil, err
	}

	return script, injectorImage, nil
}

func (s *sqliteDAGManager) AddTask(ctx context.Context, task *v1alpha1.DagTask, namespace string) error {
	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	// Rollback transaction if not committed
	defer tx.Rollback()

	var taskId int
	var version int
	var hash *string

	err = tx.QueryRowContext(ctx, `
	SELECT task_id, version, hash
	FROM Tasks
	WHERE name = ? AND namespace = ?
	ORDER BY version DESC;`, task.Name, namespace).Scan(&taskId, &version, &hash)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	hashBytes, err := hashDagTaskSpec(&task.Spec)
	if err != nil {
		return err
	}

	hashValue := fmt.Sprintf("%x", hashBytes)

	if hash != nil && *hash == hashValue {
		return fmt.Errorf("applying the same task")
	}

	var jsonValue *string
	if task.Spec.PodTemplate != nil {
		json, err := task.Spec.PodTemplate.Serialize()
		if err != nil {
			return err
		}

		jsonValue = &json
	}

	// SQLite has no slice/array type so we need to convert it to a JSON string
	commandJson, err := json.Marshal(task.Spec.Command)
	if err != nil {
		return err
	}

	argsJson, err := json.Marshal(task.Spec.Args)
	if err != nil {
		return err
	}

	paramsJson, err := json.Marshal(task.Spec.Parameters)
	if err != nil {
		return err
	}

	retryCodesJson, err := json.Marshal(task.Spec.Conditional.RetryCodes)
	if err != nil {
		return err
	}

	newVersion := version + 1

	if _, err := tx.ExecContext(ctx, `
    INSERT INTO Tasks (name, command, args, image, parameters, backoffLimit, isConditional, retryCodes, podTemplate, script, scriptInjectorImage, inline, namespace, version, hash)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, FALSE, ?, ?, ?);`,
		task.Name, commandJson, argsJson, task.Spec.Image, paramsJson, task.Spec.Backoff.Limit,
		task.Spec.Conditional.Enabled, retryCodesJson, jsonValue, task.Spec.Script, task.Spec.ScriptInjectorImage, namespace, newVersion, hashValue); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *sqliteDAGManager) GetTaskRefsParameters(ctx context.Context, taskRefs []v1alpha1.TaskRef) (map[v1alpha1.TaskRef][]string, error) {
	taskMp := map[v1alpha1.TaskRef][]string{}

	querySql := `
		SELECT parameters
		FROM Tasks
		WHERE name = ? AND version = ? AND inline = FALSE;
    `

	for _, val := range taskRefs {
		var paramsJson string
		if err := s.db.QueryRowContext(ctx, querySql, val.Name, val.Version).Scan(&paramsJson); err != nil {
			return nil, err
		}

		var parameters []string

		if err := json.Unmarshal([]byte(paramsJson), &parameters); err != nil {
			return nil, err
		}

		taskMp[val] = parameters
	}

	return taskMp, nil
}

func (s *sqliteDAGManager) DeleteTask(ctx context.Context, taskName, namespace string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	if _, err := tx.Exec(`
		DELETE FROM Tasks
		WHERE
			name = ?
		AND namespace = ?
		AND inline = FALSE;
	`, taskName, namespace); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *sqliteDAGManager) GetWebhookDetails(ctx context.Context, dagRunID int) (*v1alpha1.Webhook, error) {
	webhook := &v1alpha1.Webhook{}

	err := s.db.QueryRowContext(ctx, `
	SELECT webhookUrl, sslVerification
	FROM DAGs
	WHERE dag_id = (
		SELECT dag_id
		FROM DAG_Runs
		WHERE run_id = ?
	);
	`, dagRunID).Scan(&webhook.URL, &webhook.VerifySSL)
	if err != nil {
		return nil, err
	}

	return webhook, nil
}

// CREATE TABLE IF NOT EXISTS DAG_Workspaces (
//     id INTEGER PRIMARY KEY AUTOINCREMENT,
//     dag_id INTEGER NOT NULL,
//     accessModes TEXT[],
//     selector TEXT,
//     resources TEXT,
//     storageClassName TEXT,
//     volumeMode TEXT,
// 	FOREIGN KEY (dag_id) REFERENCES DAGs(dag_id)
// );

func (s *sqliteDAGManager) GetWorkspacePVCTemplate(ctx context.Context, dagId int) (*v1alpha1.PVC, error) {
	pvc := &v1alpha1.PVC{}

	var selectorJSON, resourcesJSON, accessModesJSON, volumeModeJSON sql.NullString

	err := s.db.QueryRowContext(ctx, `
    SELECT accessModes, selector, storageClassName, volumeMode, resources
    FROM DAG_Workspaces
    WHERE dag_id = ?;
    `, dagId).Scan(&accessModesJSON, &selectorJSON, &pvc.StorageClassName, &volumeModeJSON, &resourcesJSON)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if accessModesJSON.Valid {
		if err := json.Unmarshal([]byte(accessModesJSON.String), &pvc.AccessModes); err != nil {
			return nil, err
		}
	}

	if volumeModeJSON.Valid {
		if err := json.Unmarshal([]byte(volumeModeJSON.String), &pvc.VolumeMode); err != nil {
			return nil, err
		}
	}

	if selectorJSON.Valid {
		if err := json.Unmarshal([]byte(selectorJSON.String), &pvc.Selector); err != nil {
			return nil, err
		}
	}

	if resourcesJSON.Valid {
		if err := json.Unmarshal([]byte(resourcesJSON.String), &pvc.Resources); err != nil {
			return nil, err
		}
	}

	return pvc, nil
}

func (s *sqliteDAGManager) MarkConnectingTasksAsSuspended(ctx context.Context, dagRunId, taskRunId int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Fetch all dependencies for the given DAG
	rows, err := tx.QueryContext(ctx, `
		SELECT d.depends_on_task_id, d.task_id
		FROM Dependencies d
		JOIN DAG_Tasks dt ON d.depends_on_task_id = dt.dag_task_id
		WHERE dt.dag_id = (
			SELECT dag_id FROM DAG_Runs WHERE run_id = ?
		);
	`, dagRunId)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Store dependencies in a map
	dependencies := make(map[int][]int)
	for rows.Next() {
		var parentTaskID, dependentTaskID int
		if err := rows.Scan(&parentTaskID, &dependentTaskID); err != nil {
			return fmt.Errorf("failed to scan dependency row: %w", err)
		}
		dependencies[parentTaskID] = append(dependencies[parentTaskID], dependentTaskID)
	}

	// Get the starting task_id from task_run_id
	var startingTaskID int
	if err := tx.QueryRowContext(ctx, `
		SELECT task_id FROM Task_Runs WHERE task_run_id = ?;
	`, taskRunId).Scan(&startingTaskID); err != nil {
		return fmt.Errorf("failed to get starting task ID: %w", err)
	}

	// DFS using stack
	stack := []int{startingTaskID}
	seen := make(map[int]bool)
	var updates [][]interface{}
	uniqueUpdates := make(map[int]struct{})

	for len(stack) > 0 {
		currentTaskID := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if seen[currentTaskID] {
			continue
		}
		seen[currentTaskID] = true

		if dependentTasks, exists := dependencies[currentTaskID]; exists {
			for _, taskID := range dependentTasks {
				if _, exists := uniqueUpdates[taskID]; !exists {
					updates = append(updates, []interface{}{dagRunId, taskID, "suspended", 0})
					uniqueUpdates[taskID] = struct{}{}
					stack = append(stack, taskID)
				}
			}
		}
	}

	// Batch Insert using "INSERT OR IGNORE"
	if len(updates) > 0 {
		stmt, err := tx.PrepareContext(ctx, `
			INSERT OR IGNORE INTO Task_Runs (run_id, task_id, status, attempts) 
			VALUES (?, ?, ?, ?);
		`)
		if err != nil {
			return fmt.Errorf("failed to prepare batch insert: %w", err)
		}
		defer stmt.Close()

		for _, update := range updates {
			if _, err := stmt.ExecContext(ctx, update...); err != nil {
				return fmt.Errorf("failed to execute batch insert: %w", err)
			}
		}
	}

	// Update DAG Runs count
	if _, err := tx.ExecContext(ctx, `
		UPDATE DAG_Runs 
		SET suspendedCount = suspendedCount + ?
		WHERE run_id = ?;`, len(updates), dagRunId); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *sqliteDAGManager) CheckIfAllTasksDone(ctx context.Context, dagRunID int) (bool, error) {
	var taskCount, successCount, failedCount, suspendedCount int
	err := s.db.QueryRowContext(ctx, `
        SELECT 
            (SELECT COUNT(*) FROM DAG_Tasks WHERE dag_id = dr.dag_id) as task_count,
            dr.successfulCount,
            dr.failedCount,
            dr.suspendedCount
        FROM DAG_Runs dr
        WHERE dr.run_id = ?;
    `, dagRunID).Scan(&taskCount, &successCount, &failedCount, &suspendedCount)
	if err != nil {
		return false, err
	}

	return taskCount == successCount+failedCount+suspendedCount, nil
}

func (p *sqliteDAGManager) AddPodDuration(ctx context.Context, taskRunId int, durationSec int64) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		UPDATE Task_Pods
		SET duration = ?
		WHERE task_run_id = ?;
	`, durationSec, taskRunId); err != nil {
		return err
	}

	return tx.Commit()
}
