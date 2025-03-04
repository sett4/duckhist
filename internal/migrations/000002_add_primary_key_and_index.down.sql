-- Remove index
DROP INDEX IF EXISTS idx_history_executed_at;
-- Remove primary key
ALTER TABLE history DROP PRIMARY KEY;