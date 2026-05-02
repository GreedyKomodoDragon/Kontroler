-- add composite index for fast parameter lookups by dag and name
CREATE INDEX IF NOT EXISTS idx_dag_parameters_dagid_name ON DAG_Parameters (dag_id, name);

-- add indexes for Task_Runs lookups used by dependency/status queries
CREATE INDEX IF NOT EXISTS idx_task_runs_taskid_runid ON Task_Runs (task_id, run_id);
CREATE INDEX IF NOT EXISTS idx_task_runs_runid_status ON Task_Runs (run_id, status);
