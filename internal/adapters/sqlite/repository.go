package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// initializeSchema creates/updates tables.
// NOTE: This is basic. A migration tool (like migrate, sql-migrate, goose) is better for production.
func (r *Repository) initializeSchema(ctx context.Context) error {
	// Use the schema from init.sql (excluding DROP TABLE)
	const schema = `
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
	);

	-- Indexes for positions table
	CREATE INDEX IF NOT EXISTS idx_positions_symbol_status ON positions(symbol, status);
	CREATE INDEX IF NOT EXISTS idx_positions_entry_time ON positions(entry_time);

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
	`
	// Note: This simple ExecContext won't handle schema *changes* well (e.g., adding columns).
	// It only ensures tables/indexes/triggers exist.
	_, err := r.db.ExecContext(ctx, schema)
	if err != nil {
		// Check if the error is due to the trigger already existing (common if run multiple times)
		// This is a basic check, a migration tool handles this better.
		// Using strings.Contains requires importing "strings"
		if !strings.Contains(err.Error(), "trigger enforce_one_open_position already exists") {
			return fmt.Errorf("failed to execute schema initialization: %w", err)
		}
		r.logger.Debug(ctx, "Trigger enforce_one_open_position already exists, ignoring error.")
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
	INSERT INTO positions (symbol, entry_price, quantity, leverage, stop_loss, take_profit, entry_time, status,
	                       stop_loss_order_id, take_profit_order_id)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)` // Added placeholders for new fields

	// Use sql.NullString for nullable text fields
	var slOrderID, tpOrderID sql.NullString
	if pos.StopLossOrderID != nil {
		slOrderID = sql.NullString{String: *pos.StopLossOrderID, Valid: true}
	}
	if pos.TakeProfitOrderID != nil {
		tpOrderID = sql.NullString{String: *pos.TakeProfitOrderID, Valid: true}
	}

	result, err := r.db.ExecContext(ctx, query,
		pos.Symbol, pos.EntryPrice, pos.Quantity, pos.Leverage, pos.StopLoss, pos.TakeProfit, pos.EntryTime, pos.Status,
		slOrderID, tpOrderID) // Pass new nullable fields
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

// Update modifies an existing position based on its ID. Typically used when closing a position.
func (r *Repository) Update(ctx context.Context, pos *domain.Position) error {
	const query = `
	UPDATE positions
	SET exit_price = ?, exit_time = ?, status = ?, pnl = ?, close_reason = ?,
	    stop_loss_order_id = ?, take_profit_order_id = ?
	WHERE id = ?` // Removed fields that shouldn't change on close (entry_price, quantity, etc.)

	// Prepare nullable fields for update
	var exitPrice sql.NullFloat64
	if pos.ExitPrice != 0 { // Assuming 0 means not set for exit price
		exitPrice = sql.NullFloat64{Float64: pos.ExitPrice, Valid: true}
	}
	var exitTime sql.NullTime
	if !pos.ExitTime.IsZero() {
		exitTime = sql.NullTime{Time: pos.ExitTime, Valid: true}
	}
	var pnl sql.NullFloat64
	// Check if PNL is explicitly set (might be 0 legitimately)
	// Assuming PNL is always calculated on close for update
	pnl = sql.NullFloat64{Float64: pos.PNL, Valid: true}

	var closeReason sql.NullString
	if pos.CloseReason != "" {
		closeReason = sql.NullString{String: string(pos.CloseReason), Valid: true}
	}
	var slOrderID, tpOrderID sql.NullString
	if pos.StopLossOrderID != nil {
		slOrderID = sql.NullString{String: *pos.StopLossOrderID, Valid: true}
	}
	if pos.TakeProfitOrderID != nil {
		tpOrderID = sql.NullString{String: *pos.TakeProfitOrderID, Valid: true}
	}

	result, err := r.db.ExecContext(ctx, query,
		exitPrice, exitTime, pos.Status, pnl, closeReason,
		slOrderID, tpOrderID, // Update order IDs as well (might be nullified if cancelled)
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
	// Updated SELECT to include all columns expected by scanPosition
	const query = `
	SELECT id, symbol, entry_price, exit_price, quantity, leverage,
	       stop_loss, take_profit, entry_time, exit_time, status, pnl,
	       stop_loss_order_id, take_profit_order_id, close_reason
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
	// Updated SELECT to include all columns expected by scanPosition
	const query = `
	SELECT id, symbol, entry_price, exit_price, quantity, leverage,
	       stop_loss, take_profit, entry_time, exit_time, status, pnl,
	       stop_loss_order_id, take_profit_order_id, close_reason
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
	// Updated SELECT to include all columns expected by scanPosition
	const query = `
	SELECT id, symbol, entry_price, exit_price, quantity, leverage,
	       stop_loss, take_profit, entry_time, exit_time, status, pnl,
	       stop_loss_order_id, take_profit_order_id, close_reason
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

// --- TradeRepository Implementation (Using 'positions' table) ---

// CreateTrade is removed as closed positions are handled by PositionRepository.Update.

// FindClosedBySymbol retrieves the most recent *closed* positions for a given symbol, up to a limit.
// Note: Returns domain.Position objects, not domain.Trade.
func (r *Repository) FindClosedBySymbol(ctx context.Context, symbol string, limit int) ([]*domain.Position, error) {
	// Updated SELECT to fetch all position columns, filtering by closed status and ordering by exit time
	const query = `
	SELECT id, symbol, entry_price, exit_price, quantity, leverage,
	       stop_loss, take_profit, entry_time, exit_time, status, pnl,
	       stop_loss_order_id, take_profit_order_id, close_reason
	FROM positions
	WHERE symbol = ? AND status = ? ORDER BY exit_time DESC LIMIT ?`

	rows, err := r.db.QueryContext(ctx, query, symbol, domain.StatusClosed, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query closed positions for symbol %s: %w", symbol, err)
	}
	defer rows.Close()

	positions := make([]*domain.Position, 0)
	for rows.Next() {
		pos, err := scanPosition(rows) // Use the existing scanPosition helper
		if err != nil {
			return nil, fmt.Errorf("failed to scan closed position during FindClosedBySymbol: %w", err)
		}
		positions = append(positions, pos)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating closed position rows: %w", err)
	}
	return positions, nil
}

// CountTodayBySymbol counts the number of *closed* positions executed today for a given symbol.
func (r *Repository) CountTodayBySymbol(ctx context.Context, symbol string) (int, error) {
	// Query counts closed positions where exit_time is today (local time)
	// Ensure timezone consistency might be needed depending on SQLite build/config
	const query = `SELECT COUNT(*) FROM positions WHERE symbol = ? AND status = ? AND date(exit_time) = date('now', 'localtime')`
	var count int
	err := r.db.QueryRowContext(ctx, query, symbol, domain.StatusClosed).Scan(&count)
	if err != nil {
		// If no rows, count is 0, which is not an error here. Check specifically for NoRows.
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to count closed positions today for symbol %s: %w", symbol, err)
	}
	return count, nil
}

// --- Helper Scan Functions --- (scanTrade removed)

// scanner defines an interface compatible with *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...interface{}) error
}

