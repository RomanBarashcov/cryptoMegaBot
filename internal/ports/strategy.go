package ports

import (
	"context"

	"cryptoMegaBot/internal/domain"
)

// Strategy defines the interface for trading strategies.
type Strategy interface {
	// RequiredDataPoints returns the minimum number of klines needed for the strategy calculations.
	RequiredDataPoints() int

	// ShouldEnterTrade implements the logic to decide if a trade should be entered.
	ShouldEnterTrade(ctx context.Context, klines []*domain.Kline, currentPrice float64) bool

	// ShouldClosePosition implements the logic to decide if an open position should be closed.
	ShouldClosePosition(ctx context.Context, position *domain.Position, klines []*domain.Kline, currentPrice float64) (bool, domain.CloseReason)
}
