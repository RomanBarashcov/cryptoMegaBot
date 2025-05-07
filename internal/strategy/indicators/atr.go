package indicators

import (
	"context"
	"cryptoMegaBot/internal/domain"
	"fmt"
	"math"
)

// ATRConfig holds configuration for the Average True Range indicator
type ATRConfig struct {
	IndicatorConfig
}

// ATR implements the Average True Range indicator
type ATR struct {
	config ATRConfig
}

// NewATR creates a new Average True Range indicator instance
func NewATR(config ATRConfig) *ATR {
	return &ATR{
		config: config,
	}
}

// Calculate computes the Average True Range value for the given klines
func (a *ATR) Calculate(ctx context.Context, klines []*domain.Kline) (float64, error) {
	period := a.config.Period
	if len(klines) < period+1 {
		return 0, fmt.Errorf("not enough data points for ATR calculation: need %d, got %d", period+1, len(klines))
	}

	// Calculate true ranges
	trueRanges := make([]float64, len(klines))

	// First TR is just the high-low range
	trueRanges[0] = klines[0].High - klines[0].Low

	// Calculate subsequent TRs
	for i := 1; i < len(klines); i++ {
		high := klines[i].High
		low := klines[i].Low
		prevClose := klines[i-1].Close

		// True Range is the greatest of:
		// 1. Current High - Current Low
		// 2. |Current High - Previous Close|
		// 3. |Current Low - Previous Close|
		tr1 := high - low
		tr2 := math.Abs(high - prevClose)
		tr3 := math.Abs(low - prevClose)

		trueRanges[i] = math.Max(tr1, math.Max(tr2, tr3))
	}

	// Calculate ATR using Wilder's smoothing method
	// First ATR is simple average of first 'period' true ranges
	atr := 0.0
	for i := 0; i < period; i++ {
		atr += trueRanges[i]
	}
	atr /= float64(period)

	// Apply smoothing formula for remaining periods
	for i := period; i < len(klines); i++ {
		atr = (atr*float64(period-1) + trueRanges[i]) / float64(period)
	}

	return atr, nil
}
