ALTER TABLE Task_Runs ADD COLUMN claimed_by TEXT;
ALTER TABLE Task_Runs ADD COLUMN claimed_at DATETIME;
ALTER TABLE Task_Runs ADD COLUMN lease_expires_at DATETIME;
ALTER TABLE Task_Runs ADD COLUMN scheduled_start DATETIME;

CREATE INDEX IF NOT EXISTS idx_task_runs_status_scheduled ON Task_Runs (status, scheduled_start);
CREATE INDEX IF NOT EXISTS idx_task_runs_lease_expires_at ON Task_Runs (lease_expires_at);
