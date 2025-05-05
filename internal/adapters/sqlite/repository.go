package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"cryptoMegaBot/internal/domain"
	"cryptoMegaBot/internal/ports"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// Repository implements the ports.PositionRepository and ports.TradeRepository interfaces using SQLite.
type Repository struct {
	db     *sql.DB
	logger ports.Logger
}

// Config holds configuration for the SQLite repository.
type Config struct {
	DBPath string
	Logger ports.Logger
}

// NewRepository creates a new SQLite repository instance.
func NewRepository(cfg Config) (*Repository, error) {
	if cfg.Logger == nil {
		return nil, fmt.Errorf("logger is required for SQLite repository")
	}
	dbPath := cfg.DBPath
	if dbPath == "" {
		dbPath = "./data/trading_bot.db" // Default path
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		err = fmt.Errorf("failed to create data directory '%s': %w", filepath.Dir(dbPath), err)
		cfg.Logger.Error(context.Background(), err, "SQLite repository initialization failed")
		return nil, err
	}
	cfg.Logger.Info(context.Background(), "Data directory checked/created", map[string]interface{}{"path": filepath.Dir(dbPath)})

	// Open database connection
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000") // WAL mode for better concurrency
	if err != nil {
		err = fmt.Errorf("failed to open database at '%s': %w", dbPath, err)
		cfg.Logger.Error(context.Background(), err, "SQLite repository initialization failed")
		return nil, err
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close() // Close the connection if ping fails
		err = fmt.Errorf("failed to ping database at '%s': %w", dbPath, err)
		cfg.Logger.Error(context.Background(), err, "SQLite repository initialization failed")
		return nil, err
	}

	// Set connection pool settings (important for SQLite)
	db.SetMaxOpenConns(1) // SQLite handles concurrency internally, but Go driver benefits from limiting connections
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour) // Optional: recycle connections periodically

	cfg.Logger.Info(context.Background(), "SQLite database connection established", map[string]interface{}{"path": dbPath})

	repo := &Repository{db: db, logger: cfg.Logger}

	// Initialize schema (consider moving to a separate migration tool/step)
	if err := repo.initializeSchema(context.Background()); err != nil {
		db.Close()
		err = fmt.Errorf("failed to initialize database schema: %w", err)
		cfg.Logger.Error(context.Background(), err, "SQLite repository initialization failed")
		return nil, err
	}
	cfg.Logger.Info(context.Background(), "Database schema initialized/verified")

	return repo, nil
}

// initializeSchema creates tables if they don't exist.
// NOTE: This is a basic approach. A proper migration tool is recommended for production.
func (r *Repository) initializeSchema(ctx context.Context) error {
	const schema = `
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
		pnl REAL DEFAULT NULL
	);

	CREATE TABLE IF NOT EXISTS trade_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		symbol TEXT NOT NULL,
		entry_price REAL NOT NULL,
		exit_price REAL NOT NULL,
		quantity REAL NOT NULL,
		leverage INTEGER NOT NULL,
		pnl REAL NOT NULL,
		entry_time TIMESTAMP NOT NULL,
		exit_time TIMESTAMP NOT NULL,
		position_id INTEGER NULL, -- No foreign key constraint for simplicity here
		close_reason TEXT NULL
	);
	-- Add indexes for common lookups
	CREATE INDEX IF NOT EXISTS idx_positions_symbol_status ON positions (symbol, status);
	CREATE INDEX IF NOT EXISTS idx_trade_history_symbol_entry_time ON trade_history (symbol, entry_time);
	`
	_, err := r.db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("failed to execute schema initialization: %w", err)
	}
	return nil
}

// Close closes the database connection.
func (r *Repository) Close() error {
	if r.db != nil {
		r.logger.Info(context.Background(), "Closing SQLite database connection")
		return r.db.Close()
	}
	return nil
}

// --- PositionRepository Implementation ---

