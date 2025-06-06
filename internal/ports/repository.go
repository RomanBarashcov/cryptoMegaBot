package ports

import (
	"context"

	"cryptoMegaBot/internal/domain"
)

// PositionRepository defines the interface for storing and retrieving trading positions.
type PositionRepository interface {
	// Create saves a new position and returns its assigned ID.
	Create(ctx context.Context, pos *domain.Position) (int64, error)
	// Update modifies an existing position.
	Update(ctx context.Context, pos *domain.Position) error
	// FindOpenBySymbol retrieves the currently open position for a given symbol, if any.
	// Returns nil, nil if no open position is found.
	FindOpenBySymbol(ctx context.Context, symbol string) (*domain.Position, error)
	// FindByID retrieves a position by its unique ID.
	// Returns nil, nil if not found.
	FindByID(ctx context.Context, id int64) (*domain.Position, error)
	// FindAll retrieves all positions, ordered by entry time descending.
	FindAll(ctx context.Context) ([]*domain.Position, error)
	// GetTotalProfit calculates the sum of PNL for all closed positions.
	GetTotalProfit(ctx context.Context) (float64, error)
}

// TradeRepository defines the interface for querying trade-related data, now primarily from the positions table.
type TradeRepository interface {
	// CreateTrade is removed - handled by PositionRepository.Update when closing.

	// FindClosedBySymbol retrieves the most recent *closed* positions for a given symbol, up to a limit.
	// Note: Returns domain.Position objects which contain trade details.
	FindClosedBySymbol(ctx context.Context, symbol string, limit int) ([]*domain.Position, error)

	// CountTodayBySymbol counts the number of *closed* positions executed today for a given symbol.
	CountTodayBySymbol(ctx context.Context, symbol string) (int, error)
}
