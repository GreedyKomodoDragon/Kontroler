-- PostgreSQL initial schema migration

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS IdTable (
    unique_id UUID PRIMARY KEY DEFAULT uuid_generate_v4()
);

CREATE TABLE IF NOT EXISTS DAGs (
    dag_id SERIAL PRIMARY KEY,
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
    id SERIAL PRIMARY KEY,
    dag_id INTEGER NOT NULL,
    accessModes TEXT[],
    selector JSONB,
    resources JSONB,
    storageClassName TEXT,
    volumeMode TEXT,
    FOREIGN KEY (dag_id) REFERENCES DAGs(dag_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS DAG_Parameters (
    parameter_id SERIAL PRIMARY KEY,
    dag_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    isSecret BOOLEAN NOT NULL,
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
    isConditional BOOLEAN NOT NULL,
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
    isSecret BOOLEAN NOT NULL,
    FOREIGN KEY (run_id) REFERENCES DAG_Runs(run_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS Task_Runs (
    task_run_id SERIAL PRIMARY KEY,
    run_id INTEGER NOT NULL,
    task_id INTEGER NOT NULL,
    status VARCHAR(255) NOT NULL,
    attempts INTEGER NOT NULL,
    FOREIGN KEY (task_id) REFERENCES DAG_Tasks(dag_task_id) ON DELETE CASCADE,
    FOREIGN KEY (run_id) REFERENCES DAG_Runs(run_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS Task_Pods (
    Pod_UID VARCHAR(255) PRIMARY KEY,
    task_run_id INTEGER NOT NULL,
    exitCode INTEGER,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(255) NOT NULL,
    namespace TEXT NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    duration INTEGER,
    FOREIGN KEY (task_run_id) REFERENCES Task_Runs(task_run_id) ON DELETE CASCADE
);

-- creating indexes
CREATE INDEX IF NOT EXISTS idx_dags_name_version ON DAGs (name, version DESC);
CREATE INDEX IF NOT EXISTS idx_dags_dag_id ON DAGs (dag_id);
CREATE INDEX IF NOT EXISTS idx_dag_tasks_dag_id ON DAG_Tasks (dag_id);
CREATE INDEX IF NOT EXISTS idx_dag_tasks_task_id ON DAG_Tasks (task_id);
CREATE INDEX IF NOT EXISTS idx_dag_tasks_dag_task_id ON DAG_Tasks (dag_task_id);
CREATE INDEX IF NOT EXISTS idx_dependencies_task_id ON Dependencies (task_id);
CREATE INDEX IF NOT EXISTS idx_dependencies_depends_on_task_id ON Dependencies (depends_on_task_id);
CREATE INDEX IF NOT EXISTS idx_dag_runs_dag_id ON DAG_Runs (dag_id);
CREATE INDEX IF NOT EXISTS idx_dag_runs_run_id ON DAG_Runs (run_id);
CREATE INDEX IF NOT EXISTS idx_tasks_task_id ON Tasks (task_id);
CREATE INDEX IF NOT EXISTS idx_tasks_name_version_namespace ON Tasks (name, version, namespace);