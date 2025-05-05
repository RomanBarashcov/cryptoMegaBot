package indicators

import (
	"context"
	"cryptoMegaBot/internal/domain"
)

// Indicator represents a technical indicator that can be calculated from price data
type Indicator interface {
	// Calculate computes the indicator value for the given price data
	Calculate(ctx context.Context, klines []*domain.Kline) (float64, error)

	// RequiredDataPoints returns the minimum number of klines needed for calculation
	RequiredDataPoints() int

	// Name returns the name of the indicator
	Name() string
}

// IndicatorConfig holds common configuration for indicators
type IndicatorConfig struct {
	Period int
}

// BaseIndicator provides common functionality for indicators
type BaseIndicator struct {
	Config IndicatorConfig
}

// RequiredDataPoints returns the minimum number of klines needed for calculation
func (b *BaseIndicator) RequiredDataPoints() int {
	return b.Config.Period
}
