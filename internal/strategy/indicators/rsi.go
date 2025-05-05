package indicators

import (
	"context"
	"cryptoMegaBot/internal/domain"
	"fmt"
)

// RSIConfig holds configuration for the RSI indicator
type RSIConfig struct {
	IndicatorConfig
	Overbought float64
	Oversold   float64
}

// RSI implements the Relative Strength Index indicator
type RSI struct {
	BaseIndicator
	config RSIConfig
}

// NewRSI creates a new RSI indicator instance
func NewRSI(config RSIConfig) *RSI {
	return &RSI{
		BaseIndicator: BaseIndicator{Config: config.IndicatorConfig},
		config:        config,
	}
}

// Name returns the name of the indicator
func (r *RSI) Name() string {
	return "RSI"
}

// Calculate computes the RSI value using Wilder's smoothing method
func (r *RSI) Calculate(ctx context.Context, klines []*domain.Kline) (float64, error) {
	if len(klines) <= r.Config.Period {
		return 0, fmt.Errorf("not enough data (%d) to calculate RSI for period %d", len(klines), r.Config.Period)
	}

	// Calculate price changes
	changes := make([]float64, 0, len(klines)-1)
	for i := 1; i < len(klines); i++ {
		change := klines[i].Close - klines[i-1].Close
		changes = append(changes, change)
	}

	// Calculate initial average gain and loss
	var avgGain, avgLoss float64
	for i := 0; i < r.Config.Period; i++ {
		if changes[i] > 0 {
			avgGain += changes[i]
		} else {
			avgLoss -= changes[i]
		}
	}
	avgGain /= float64(r.Config.Period)
	avgLoss /= float64(r.Config.Period)

	// Calculate smoothed average gain and loss using Wilder's smoothing
	for i := r.Config.Period; i < len(changes); i++ {
		if changes[i] > 0 {
			avgGain = (avgGain*float64(r.Config.Period-1) + changes[i]) / float64(r.Config.Period)
			avgLoss = (avgLoss * float64(r.Config.Period-1)) / float64(r.Config.Period)
		} else {
			avgGain = (avgGain * float64(r.Config.Period-1)) / float64(r.Config.Period)
			avgLoss = (avgLoss*float64(r.Config.Period-1) - changes[i]) / float64(r.Config.Period)
		}
	}

	// Handle edge cases
	if avgLoss == 0 {
		if avgGain == 0 {
			return 50, nil // Neutral if no change
		}
		return 100, nil // Max RSI if only gains
	}

	// Calculate RSI
	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))

	// Ensure RSI is within bounds
	if rsi > 100 {
		rsi = 100
	} else if rsi < 0 {
		rsi = 0
	}

	return rsi, nil
}

// IsOverbought checks if the RSI value indicates an overbought condition
func (r *RSI) IsOverbought(value float64) bool {
	return value >= r.config.Overbought
}

// IsOversold checks if the RSI value indicates an oversold condition
func (r *RSI) IsOversold(value float64) bool {
	return value <= r.config.Oversold
}
