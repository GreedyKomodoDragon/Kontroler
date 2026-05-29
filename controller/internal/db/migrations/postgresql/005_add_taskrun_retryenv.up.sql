ALTER TABLE Task_Runs
  ADD COLUMN IF NOT EXISTS retry_env JSONB;

CREATE INDEX IF NOT EXISTS idx_task_runs_retry_env ON Task_Runs USING gin (coalesce(retry_env, '{}'::jsonb));