// scanPosition scans a row into a domain.Position struct.
func scanPosition(s scanner) (*domain.Position, error) {
	p := &domain.Position{}
	var exitTime sql.NullTime
	var status string
	var pnl sql.NullFloat64 // Use NullFloat64 for nullable PNL
	var slOrderID sql.NullString
	var tpOrderID sql.NullString
	var closeReason sql.NullString
	var exitPrice sql.NullFloat64 // Add NullFloat64 for exit_price

	// Ensure the Scan call matches the SELECT query columns exactly
	err := s.Scan(
		&p.ID, &p.Symbol, &p.EntryPrice, &exitPrice, &p.Quantity, &p.Leverage,
		&p.StopLoss, &p.TakeProfit, &p.EntryTime, &exitTime, &status, &pnl,
		&slOrderID, &tpOrderID, &closeReason, // Scan new columns
	)
	if err != nil {
		return nil, err // Handle sql.ErrNoRows in the caller
	}

	if exitTime.Valid {
		p.ExitTime = exitTime.Time
	}
	if exitPrice.Valid {
		p.ExitPrice = exitPrice.Float64
	}
	if pnl.Valid {
		p.PNL = pnl.Float64 // Assign if not NULL
	} else {
		p.PNL = 0 // Default PNL to 0 if NULL
	}
	if slOrderID.Valid {
		p.StopLossOrderID = &slOrderID.String // Assign pointer if not NULL
	}
	if tpOrderID.Valid {
		p.TakeProfitOrderID = &tpOrderID.String // Assign pointer if not NULL
	}
	if closeReason.Valid {
		p.CloseReason = domain.CloseReason(closeReason.String) // Assign if not NULL
	} else {
		p.CloseReason = "" // Default to empty string if NULL
	}

	p.Status = domain.PositionStatus(status) // Convert string to domain type
	return p, nil
}

// scanTrade function removed.
