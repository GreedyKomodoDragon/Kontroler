-- SQLite initial schema migration

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