-- Add primary key with descending order
ALTER TABLE history
ADD PRIMARY KEY (id DESC);
-- Add descending index on executed_at
CREATE INDEX idx_history_executed_at ON history (executed_at DESC);