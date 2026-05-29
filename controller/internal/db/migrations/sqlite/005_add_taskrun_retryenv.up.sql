ALTER TABLE Task_Runs ADD COLUMN retry_env TEXT;
CREATE INDEX IF NOT EXISTS idx_task_runs_retry_env ON Task_Runs (retry_env);
