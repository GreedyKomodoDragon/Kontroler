CREATE TABLE IF NOT EXISTS IdTable (
    unique_id UUID DEFAULT gen_random_uuid() PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS DAGs (
    dag_id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    version INTEGER NOT NULL,
    hash VARCHAR(64) NOT NULL,
    schedule VARCHAR(255) NOT NULL,
    namespace VARCHAR(63) NOT NULL,
    active BOOL NOT NULL,
    taskCount INTEGER NOT NULL,
    nexttime TIMESTAMP,
	webhookUrl VARCHAR(255),
	sslVerification BOOL,
    workspaceEnabled BOOL,
    UNIQUE(name, version, namespace)
);

CREATE TABLE IF NOT EXISTS DAG_Workspaces (
    id SERIAL PRIMARY KEY,
    dag_id INT REFERENCES DAGs(dag_id) ON DELETE CASCADE,
    accessModes TEXT[],
    selector JSONB,
    resources JSONB,
    storageClassName TEXT,
    volumeMode TEXT
);

CREATE TABLE IF NOT EXISTS DAG_Parameters (
    parameter_id SERIAL PRIMARY KEY,
    dag_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    isSecret BOOL NOT NULL,
    defaultValue VARCHAR(255) NOT NULL,
    FOREIGN KEY (dag_id) REFERENCES DAGs(dag_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS Tasks (
    task_id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    command TEXT[],
    args TEXT[],
    image VARCHAR(255) NOT NULL,
    parameters TEXT[],
    backoffLimit BIGINT NOT NULL,
    isConditional BOOL NOT NULL,
    podTemplate JSONB,
    retryCodes INTEGER[],
    script TEXT NOT NULL,
    scriptInjectorImage TEXT,
    inline BOOL NOT NULL,
    namespace VARCHAR(63) NOT NULL,
    version INTEGER NOT NULL,
    hash VARCHAR(64),
    UNIQUE(name, version, namespace)
);

CREATE TABLE IF NOT EXISTS DAG_Tasks (
    dag_task_id SERIAL PRIMARY KEY,
    dag_id INTEGER NOT NULL,
    task_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    version INTEGER NOT NULL,
    FOREIGN KEY (dag_id) REFERENCES DAGs(dag_id) ON DELETE CASCADE,
    FOREIGN KEY (task_id) REFERENCES Tasks(task_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS Dependencies (
    task_id INTEGER NOT NULL,
    depends_on_task_id INTEGER NOT NULL,
    FOREIGN KEY (task_id) REFERENCES DAG_Tasks(dag_task_id) ON DELETE CASCADE,
    FOREIGN KEY (depends_on_task_id) REFERENCES DAG_Tasks(dag_task_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS DAG_Runs (
    run_id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    dag_id INTEGER NOT NULL,
    status VARCHAR(255) NOT NULL,
    successfulCount INTEGER NOT NULL,
    failedCount INTEGER NOT NULL,
    suspendedCount INTEGER NOT NULL,
    run_time TIMESTAMP NOT NULL,
    pvcName VARCHAR(255),
    FOREIGN KEY (dag_id) REFERENCES DAGs(dag_id) ON DELETE CASCADE,
    UNIQUE(name)
);

CREATE TABLE IF NOT EXISTS DAG_Run_Parameters (
    param_id SERIAL PRIMARY KEY,
    run_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    value VARCHAR(255) NOT NULL,
    isSecret BOOL NOT NULL,
    FOREIGN KEY (run_id) REFERENCES DAG_Runs(run_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS Task_Runs (
    task_run_id SERIAL PRIMARY KEY,
    run_id INTEGER NOT NULL,
    task_id INTEGER NOT NULL,
    status VARCHAR(255) NOT NULL,
    attempts INTEGER NOT NULL,
    FOREIGN KEY (task_id) REFERENCES DAG_Tasks(dag_task_id) ON DELETE CASCADE,
    FOREIGN KEY (run_id) REFERENCES DAG_Runs(run_id) ON DELETE CASCADE,
    UNIQUE(run_id, task_id)
);

CREATE TABLE IF NOT EXISTS Task_Pods (
    Pod_UID VARCHAR(255) NOT NULL,
    task_run_id INTEGER NOT NULL,
    exitCode INTEGER,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(255) NOT NULL,
    namespace TEXT NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (Pod_UID),
    FOREIGN KEY (task_run_id) REFERENCES Task_Runs(task_run_id) ON DELETE CASCADE
);

-- creating indexes
-- Indexes for DAGs table to speed up lookup by name and version
CREATE INDEX IF NOT EXISTS idx_dags_name_version ON DAGs (name, version DESC);
CREATE INDEX IF NOT EXISTS idx_dags_dag_id ON DAGs (dag_id);

-- Indexes for DAG_Tasks to speed up joins on dag_id and task_id
CREATE INDEX IF NOT EXISTS idx_dag_tasks_dag_id ON DAG_Tasks (dag_id);
CREATE INDEX IF NOT EXISTS idx_dag_tasks_task_id ON DAG_Tasks (task_id);
CREATE INDEX IF NOT EXISTS idx_dag_tasks_dag_task_id ON DAG_Tasks (dag_task_id);

-- Index for Dependencies table to speed up dependency lookups
CREATE INDEX IF NOT EXISTS idx_dependencies_task_id ON Dependencies (task_id);
CREATE INDEX IF NOT EXISTS idx_dependencies_depends_on_task_id ON Dependencies (depends_on_task_id);

-- Indexes for DAG_Runs to optimize query filters
CREATE INDEX IF NOT EXISTS idx_dag_runs_dag_id ON DAG_Runs (dag_id);
CREATE INDEX IF NOT EXISTS idx_dag_runs_run_id ON DAG_Runs (run_id);

-- Indexes for Tasks to speed up lookup by task_id
CREATE INDEX IF NOT EXISTS idx_tasks_task_id ON Tasks (task_id);
CREATE INDEX IF NOT EXISTS idx_tasks_name_version_namespace ON Tasks (name, version, namespace);
