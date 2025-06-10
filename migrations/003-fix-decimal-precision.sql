-- Migration 003: Fix decimal precision for price fields
-- The current DECIMAL(15,5) format is causing numeric field overflow
-- Changing to DECIMAL(20,8) to accommodate larger values with more precision

-- Update signals table price fields
ALTER TABLE signals 
ALTER COLUMN price TYPE DECIMAL(20,8),
ALTER COLUMN stop_loss TYPE DECIMAL(20,8),
ALTER COLUMN take_profit TYPE DECIMAL(20,8),
ALTER COLUMN tp1 TYPE DECIMAL(20,8),
ALTER COLUMN tp2 TYPE DECIMAL(20,8),
ALTER COLUMN sl1 TYPE DECIMAL(20,8),
ALTER COLUMN sl2 TYPE DECIMAL(20,8);

-- Update trades table price fields
ALTER TABLE trades 
ALTER COLUMN entry_price TYPE DECIMAL(20,8),
ALTER COLUMN current_price TYPE DECIMAL(20,8),
ALTER COLUMN stop_loss TYPE DECIMAL(20,8),
ALTER COLUMN take_profit TYPE DECIMAL(20,8),
ALTER COLUMN tp1 TYPE DECIMAL(20,8),
ALTER COLUMN tp2 TYPE DECIMAL(20,8),
ALTER COLUMN sl1 TYPE DECIMAL(20,8),
ALTER COLUMN sl2 TYPE DECIMAL(20,8);

-- Keep profit/loss fields with less precision as they don't need as much
-- ALTER TABLE trades 
-- ALTER COLUMN profit_loss TYPE DECIMAL(20,2),
-- ALTER COLUMN commission TYPE DECIMAL(20,2),
-- ALTER COLUMN swap TYPE DECIMAL(20,2); 