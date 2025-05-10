-- Add suspension capability to DAGs
ALTER TABLE DAGs ADD COLUMN suspended BOOLEAN NOT NULL DEFAULT FALSE;