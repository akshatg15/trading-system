-- Migration 002: Add partial profit and trade tracking fields
-- Add missing fields to signals table
ALTER TABLE signals 
ADD COLUMN IF NOT EXISTS tp1 DECIMAL(15,5),
ADD COLUMN IF NOT EXISTS tp2 DECIMAL(15,5),
ADD COLUMN IF NOT EXISTS sl1 DECIMAL(15,5),
ADD COLUMN IF NOT EXISTS sl2 DECIMAL(15,5);

-- Add missing fields to trades table
ALTER TABLE trades 
ADD COLUMN IF NOT EXISTS parent_signal_id INTEGER REFERENCES signals(id),
ADD COLUMN IF NOT EXISTS trade_type VARCHAR(20) NOT NULL DEFAULT 'entry' CHECK (trade_type IN ('entry', 'tp1', 'tp2', 'sl', 'manual_close')),
ADD COLUMN IF NOT EXISTS tp1 DECIMAL(15,5),
ADD COLUMN IF NOT EXISTS tp2 DECIMAL(15,5),
ADD COLUMN IF NOT EXISTS sl1 DECIMAL(15,5),
ADD COLUMN IF NOT EXISTS sl2 DECIMAL(15,5);

-- Add indexes for new fields
CREATE INDEX IF NOT EXISTS idx_trades_parent_signal_id ON trades(parent_signal_id);
CREATE INDEX IF NOT EXISTS idx_trades_trade_type ON trades(trade_type);

-- Update existing trades to have trade_type = 'entry' if null
UPDATE trades SET trade_type = 'entry' WHERE trade_type IS NULL;

-- Add constraint to ensure trade_type is never null
ALTER TABLE trades ALTER COLUMN trade_type SET NOT NULL; 