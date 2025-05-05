-- Drop old tables if they exist (Keep DROP for positions for easier reset during dev)
DROP TABLE IF EXISTS positions;
DROP TABLE IF EXISTS trade_history; -- Removing this table

-- Create positions table (Stores both open and closed positions)
CREATE TABLE IF NOT EXISTS positions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol TEXT NOT NULL,
    entry_price REAL NOT NULL,
    exit_price REAL DEFAULT NULL, -- Null if open
    quantity REAL NOT NULL,
    leverage INTEGER NOT NULL,
    stop_loss REAL NOT NULL,     -- Price level
    take_profit REAL NOT NULL,   -- Price level
    entry_time TIMESTAMP NOT NULL,
    exit_time TIMESTAMP DEFAULT NULL,  -- Null if open
    status TEXT NOT NULL CHECK(status IN ('open', 'closed')), -- Use CHECK constraint
    pnl REAL DEFAULT NULL,             -- Null if open
    stop_loss_order_id TEXT DEFAULT NULL, -- Store associated SL order ID (nullable)
    take_profit_order_id TEXT DEFAULT NULL, -- Store associated TP order ID (nullable)
    close_reason TEXT DEFAULT NULL     -- Reason for closing (SL, TP, Market, etc.) (nullable)
    -- Removed UNIQUE constraint, trigger handles the 'one open position' rule
);

-- Indexes for positions table
CREATE INDEX IF NOT EXISTS idx_positions_symbol_status ON positions(symbol, status); -- Combined index is often better
CREATE INDEX IF NOT EXISTS idx_positions_entry_time ON positions(entry_time);
-- Removed indexes for trade_history

-- Trigger to enforce only one 'open' position per symbol
CREATE TRIGGER IF NOT EXISTS enforce_one_open_position
BEFORE INSERT ON positions
WHEN NEW.status = 'open'
BEGIN
    SELECT RAISE(ABORT, 'Only one open position per symbol allowed')
    WHERE EXISTS (
        SELECT 1 FROM positions 
        WHERE symbol = NEW.symbol AND status = 'open'
    );
END;
