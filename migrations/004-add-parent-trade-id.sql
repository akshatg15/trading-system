-- Migration: Add parent_trade_id column to trades table
-- This enables proper parent-child relationships between entry trades and TP trades

-- Add parent_trade_id column to trades table
ALTER TABLE trades ADD COLUMN parent_trade_id INTEGER;

-- Add foreign key constraint to ensure referential integrity
ALTER TABLE trades ADD CONSTRAINT fk_trades_parent_trade_id 
    FOREIGN KEY (parent_trade_id) REFERENCES trades(id) ON DELETE SET NULL;

-- Add index for better query performance when looking up child trades
CREATE INDEX idx_trades_parent_trade_id ON trades(parent_trade_id);

-- Add index for querying trades by signal and parent relationship
CREATE INDEX idx_trades_signal_parent ON trades(signal_id, parent_trade_id);

-- Add comment to document the purpose
COMMENT ON COLUMN trades.parent_trade_id IS 'References the parent trade ID for TP trades (entry trade ID)'; 