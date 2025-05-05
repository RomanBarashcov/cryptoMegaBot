-- Drop old tables if they exist
DROP TABLE IF EXISTS trades;
DROP TABLE IF EXISTS positions;
DROP TABLE IF EXISTS trade_history;

-- Create positions table
CREATE TABLE IF NOT EXISTS positions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol TEXT NOT NULL,
    entry_price REAL NOT NULL,
    exit_price REAL DEFAULT NULL,
    quantity REAL NOT NULL,
    leverage INTEGER NOT NULL,
    stop_loss REAL NOT NULL,
    take_profit REAL NOT NULL,
    entry_time TIMESTAMP NOT NULL,
    exit_time TIMESTAMP DEFAULT NULL,
    status TEXT NOT NULL,
    pnl REAL DEFAULT NULL,
    UNIQUE(symbol, status)
);

-- Create trade history table
CREATE TABLE IF NOT EXISTS trade_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol TEXT NOT NULL,
    entry_price REAL NOT NULL,
    exit_price REAL NOT NULL,
    quantity REAL NOT NULL,
    leverage INTEGER NOT NULL,
    pnl REAL NOT NULL,
    entry_time TIMESTAMP NOT NULL,
    exit_time TIMESTAMP NOT NULL
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_positions_symbol ON positions(symbol);
CREATE INDEX IF NOT EXISTS idx_positions_status ON positions(status);
CREATE INDEX IF NOT EXISTS idx_positions_entry_time ON positions(entry_time);
CREATE INDEX IF NOT EXISTS idx_trade_history_symbol ON trade_history(symbol);
CREATE INDEX IF NOT EXISTS idx_trade_history_entry_time ON trade_history(entry_time);

-- Create trigger to enforce one open position per symbol
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