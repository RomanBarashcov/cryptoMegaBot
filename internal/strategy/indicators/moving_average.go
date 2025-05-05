package indicators

import (
	"context"
	"cryptoMegaBot/internal/domain"
	"fmt"
)

// MovingAverageType defines the type of moving average
type MovingAverageType string

const (
	// SimpleMovingAverage represents a simple moving average
	SimpleMovingAverage MovingAverageType = "SMA"
	// ExponentialMovingAverage represents an exponential moving average
	ExponentialMovingAverage MovingAverageType = "EMA"
)

// MovingAverageConfig holds configuration for moving average indicators
type MovingAverageConfig struct {
	IndicatorConfig
	Type MovingAverageType
}

// MovingAverage implements both SMA and EMA indicators
type MovingAverage struct {
	BaseIndicator
	config MovingAverageConfig
}

// NewMovingAverage creates a new moving average indicator instance
func NewMovingAverage(config MovingAverageConfig) *MovingAverage {
	return &MovingAverage{
		BaseIndicator: BaseIndicator{Config: config.IndicatorConfig},
		config:        config,
	}
}

// Name returns the name of the indicator
func (m *MovingAverage) Name() string {
	return string(m.config.Type)
}

// Calculate computes the moving average value based on the configured type
func (m *MovingAverage) Calculate(ctx context.Context, klines []*domain.Kline) (float64, error) {
	switch m.config.Type {
	case SimpleMovingAverage:
		return m.calculateSMA(klines)
	case ExponentialMovingAverage:
		return m.calculateEMA(klines)
	default:
		return 0, fmt.Errorf("unsupported moving average type: %s", m.config.Type)
	}
}

// calculateSMA computes the Simple Moving Average
func (m *MovingAverage) calculateSMA(klines []*domain.Kline) (float64, error) {
	if len(klines) < m.Config.Period {
		return 0, fmt.Errorf("not enough data (%d) to calculate SMA for period %d", len(klines), m.Config.Period)
	}

	total := 0.0
	for i := len(klines) - m.Config.Period; i < len(klines); i++ {
		total += klines[i].Close
	}
	return total / float64(m.Config.Period), nil
}

// calculateEMA computes the Exponential Moving Average
func (m *MovingAverage) calculateEMA(klines []*domain.Kline) (float64, error) {
	if len(klines) < m.Config.Period {
		return 0, fmt.Errorf("not enough data (%d) to calculate EMA for period %d", len(klines), m.Config.Period)
	}

	multiplier := 2.0 / float64(m.Config.Period+1)

	// Calculate initial SMA for the first 'period' klines
	initialSMA, err := m.calculateSMA(klines[:m.Config.Period])
	if err != nil {
		return 0, fmt.Errorf("failed to calculate initial SMA for EMA: %w", err)
	}
	ema := initialSMA

	// Apply EMA formula for the rest of the klines
	for i := m.Config.Period; i < len(klines); i++ {
		closePrice := klines[i].Close
		ema = (closePrice-ema)*multiplier + ema
	}

	return ema, nil
}