// Create saves a new position and returns its assigned ID.
func (r *Repository) Create(ctx context.Context, pos *domain.Position) (int64, error) {
	const query = `
	INSERT INTO positions (symbol, entry_price, quantity, leverage, stop_loss, take_profit, entry_time, status)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := r.db.ExecContext(ctx, query,
		pos.Symbol, pos.EntryPrice, pos.Quantity, pos.Leverage, pos.StopLoss, pos.TakeProfit, pos.EntryTime, pos.Status)
	if err != nil {
		return 0, fmt.Errorf("failed to insert position for symbol %s: %w", pos.Symbol, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		// This might happen if the table doesn't have AUTOINCREMENT or similar issues
		return 0, fmt.Errorf("failed to get last insert ID for position %s: %w", pos.Symbol, err)
	}
	pos.ID = id // Update the domain object with the ID
	r.logger.Debug(ctx, "Position created", map[string]interface{}{"positionID": id, "symbol": pos.Symbol})
	return id, nil
}

// Update modifies an existing position based on its ID.
func (r *Repository) Update(ctx context.Context, pos *domain.Position) error {
	const query = `
	UPDATE positions
	SET entry_price = ?, exit_price = ?, quantity = ?, leverage = ?, stop_loss = ?,
	    take_profit = ?, entry_time = ?, exit_time = ?, status = ?, pnl = ?
	WHERE id = ?`

	var exitTime sql.NullTime
	if !pos.ExitTime.IsZero() {
		exitTime = sql.NullTime{Time: pos.ExitTime, Valid: true}
	}

	result, err := r.db.ExecContext(ctx, query,
		pos.EntryPrice, pos.ExitPrice, pos.Quantity, pos.Leverage, pos.StopLoss,
		pos.TakeProfit, pos.EntryTime, exitTime, pos.Status, pos.PNL,
		pos.ID)
	if err != nil {
		return fmt.Errorf("failed to update position ID %d: %w", pos.ID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected for update position ID %d: %w", pos.ID, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("position ID %d not found for update: %w", pos.ID, ports.ErrNotFound)
	}
	r.logger.Debug(ctx, "Position updated", map[string]interface{}{"positionID": pos.ID, "symbol": pos.Symbol, "status": pos.Status})
	return nil
}

// FindOpenBySymbol retrieves the currently open position for a given symbol, if any.
func (r *Repository) FindOpenBySymbol(ctx context.Context, symbol string) (*domain.Position, error) {
	const query = `
	SELECT id, symbol, entry_price, COALESCE(exit_price, 0), quantity, leverage,
	       stop_loss, take_profit, entry_time, exit_time, status, COALESCE(pnl, 0)
	FROM positions
	WHERE symbol = ? AND status = ?`

	row := r.db.QueryRowContext(ctx, query, symbol, domain.StatusOpen)
	pos, err := scanPosition(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.logger.Debug(ctx, "No open position found for symbol", map[string]interface{}{"symbol": symbol})
			return nil, nil // Not an error, just not found
		}
		return nil, fmt.Errorf("failed to query open position for symbol %s: %w", symbol, err)
	}
	return pos, nil
}

// FindByID retrieves a position by its unique ID.
func (r *Repository) FindByID(ctx context.Context, id int64) (*domain.Position, error) {
	const query = `
	SELECT id, symbol, entry_price, COALESCE(exit_price, 0), quantity, leverage,
	       stop_loss, take_profit, entry_time, exit_time, status, COALESCE(pnl, 0)
	FROM positions
	WHERE id = ?`

	row := r.db.QueryRowContext(ctx, query, id)
	pos, err := scanPosition(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.logger.Debug(ctx, "Position not found by ID", map[string]interface{}{"positionID": id})
			return nil, nil // Not an error, just not found
		}
		return nil, fmt.Errorf("failed to query position by ID %d: %w", id, err)
	}
	return pos, nil
}

// FindAll retrieves all positions, ordered by entry time descending.
func (r *Repository) FindAll(ctx context.Context) ([]*domain.Position, error) {
	const query = `
	SELECT id, symbol, entry_price, COALESCE(exit_price, 0), quantity, leverage,
	       stop_loss, take_profit, entry_time, exit_time, status, COALESCE(pnl, 0)
	FROM positions
	ORDER BY entry_time DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query all positions: %w", err)
	}
	defer rows.Close()

	positions := make([]*domain.Position, 0)
	for rows.Next() {
		pos, err := scanPosition(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan position during FindAll: %w", err)
		}
		positions = append(positions, pos)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating position rows: %w", err)
	}
	return positions, nil
}

// GetTotalProfit calculates the sum of PNL for all closed positions.
func (r *Repository) GetTotalProfit(ctx context.Context) (float64, error) {
	const query = `SELECT COALESCE(SUM(pnl), 0) FROM positions WHERE status = ?`
	var totalProfit float64
	err := r.db.QueryRowContext(ctx, query, domain.StatusClosed).Scan(&totalProfit)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate total profit: %w", err)
	}
	return totalProfit, nil
}

