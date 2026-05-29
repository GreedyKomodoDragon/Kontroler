ALTER TABLE Task_Runs
  ADD COLUMN IF NOT EXISTS claimed_by TEXT,
  ADD COLUMN IF NOT EXISTS claimed_at TIMESTAMP WITH TIME ZONE,
  ADD COLUMN IF NOT EXISTS lease_expires_at TIMESTAMP WITH TIME ZONE,
  ADD COLUMN IF NOT EXISTS scheduled_start TIMESTAMP WITH TIME ZONE;

-- Indexes to speed up claim and recovery operations
CREATE INDEX IF NOT EXISTS idx_task_runs_status_scheduled ON Task_Runs (status, scheduled_start);
CREATE INDEX IF NOT EXISTS idx_task_runs_lease_expires_at ON Task_Runs (lease_expires_at);
