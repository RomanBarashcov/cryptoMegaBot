package strategies

import (
	"context"
	"cryptoMegaBot/internal/domain"
	"cryptoMegaBot/internal/ports"
)

// Strategy defines the interface for trading strategies
type Strategy interface {
	// ShouldEnterTrade determines if a new trade should be entered
	ShouldEnterTrade(ctx context.Context, klines []*domain.Kline, currentPrice float64) bool

	// ShouldClosePosition determines if an open position should be closed
	ShouldClosePosition(ctx context.Context, position *domain.Position, klines []*domain.Kline, currentPrice float64) (bool, domain.CloseReason)

	// RequiredDataPoints returns the minimum number of klines needed for the strategy
	RequiredDataPoints() int

	// Name returns the name of the strategy
	Name() string
}

// BaseStrategy provides common functionality for strategies
type BaseStrategy struct {
	logger ports.Logger
}

// NewBaseStrategy creates a new base strategy instance
func NewBaseStrategy(logger ports.Logger) *BaseStrategy {
	return &BaseStrategy{
		logger: logger,
	}
}