// --- TradeRepository Implementation ---

// CreateTrade saves a new trade record and returns its assigned ID.
func (r *Repository) CreateTrade(ctx context.Context, trade *domain.Trade) (int64, error) {
	const query = `
    	INSERT INTO trade_history (symbol, entry_price, exit_price, quantity, leverage, pnl,
    	                           entry_time, exit_time, position_id, close_reason)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var positionID sql.NullInt64
	if trade.PositionID != 0 {
		positionID = sql.NullInt64{Int64: trade.PositionID, Valid: true}
	}

	result, err := r.db.ExecContext(ctx, query,
		trade.Symbol, trade.EntryPrice, trade.ExitPrice, trade.Quantity, trade.Leverage, trade.PNL,
		trade.EntryTime, trade.ExitTime, positionID, trade.CloseReason)
	if err != nil {
		return 0, fmt.Errorf("failed to insert trade history for symbol %s: %w", trade.Symbol, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID for trade history %s: %w", trade.Symbol, err)
	}
	trade.ID = id // Update domain object
	r.logger.Debug(ctx, "Trade history created", map[string]interface{}{"tradeID": id, "symbol": trade.Symbol, "pnl": trade.PNL})
	return id, nil
}

// FindBySymbol retrieves the most recent trades for a given symbol, up to a limit.
func (r *Repository) FindBySymbol(ctx context.Context, symbol string, limit int) ([]*domain.Trade, error) {
	const query = `
	SELECT id, symbol, entry_price, exit_price, quantity, leverage, pnl,
	       entry_time, exit_time, position_id, close_reason
	FROM trade_history
	WHERE symbol = ? ORDER BY entry_time DESC LIMIT ?`

	rows, err := r.db.QueryContext(ctx, query, symbol, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query trade history for symbol %s: %w", symbol, err)
	}
	defer rows.Close()

	trades := make([]*domain.Trade, 0)
	for rows.Next() {
		trade, err := scanTrade(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan trade history during FindBySymbol: %w", err)
		}
		trades = append(trades, trade)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating trade history rows: %w", err)
	}
	return trades, nil
}

// CountTodayBySymbol counts the number of trades executed today for a given symbol.
func (r *Repository) CountTodayBySymbol(ctx context.Context, symbol string) (int, error) {
	// Ensure timezone consistency might be needed depending on SQLite build/config
	const query = `SELECT COUNT(*) FROM trade_history WHERE symbol = ? AND date(entry_time) = date('now', 'localtime')`
	var count int
	err := r.db.QueryRowContext(ctx, query, symbol).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count trades today for symbol %s: %w", symbol, err)
	}
	return count, nil
}

// --- Helper Scan Functions ---

// scanner defines an interface compatible with *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...interface{}) error
}

// scanPosition scans a row into a domain.Position struct.
func scanPosition(s scanner) (*domain.Position, error) {
	p := &domain.Position{}
	var exitTime sql.NullTime
	var status string
	err := s.Scan(
		&p.ID, &p.Symbol, &p.EntryPrice, &p.ExitPrice, &p.Quantity, &p.Leverage,
		&p.StopLoss, &p.TakeProfit, &p.EntryTime, &exitTime, &status, &p.PNL)
	if err != nil {
		return nil, err // Handle sql.ErrNoRows in the caller
	}
	if exitTime.Valid {
		p.ExitTime = exitTime.Time
	}
	p.Status = domain.PositionStatus(status) // Convert string to domain type
	return p, nil
}

// scanTrade scans a row into a domain.Trade struct.
func scanTrade(s scanner) (*domain.Trade, error) {
	th := &domain.Trade{}
	var positionID sql.NullInt64
	var closeReason sql.NullString
	err := s.Scan(
		&th.ID, &th.Symbol, &th.EntryPrice, &th.ExitPrice, &th.Quantity, &th.Leverage, &th.PNL,
		&th.EntryTime, &th.ExitTime, &positionID, &closeReason)
	if err != nil {
		return nil, err // Handle sql.ErrNoRows in the caller
	}
	if positionID.Valid {
		th.PositionID = positionID.Int64
	}
	if closeReason.Valid {
		th.CloseReason = domain.CloseReason(closeReason.String)
	} else {
		th.CloseReason = domain.CloseReasonUnknown // Default if NULL
	}
	return th, nil
}
