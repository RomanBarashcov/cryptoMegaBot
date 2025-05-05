package strategies

import (
	"context"
	"cryptoMegaBot/internal/domain"
	"cryptoMegaBot/internal/ports"
	"cryptoMegaBot/internal/strategy/indicators"
	"fmt"
)

// MACrossoverConfig holds configuration for the Moving Average Crossover strategy
type MACrossoverConfig struct {
	ShortTermMAPeriod int
	LongTermMAPeriod  int
	EMAPeriod         int
	RSIPeriod         int
	RSIOverbought     float64
	RSIOversold       float64
}

// MACrossover implements the Moving Average Crossover strategy
type MACrossover struct {
	*BaseStrategy
	config      MACrossoverConfig
	shortTermMA *indicators.MovingAverage
	longTermMA  *indicators.MovingAverage
	ema         *indicators.MovingAverage
	rsi         *indicators.RSI
}

// NewMACrossover creates a new Moving Average Crossover strategy instance
func NewMACrossover(config MACrossoverConfig, logger ports.Logger) (*MACrossover, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is required for strategy")
	}

	// Validate configuration
	if config.ShortTermMAPeriod <= 0 || config.LongTermMAPeriod <= 0 || config.EMAPeriod <= 0 || config.RSIPeriod <= 0 {
		return nil, fmt.Errorf("strategy periods must be positive")
	}
	if config.ShortTermMAPeriod >= config.LongTermMAPeriod {
		return nil, fmt.Errorf("short term MA period must be less than long term MA period")
	}

	// Create indicators
	shortTermMA := indicators.NewMovingAverage(indicators.MovingAverageConfig{
		IndicatorConfig: indicators.IndicatorConfig{Period: config.ShortTermMAPeriod},
		Type:            indicators.SimpleMovingAverage,
	})

	longTermMA := indicators.NewMovingAverage(indicators.MovingAverageConfig{
		IndicatorConfig: indicators.IndicatorConfig{Period: config.LongTermMAPeriod},
		Type:            indicators.SimpleMovingAverage,
	})

	ema := indicators.NewMovingAverage(indicators.MovingAverageConfig{
		IndicatorConfig: indicators.IndicatorConfig{Period: config.EMAPeriod},
		Type:            indicators.ExponentialMovingAverage,
	})

	rsi := indicators.NewRSI(indicators.RSIConfig{
		IndicatorConfig: indicators.IndicatorConfig{Period: config.RSIPeriod},
		Overbought:      config.RSIOverbought,
		Oversold:        config.RSIOversold,
	})

	return &MACrossover{
		BaseStrategy: NewBaseStrategy(logger),
		config:       config,
		shortTermMA:  shortTermMA,
		longTermMA:   longTermMA,
		ema:          ema,
		rsi:          rsi,
	}, nil
}

// Name returns the name of the strategy
func (m *MACrossover) Name() string {
	return "Moving Average Crossover"
}

// RequiredDataPoints returns the minimum number of klines needed for the strategy
func (m *MACrossover) RequiredDataPoints() int {
	maxPeriod := m.config.LongTermMAPeriod
	if m.config.EMAPeriod > maxPeriod {
		maxPeriod = m.config.EMAPeriod
	}
	if m.config.RSIPeriod > maxPeriod {
		maxPeriod = m.config.RSIPeriod
	}
	return maxPeriod + 1
}

// ShouldEnterTrade implements the strategy's entry logic
func (m *MACrossover) ShouldEnterTrade(ctx context.Context, klines []*domain.Kline, currentPrice float64) bool {
	requiredPoints := m.RequiredDataPoints()
	if len(klines) < requiredPoints {
		m.logger.Debug(ctx, "Not enough kline data for strategy evaluation",
			map[string]interface{}{"available": len(klines), "required": requiredPoints})
		return false
	}

	// Calculate indicators
	shortTermMA, err := m.shortTermMA.Calculate(ctx, klines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate short term MA")
		return false
	}

	longTermMA, err := m.longTermMA.Calculate(ctx, klines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate long term MA")
		return false
	}

	ema, err := m.ema.Calculate(ctx, klines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate EMA")
		return false
	}

	rsi, err := m.rsi.Calculate(ctx, klines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate RSI")
		return false
	}

	// Entry conditions
	isTrendingUp := currentPrice > shortTermMA && currentPrice > longTermMA && shortTermMA > longTermMA
	isNotOverbought := !m.rsi.IsOverbought(rsi)
	isAboveEMA := currentPrice > ema

	if isTrendingUp && isNotOverbought && isAboveEMA {
		m.logger.Info(ctx, "Trade entry conditions met", map[string]interface{}{
			"currentPrice": currentPrice,
			"shortMA":      shortTermMA,
			"longMA":       longTermMA,
			"ema":          ema,
			"rsi":          rsi,
			"rsiLimit":     m.config.RSIOverbought,
		})
		return true
	}

	m.logger.Debug(ctx, "Trade entry conditions not met", map[string]interface{}{
		"currentPrice":    currentPrice,
		"shortMA":         shortTermMA,
		"longMA":          longTermMA,
		"ema":             ema,
		"rsi":             rsi,
		"isTrendingUp":    isTrendingUp,
		"isNotOverbought": isNotOverbought,
		"isAboveEMA":      isAboveEMA,
	})
	return false
}

// ShouldClosePosition implements the strategy's exit logic
func (m *MACrossover) ShouldClosePosition(ctx context.Context, position *domain.Position, klines []*domain.Kline, currentPrice float64) (bool, domain.CloseReason) {
	if !position.IsOpen() {
		return false, ""
	}

	// Check basic SL/TP
	if currentPrice <= position.StopLoss {
		m.logger.Info(ctx, "Stop loss condition met", map[string]interface{}{
			"positionID":   position.ID,
			"currentPrice": currentPrice,
			"stopLoss":     position.StopLoss,
		})
		return true, domain.CloseReasonStopLoss
	}
	if currentPrice >= position.TakeProfit {
		m.logger.Info(ctx, "Take profit condition met", map[string]interface{}{
			"positionID":   position.ID,
			"currentPrice": currentPrice,
			"takeProfit":   position.TakeProfit,
		})
		return true, domain.CloseReasonTakeProfit
	}

	// Additional exit conditions can be added here
	return false, ""
}
