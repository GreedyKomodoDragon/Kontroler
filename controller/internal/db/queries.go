package db

const (
	// DAG queries
	QueryGetDAG = `
		SELECT dag_id, version, hash, suspended
		FROM DAGs
		WHERE name = $1 AND namespace = $2
		ORDER BY version DESC;`

	QueryInsertDAG = `
		INSERT INTO DAGs (name, version, hash, schedule, namespace, active, nexttime, taskCount, webhookUrl, sslVerification, workspaceEnabled, suspended) 
		VALUES ($1, $2, $3, $4, $5, TRUE, $6, $7, $8, $9, $10, $11)
		RETURNING dag_id;`

	QueryInsertWorkspace = `
		INSERT INTO DAG_Workspaces (dag_id, accessModes, selector, resources, storageClassName, volumeMode) 
		VALUES ($1, $2, $3, $4, $5, $6);`

	QueryInsertParameter = `
		INSERT INTO DAG_Parameters (dag_id, name, isSecret, defaultValue) 
		VALUES ($1, $2, $3, $4)`

	QueryGetTaskByRef = `
		SELECT task_id FROM Tasks
		WHERE name = $1 AND inline = FALSE and version = $2;`

	QueryInsertInlineTask = `
		INSERT INTO Tasks (name, command, args, image, parameters, backoffLimit, isConditional, retryCodes, podTemplate, script, scriptInjectorImage, inline, namespace, version) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, TRUE, $12, $13) 
		RETURNING task_id;`

	QueryInsertDagTask = `
		INSERT INTO DAG_Tasks (dag_id, task_id, name, version)
		VALUES ($1, $2, $3, $4)`

	QueryGetTaskID = `
		SELECT dag_task_id
		FROM DAG_Tasks 
		WHERE dag_id = $1 AND name = $2 AND version = $3;`

	QueryGetDependencyTaskID = `
		SELECT dag_task_id
		FROM DAG_Tasks 
		WHERE dag_id = $1 AND name = $2
		ORDER BY version DESC
		LIMIT 1;`

	QueryInsertDependency = `
		INSERT INTO Dependencies (task_id, depends_on_task_id) 
		VALUES ($1, $2);`

	QuerySetInactive = `
		UPDATE DAGs 
		SET active = FALSE 
		WHERE name = $1 AND namespace = $2 AND version = $3`

	QueryMarkTaskSuccess = `
		UPDATE Task_Runs 
		SET status = 'success' 
		WHERE task_run_id = $1 
		RETURNING run_id`

	QueryUpdateSuccessCount = `
		UPDATE DAG_Runs
		SET successfulCount = successfulCount + 1
		WHERE run_id = $1;`

	QueryUpdateDAGRunStatus = `
		UPDATE DAG_Runs
		SET status = 'success'
		FROM DAGs
		WHERE DAG_Runs.dag_id = DAGs.dag_id
		AND DAGs.taskCount = DAG_Runs.successfulCount
		AND DAG_Runs.run_id = $1
		RETURNING DAG_Runs.status;`

	// Task queries
	QueryInsertTask = `
		INSERT INTO Tasks (name, command, args, image, parameters, backoffLimit, isConditional, retryCodes, podTemplate, script, scriptInjectorImage, inline, namespace, version) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, TRUE, $12, $13) 
		RETURNING task_id;`
)
