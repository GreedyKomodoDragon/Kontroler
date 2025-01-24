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
	"github.com/jackc/pgx/v5/pgxpool"
	cron "github.com/robfig/cron/v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	_ "embed"
)

var (
	//go:embed postgresSchema.sql
	progresSchema string
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
	if _, err := p.pool.Exec(ctx, progresSchema); err != nil {
		return err
	}

	return nil
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
		return fmt.Errorf("failed when getting hash: %w", err)
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
			return fmt.Errorf("failed when parsing: %w", err)
		}

		// Get the next occurrence of the scheduled time
		t := sched.Next(time.Now())
		nextTime = &t
	}

	var dagID int
	if err := tx.QueryRow(ctx, `
	INSERT INTO DAGs (name, version, hash, schedule, namespace, active, nexttime, taskCount, webhookUrl, sslVerification, workspaceEnabled) 
	VALUES ($1, $2, $3, $4, $5, TRUE, $6, $7, $8, $9, $10)
	RETURNING dag_id;`, dag.Name, version, hash, dag.Spec.Schedule, namespace, nextTime, len(dag.Spec.Task), dag.Spec.Webhook.URL, dag.Spec.Webhook.VerifySSL, dag.Spec.Workspace.Enabled).Scan(&dagID); err != nil {
		return fmt.Errorf("failed inserting DAG: %w", err)
	}

	// only insert workspace if enabled
	if dag.Spec.Workspace.Enabled {
		if err := p.insertWorkspace(ctx, tx, dagID, &dag.Spec.Workspace.PvcSpec); err != nil {
			return fmt.Errorf("failed to insert workspace: %w", err)
		}
	}

	// Insert all tasks into DAG_Tasks
	for _, task := range dag.Spec.Task {
		version := getTaskVersion(&task)

		if err := p.insertTask(ctx, tx, dagID, &task, namespace, version); err != nil {
			return err
		}
	}

	// After all tasks are inserted, handle dependencies
	for _, task := range dag.Spec.Task {
		version := getTaskVersion(&task)

		if err := p.createDependencyConnection(ctx, tx, dagID, &task, version); err != nil {
			return fmt.Errorf("failed to create dependency connection: %s", err)
		}
	}

	// Insert parameters and map them to the DAG
	for _, parameter := range dag.Spec.Parameters {
		if err := p.insertParameter(ctx, tx, dagID, &parameter); err != nil {
			return fmt.Errorf("failed to insert parameter '%s': %s", parameter.Name, err.Error())
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

func (p *postgresDAGManager) insertWorkspace(ctx context.Context, tx pgx.Tx, dagID int, workspace *v1alpha1.PVC) error {
	// Insert the workspace
	if _, err := tx.Exec(ctx, `
	INSERT INTO DAG_Workspaces (dag_id, accessModes, selector, resources, storageClassName, volumeMode) 
	VALUES ($1, $2, $3, $4, $5, $6);`, dagID, workspace.AccessModes, workspace.Selector, workspace.Resources, workspace.StorageClassName, workspace.VolumeMode); err != nil {
		return err
	}

	return nil
}

func (p *postgresDAGManager) insertTask(ctx context.Context, tx pgx.Tx, dagID int, task *v1alpha1.TaskSpec, namespace string, version int) error {
	var jsonValue *string
	if task.PodTemplate != nil {
		json, err := task.PodTemplate.Serialize()
		if err != nil {
			return fmt.Errorf("failed to serialise podTemplate: %s", err.Error())
		}

		jsonValue = &json
	}

	// Insert the task
	// Must check if it is inline or not
	var taskId int

	inline := task.TaskRef == nil
	if !inline {
		err := tx.QueryRow(ctx, `
		SELECT task_id FROM Tasks
		WHERE name = $1 AND inline = FALSE and version = $2;`, task.TaskRef.Name, task.TaskRef.Version).Scan(&taskId)
		if err != nil {
			return fmt.Errorf("failed to get task ref when inserting dag: %w, name: %s, version: %v", err, task.TaskRef.Name, task.TaskRef.Version)
		}

	} else {
		// must provide a unique name - name is used not used for in-line and must just be unique
		newUUID := uuid.New()

		if err := tx.QueryRow(ctx, `
		INSERT INTO Tasks (name, command, args, image, parameters, backoffLimit, isConditional, retryCodes, podTemplate, script, scriptInjectorImage, inline, namespace, version) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, TRUE, $12, $13) 
		RETURNING task_id;`,
			newUUID.String(), task.Command, task.Args, task.Image, task.Parameters, task.Backoff.Limit,
			task.Conditional.Enabled, task.Conditional.RetryCodes, jsonValue, task.Script, task.ScriptInjectorImage, namespace, version).Scan(&taskId); err != nil {
			return fmt.Errorf("failed to insert line task: %s", err.Error())
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO DAG_Tasks (dag_id, task_id, name, version)
		VALUES ($1, $2, $3, $4)`, dagID, taskId, task.Name, version); err != nil {
		return fmt.Errorf("failed to insert dag task: %s", err.Error())
	}

	return nil
}

func (p *postgresDAGManager) createDependencyConnection(ctx context.Context, tx pgx.Tx, dagID int, task *v1alpha1.TaskSpec, version int) error {
	for _, dependency := range task.RunAfter {
		var taskId, depId int

		err := tx.QueryRow(ctx, `
		SELECT dag_task_id
		FROM DAG_Tasks 
		WHERE dag_id = $1 AND name = $2 AND version = $3;`, dagID, task.Name, version).Scan(&taskId)
		if err != nil {
			return fmt.Errorf("task %s not found for version %d", task.Name, version)
		}

		err = tx.QueryRow(ctx, `
		SELECT dag_task_id
		FROM DAG_Tasks 
		WHERE dag_id = $1 AND name = $2
		ORDER BY version DESC
		LIMIT 1;
		;`, dagID, dependency).Scan(&depId)
		if err != nil {
			if err == pgx.ErrNoRows {
				return fmt.Errorf("dependency task %s not found for version %d", dependency, version)
			}
			return err
		}

		if _, err := tx.Exec(ctx, `
		INSERT INTO Dependencies (task_id, depends_on_task_id) 
		VALUES ($1, $2);`, taskId, depId); err != nil {
			return err
		}
	}

	return nil
}

func (p *postgresDAGManager) CreateDAGRun(ctx context.Context, name string, dag *v1alpha1.DagRunSpec, parameters map[string]v1alpha1.ParameterSpec, pvcName *string) (int, error) {
	dagId, err := p.dagNameToDagId(ctx, dag.DagName)
	if err != nil {
		return 0, err
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}

	defer tx.Rollback(ctx)

	// Map the task to the DAG
	var dagRunID int
	if err := tx.QueryRow(ctx, `
	INSERT INTO DAG_Runs (dag_id, name, status, successfulCount, failedCount, run_time, pvcName) 
	VALUES ($1, $2, 'running', 0, 0, NOW(), $3) 
	RETURNING run_id`, dagId, name, pvcName).Scan(&dagRunID); err != nil {
		return 0, err
	}

	// TODO: batch this
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

func (p *postgresDAGManager) GetStartingTasks(ctx context.Context, dagName string) ([]Task, error) {
	rows, err := p.pool.Query(ctx, `
	SELECT 
		dt.dag_task_id,
		dt.name, 
		t.image, 
		t.command, 
		t.args, 
		t.parameters, 
		t.podTemplate, 
		dt.dag_id, 
		t.script
	FROM 
		Tasks t
	JOIN 
		DAG_Tasks dt ON t.task_id = dt.task_id
	LEFT JOIN 
		Dependencies d ON dt.dag_task_id = d.task_id
	LEFT JOIN 
		DAG_Tasks dat ON dat.dag_task_id = d.depends_on_task_id
	WHERE 
		d.depends_on_task_id IS NULL  -- Ensure tasks with no dependencies
		AND dt.dag_id = (
			SELECT dag_id
			FROM DAGs
			WHERE name = $1
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
		var task Task
		var parameters []string
		var podTemplateJSON sql.NullString
		var script sql.NullString
		var dagId int

		if err := rows.Scan(&task.Id, &task.Name, &task.Image, &task.Command, &task.Args, &parameters, &podTemplateJSON, &dagId, &script); err != nil {
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
		if podTemplateJSON.Valid {
			if err := json.Unmarshal([]byte(podTemplateJSON.String), &podTemplate); err != nil {
				return nil, err
			}
		}

		if script.Valid {
			task.Script = script.String
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

	return tx.Commit(ctx)
}

func (p *postgresDAGManager) MarkSuccessAndGetNextTasks(ctx context.Context, taskRunId int) ([]Task, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}

	defer tx.Rollback(ctx)

	var runId int
	err = tx.QueryRow(ctx, `
	UPDATE Task_Runs 
	SET status = 'success' 
	WHERE task_run_id = $1 
	RETURNING run_id`, taskRunId).Scan(&runId)
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

	dagId, err := p.getDAGIdFromRun(ctx, tx, runId)
	if err != nil {
		return nil, err
	}

	tasks, parameters, err := p.getNextRunnableTasks(ctx, tx, taskRunId, runId, dagId)
	if err != nil {
		return nil, err
	}

	if err := p.fetchTaskParameters(ctx, tx, dagId, tasks, parameters); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (p *postgresDAGManager) getNextRunnableTasks(ctx context.Context, tx pgx.Tx, taskRunId, runId, dagId int) ([]Task, [][]string, error) {
	dependencyCounts, err := p.getDependencyCounts(ctx, tx, dagId)
	if err != nil {
		return nil, nil, err
	}

	metDependencies, err := p.getMetDependencies(ctx, tx, dagId, runId)
	if err != nil {
		return nil, nil, err
	}

	runnableTasks, err := p.getRunnableTasks(ctx, tx, dependencyCounts, metDependencies, taskRunId)
	if err != nil {
		return nil, nil, err
	}

	return p.getTasksByIds(ctx, tx, runnableTasks)
}

func (p *postgresDAGManager) getRunnableTasks(ctx context.Context, tx pgx.Tx, dependencyCounts, metDependencies map[int]int, taskRunId int) ([]int, error) {
	var runnableTasks []int

	for taskId, totalDeps := range dependencyCounts {
		metDeps := metDependencies[taskId]
		if totalDeps != metDeps {
			continue
		}
		var taskStatus string
		err := tx.QueryRow(ctx, `
                SELECT status
                FROM Task_Runs
                WHERE task_id = $1 AND run_id = $2;
            `, taskId, taskRunId).Scan(&taskStatus)

		if err == pgx.ErrNoRows {
			runnableTasks = append(runnableTasks, taskId)
			continue
		} else if err != nil {
			return nil, err
		}
	}

	return runnableTasks, nil
}

func (p *postgresDAGManager) getDependencyCounts(ctx context.Context, tx pgx.Tx, dagId int) (map[int]int, error) {
	rows, err := tx.Query(ctx, `
		SELECT d.task_id, COUNT(d.depends_on_task_id)
		FROM Dependencies d
		JOIN DAG_Tasks dt ON d.task_id = dt.dag_task_id
		WHERE dt.dag_id = $1
		GROUP BY d.task_id`, dagId)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

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

func (p *postgresDAGManager) getMetDependencies(ctx context.Context, tx pgx.Tx, dagId, runID int) (map[int]int, error) {
	// Query to get the count of met dependencies for tasks in the same DAG and not already started/completed
	rows, err := tx.Query(ctx, `
		SELECT d.task_id, COUNT(d.depends_on_task_id)
		FROM Dependencies d
		JOIN Task_Runs tr ON d.depends_on_task_id = tr.task_id
		WHERE tr.status = 'success'
		AND d.task_id IN (
			SELECT dag_task_id
			FROM DAG_Tasks 
			WHERE dag_id = $1
		)
		AND d.task_id NOT IN (
			SELECT task_id 
			FROM Task_Runs 
			WHERE
				status IN ('running', 'success')
			AND run_id = $2
		)
		AND tr.run_id = $2
		GROUP BY d.task_id`, dagId, runID)

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

func (p *postgresDAGManager) getTasksByIds(ctx context.Context, tx pgx.Tx, taskIds []int) ([]Task, [][]string, error) {
	// Ensure there are task IDs to query
	if len(taskIds) == 0 {
		return []Task{}, [][]string{}, nil
	}

	// Dynamically generate placeholders for the task IDs
	placeholders := []string{}
	args := []interface{}{}
	for i, id := range taskIds {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1)) // Create placeholders like $1, $2, ...
		args = append(args, id)
	}

	// Construct the query
	query := fmt.Sprintf(`
		SELECT dat.dag_task_id, dat.name, t.image, t.command, t.args, t.parameters, t.scriptInjectorImage, t.script, t.podTemplate
		FROM Tasks t
		JOIN DAG_Tasks dat ON dat.task_id = t.task_id
		WHERE dat.dag_task_id IN (%s)`, strings.Join(placeholders, ","))

	// Execute the query
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	// Process the rows
	tasks := []Task{}
	parameters := [][]string{}
	for rows.Next() {
		var task Task
		var params []string
		var podTemplateJSON sql.NullString

		if err := rows.Scan(&task.Id, &task.Name, &task.Image, &task.Command, &task.Args, &params, &task.ScriptInjectorImage, &task.Script, &podTemplateJSON); err != nil {
			return nil, nil, err
		}

		if podTemplateJSON.Valid {
			var podTemplate *v1alpha1.PodTemplateSpec
			if err := json.Unmarshal([]byte(podTemplateJSON.String), &podTemplate); err != nil {
				return nil, nil, err
			}

			task.PodTemplate = podTemplate
		}

		parameters = append(parameters, params)
		tasks = append(tasks, task)
	}

	return tasks, parameters, nil
}

func (p *postgresDAGManager) fetchTaskParameters(ctx context.Context, tx pgx.Tx, dagId int, tasks []Task, parameters [][]string) error {
	// Build a map to store task parameters
	taskParamMap := make(map[int][]Parameter)

	// Flatten parameters and associate them with task indices
	var flattenedParams []string
	taskIndices := make(map[string]int)

	for i, taskParams := range parameters {
		for _, param := range taskParams {
			flattenedParams = append(flattenedParams, param)
			taskIndices[param] = i
		}
	}

	// Query all parameters in a single batch
	rows, err := tx.Query(ctx, `
		SELECT name, isSecret, defaultValue
		FROM DAG_Parameters
		WHERE dag_id = $1 AND name = ANY($2)
	`, dagId, flattenedParams)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Map the results to their respective tasks
	for rows.Next() {
		var name string
		var isSecret bool
		var value string

		err := rows.Scan(&name, &isSecret, &value)
		if err != nil {
			return err
		}

		// Find the task index from the taskIndices map
		taskIdx := taskIndices[name]
		taskParamMap[taskIdx] = append(taskParamMap[taskIdx], Parameter{
			Name:     name,
			IsSecret: isSecret,
			Value:    value,
		})
	}

	// Assign the collected parameters back to tasks
	for i := range tasks {
		tasks[i].Parameters = taskParamMap[i]
	}

	return nil
}

func (p *postgresDAGManager) getDAGIdFromRun(ctx context.Context, tx pgx.Tx, runId int) (int, error) {
	var dagId int
	err := tx.QueryRow(ctx, `
		SELECT dag_id
		FROM dag_runs
		WHERE run_id = $1
	`, runId).Scan(&dagId)

	return dagId, err
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

type DagInfo struct {
	DagId            int
	DagName          string
	Namespace        string
	WorkspaceEnabled bool
}

func (p *postgresDAGManager) GetDAGsToStartAndUpdate(ctx context.Context) ([]*DagInfo, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}

	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
        SELECT dag_id, name, schedule, namespace, workspaceEnabled
        FROM DAGs
        WHERE nexttime <= NOW() AND schedule != '' AND active = TRUE;
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	namespaces := []*DagInfo{}

	schedules := []string{}
	for rows.Next() {
		var dagId int
		var name string
		var schedule string
		var namespace string
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
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	batch := &pgx.Batch{}
	for i, schedule := range schedules {
		// Parse the cron expression
		sched, err := p.parser.Parse(schedule)
		if err != nil {
			return nil, err
		}

		// Get the next occurrence of the scheduled time
		nextTime := sched.Next(time.Now())

		batch.Queue(`
            UPDATE DAGs 
            SET nextTime = $1 
            WHERE dag_id = $2;`, nextTime, namespaces[i].DagId)
	}

	br := tx.SendBatch(ctx, batch)
	if err := br.Close(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return namespaces, nil
}

func (p *postgresDAGManager) GetDagParameters(ctx context.Context, dagName string) (map[string]*Parameter, error) {
	rows, err := p.pool.Query(ctx, `
	SELECT name, isSecret, defaultValue
	FROM DAG_Parameters
	WHERE dag_id IN (
		SELECT dag_id
		FROM DAGs
		WHERE name = $1
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

func (p *postgresDAGManager) DagExists(ctx context.Context, dagName string) (bool, int, error) {
	dagId := -1
	if err := p.pool.QueryRow(ctx, `
		SELECT dag_id
		FROM DAGs
		WHERE name = $1
	`, dagName).Scan(&dagId); err != nil && err != pgx.ErrNoRows {
		return false, -1, err
	}

	return dagId != -1, dagId, nil
}

func (p *postgresDAGManager) ShouldRerun(ctx context.Context, taskRunid int, exitCode int32) (bool, error) {
	query := `
    SELECT EXISTS (
        SELECT 1
        FROM tasks t
        JOIN DAG_Tasks dt ON t.task_id = dt.task_id
        JOIN Task_Runs r ON dt.dag_task_id = r.task_id
        WHERE r.task_run_id = $1
          AND r.attempts <= t.backoffLimit
          AND (t.isConditional = FALSE OR $2 = ANY(t.retryCodes))
    )
	`

	var rerunNeeded bool
	err := p.pool.QueryRow(ctx, query, taskRunid, exitCode).Scan(&rerunNeeded)
	if err != nil {
		return false, err
	}

	return rerunNeeded, nil
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
		WHERE task_run_id = $1;
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

func (p *postgresDAGManager) MarkPodStatus(ctx context.Context, podUid types.UID, name string, taskRunID int, status v1.PodPhase, tStamp time.Time, exitCode *int32, namespace string) error {
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
        INSERT INTO Task_Pods (Pod_UID, task_run_id, name, status, namespace, updated_at, exitCode)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        ON CONFLICT (Pod_UID) 
        DO UPDATE SET status = EXCLUDED.status, updated_at = EXCLUDED.updated_at, exitCode = EXCLUDED.exitCode
        WHERE Task_Pods.updated_at < EXCLUDED.updated_at;
    `, podUid, taskRunID, name, status, namespace, tStamp, exitCode); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

type taskData struct {
	TaskID        int
	TaskName      string
	TaskNamespace string
}

func (p *postgresDAGManager) getTaskDeletionData(ctx context.Context, tx pgx.Tx, name, namespace string) ([]taskData, error) {
	// Check for tasks associated with the specified DAG
	rows, err := tx.Query(ctx, `
	SELECT DISTINCT(t.task_id), t.name, t.namespace
	FROM Tasks t
	JOIN DAG_Tasks dt ON t.task_id = dt.task_id
	JOIN DAGs d ON d.dag_id = dt.dag_id
	WHERE d.name = $1 AND d.namespace = $2 AND t.inline = FALSE;
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

func (p *postgresDAGManager) DeleteDAG(ctx context.Context, name string, namespace string) ([]string, error) {
	// Begin transaction
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}

	// Rollback transaction if not committed
	defer tx.Rollback(ctx)

	taskData, err := p.getTaskDeletionData(ctx, tx, name, namespace)
	if err != nil {
		return nil, err
	}

	// Check if each task is still associated with other DAGs
	var unusedTaskNames []string

	for _, task := range taskData {
		var count int
		err = tx.QueryRow(ctx, `
		SELECT COUNT(DISTINCT (d.name, d.namespace))
		FROM DAG_Tasks dt
		JOIN DAGs d ON dt.dag_id = d.dag_id
		WHERE 
			dt.task_id IN (
				SELECT task_id
				FROM tasks
				WHERE name = $1 AND namespace = $2
			)
			AND NOT (d.name = $3 AND d.namespace = $2);
	`, task.TaskName, namespace, name).Scan(&count)
		if err != nil {
			return nil, err
		}

		// Add tasks that are no longer connected to any DAG
		if count == 0 {
			unusedTaskNames = append(unusedTaskNames, task.TaskName)
		}
	}

	rowsTasks, err := tx.Query(ctx, `
	SELECT t.task_id
	FROM Tasks t
	JOIN dag_tasks dt ON dt.task_id = t.task_id
	LEFT JOIN dags d on dt.dag_id = d.dag_id
	WHERE d.name = $1 and t.inline = TRUE;
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
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
		i++
	}

	// Get the latest version of the DAG
	if _, err := tx.Exec(ctx, `
	DELETE FROM DAGs
	WHERE name = $1 AND namespace = $2;
	`, name, namespace); err != nil {
		return nil, err
	}

	if len(taskIds) > 0 {
		// Construct the query
		query := fmt.Sprintf(`
		DELETE FROM Tasks
		WHERE task_id IN (%s);`, strings.Join(placeholders, ","))

		if _, err := tx.Exec(ctx, query, taskIds...); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return unusedTaskNames, nil
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

func (p *postgresDAGManager) FindExistingDAGRun(ctx context.Context, name string) (bool, error) {
	var exists bool
	if err := p.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM DAG_Runs
			WHERE name = $1
		);
	`, name).Scan(&exists); err != nil && err != pgx.ErrNoRows {
		return false, err
	}

	return exists, nil
}

func (p *postgresDAGManager) dagNameToDagId(ctx context.Context, dagName string) (int, error) {
	dagId := -1
	if err := p.pool.QueryRow(ctx, `
		SELECT dag_id
		FROM DAGs
		WHERE name = $1
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

func (p *postgresDAGManager) GetID(ctx context.Context) (string, error) {
	var uniqueID string

	err := p.pool.QueryRow(ctx, "SELECT unique_id FROM IdTable LIMIT 1").Scan(&uniqueID)
	if err == nil {
		return uniqueID, nil
	}

	if err == pgx.ErrNoRows {
		err = p.pool.QueryRow(ctx, "INSERT INTO IdTable (unique_id) VALUES (gen_random_uuid()) RETURNING unique_id").Scan(&uniqueID)
		if err != nil {
			return "", fmt.Errorf("failed to insert new unique_id: %w", err)
		}
		return uniqueID, nil
	}

	return "", fmt.Errorf("failed to query IdTable: %w", err)
}

func (p *postgresDAGManager) GetTaskScriptAndInjectorImage(ctx context.Context, taskId int) (*string, *string, error) {
	var script *string
	var injectorImage *string

	if err := p.pool.QueryRow(ctx, `
	SELECT t.script, t.scriptInjectorImage
	FROM Tasks t
	WHERE t.task_id = (
		SELECT task_id
		FROM DAG_Tasks
		WHERE dag_task_id = $1
		);
	`, taskId).Scan(&script, &injectorImage); err != nil {
		return nil, nil, err
	}

	return script, injectorImage, nil
}

func (p *postgresDAGManager) AddTask(ctx context.Context, task *v1alpha1.DagTask, namespace string) error {
	// Begin transaction
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}

	// Rollback transaction if not committed
	defer tx.Rollback(ctx)

	var taskId int
	var version int
	var hash *string

	err = tx.QueryRow(ctx, `
	SELECT task_id, version, hash
	FROM Tasks
	WHERE name = $1 AND namespace = $2
	ORDER BY version DESC;`, task.Name, namespace).Scan(&taskId, &version, &hash)
	if err != nil && err != pgx.ErrNoRows {
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

	newVersion := version + 1

	if _, err := tx.Exec(ctx, `
    INSERT INTO Tasks (name, command, args, image, parameters, backoffLimit, isConditional, retryCodes, podTemplate, script, scriptInjectorImage, inline, namespace, version, hash)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, FALSE, $12, $13, $14);`,
		task.Name, task.Spec.Command, task.Spec.Args, task.Spec.Image, task.Spec.Parameters, task.Spec.Backoff.Limit,
		task.Spec.Conditional.Enabled, task.Spec.Conditional.RetryCodes, jsonValue, task.Spec.Script, task.Spec.ScriptInjectorImage, namespace, newVersion, hashValue); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (p *postgresDAGManager) GetTaskRefsParameters(ctx context.Context, taskRefs []v1alpha1.TaskRef) (map[v1alpha1.TaskRef][]string, error) {
	taskMp := map[v1alpha1.TaskRef][]string{}

	querySql := `
		SELECT parameters
		FROM Tasks
		WHERE name = $1 AND version = $2 AND inline = FALSE;
    `

	for _, val := range taskRefs {
		var parameters = []string{}
		if err := p.pool.QueryRow(ctx, querySql, val.Name, val.Version).Scan(&parameters); err != nil {
			return nil, err
		}

		taskMp[val] = parameters
	}

	return taskMp, nil
}

func (p *postgresDAGManager) DeleteTask(ctx context.Context, taskName, namespace string) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		DELETE FROM Tasks
		WHERE
			name = $1
		AND namespace = $2
		AND inline = FALSE;
	`, taskName, namespace); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (p *postgresDAGManager) GetWebhookDetails(ctx context.Context, dagRunID int) (*v1alpha1.Webhook, error) {
	webhook := &v1alpha1.Webhook{}

	err := p.pool.QueryRow(ctx, `
	SELECT webhookUrl, sslVerification
	FROM DAGs
	WHERE dag_id = (
		SELECT dag_id
		FROM DAG_Runs
		WHERE run_id = $1
	);
	`, dagRunID).Scan(&webhook.URL, &webhook.VerifySSL)
	if err != nil {
		return nil, err
	}

	return webhook, nil
}

func (p *postgresDAGManager) GetWorkspacePVCTemplate(ctx context.Context, dagId int) (*v1alpha1.PVC, error) {
	pvc := &v1alpha1.PVC{}

	err := p.pool.QueryRow(ctx, `
    SELECT accessModes, selector, storageClassName, volumeMode, resources
    FROM DAG_Workspaces
    WHERE dag_id = $1;
    `, dagId).Scan(&pvc.AccessModes, &pvc.Selector, &pvc.StorageClassName, &pvc.VolumeMode, &pvc.Resources)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return pvc, nil
}
