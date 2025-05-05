package strategy

import (
	"context"
	"fmt"

	// "math" was removed by auto-formatter, confirming removal

	"cryptoMegaBot/internal/domain"
	"cryptoMegaBot/internal/ports"
)

// Config holds parameters for the trading strategy.
type Config struct {
	ShortTermMAPeriod int     // e.g., 20
	LongTermMAPeriod  int     // e.g., 50
	EMAPeriod         int     // e.g., 20
	RSIPeriod         int     // e.g., 14
	RSIOverbought     float64 // e.g., 70.0
	RSIOversold       float64 // e.g., 30.0 (Not used in current logic, but good to have)
}

// Strategy implements the trading logic.
type Strategy struct {
	cfg    Config
	logger ports.Logger
}

// New creates a new Strategy instance.
func New(cfg Config, logger ports.Logger) (*Strategy, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is required for strategy")
	}
	// Basic validation
	if cfg.ShortTermMAPeriod <= 0 || cfg.LongTermMAPeriod <= 0 || cfg.EMAPeriod <= 0 || cfg.RSIPeriod <= 0 {
		return nil, fmt.Errorf("strategy periods must be positive")
	}
	if cfg.ShortTermMAPeriod >= cfg.LongTermMAPeriod {
		return nil, fmt.Errorf("short term MA period must be less than long term MA period")
	}
	return &Strategy{cfg: cfg, logger: logger}, nil
}

// RequiredDataPoints returns the minimum number of klines needed for the strategy calculations.
// It's the max of all indicator periods + 1 (for RSI lookback).
func (s *Strategy) RequiredDataPoints() int {
	maxPeriod := s.cfg.LongTermMAPeriod // Start with LongTermMA
	if s.cfg.EMAPeriod > maxPeriod {
		maxPeriod = s.cfg.EMAPeriod
	}
	if s.cfg.RSIPeriod > maxPeriod {
		maxPeriod = s.cfg.RSIPeriod
	}
	// RSI calculation looks one step further back than its period
	return maxPeriod + 1
}

// calculateRSI calculates the Relative Strength Index (RSI) using Wilder's smoothing method.
func calculateRSI(klines []*domain.Kline, period int) (float64, error) {
	if len(klines) <= period {
		return 0, fmt.Errorf("not enough data (%d) to calculate RSI for period %d", len(klines), period)
	}

	// Calculate price changes
	changes := make([]float64, 0, len(klines)-1)
	for i := 1; i < len(klines); i++ {
		change := klines[i].Close - klines[i-1].Close
		changes = append(changes, change)
	}

	// Calculate initial average gain and loss
	var avgGain, avgLoss float64
	for i := 0; i < period; i++ {
		if changes[i] > 0 {
			avgGain += changes[i]
		} else {
			avgLoss -= changes[i]
		}
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	// Calculate smoothed average gain and loss using Wilder's smoothing
	for i := period; i < len(changes); i++ {
		if changes[i] > 0 {
			avgGain = (avgGain*float64(period-1) + changes[i]) / float64(period)
			avgLoss = (avgLoss * float64(period-1)) / float64(period)
		} else {
			avgGain = (avgGain * float64(period-1)) / float64(period)
			avgLoss = (avgLoss*float64(period-1) - changes[i]) / float64(period)
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

// calculateMovingAverage calculates the Simple Moving Average (SMA).
func calculateMovingAverage(klines []*domain.Kline, period int) (float64, error) {
	if len(klines) < period {
		return 0, fmt.Errorf("not enough data (%d) to calculate MA for period %d", len(klines), period)
	}

	total := 0.0
	// Sum the closing prices for the specified period
	for i := len(klines) - period; i < len(klines); i++ {
		total += klines[i].Close
	}
	return total / float64(period), nil
}

// calculateEMA calculates the Exponential Moving Average (EMA).
func calculateEMA(klines []*domain.Kline, period int) (float64, error) {
	if len(klines) < period {
		return 0, fmt.Errorf("not enough data (%d) to calculate EMA for period %d", len(klines), period)
	}

	multiplier := 2.0 / float64(period+1)

	// Calculate the initial SMA for the first 'period' klines as the seed value
	initialSMA, err := calculateMovingAverage(klines[:len(klines)-period+period], period)
	if err != nil {
		// This error case should ideally not happen if the length check above is correct,
		// but handle it defensively.
		return 0, fmt.Errorf("failed to calculate initial SMA for EMA seed: %w", err)
	}
	ema := initialSMA

	// Apply EMA formula for the rest of the klines
	// Start from the element after the initial SMA period
	for i := len(klines) - period + 1; i < len(klines); i++ {
		closePrice := klines[i].Close
		ema = (closePrice-ema)*multiplier + ema
	}

	return ema, nil
}

// ShouldEnterTrade implements the logic to decide if a trade should be entered.
// It now uses the injected configuration and logger.
func (s *Strategy) ShouldEnterTrade(ctx context.Context, klines []*domain.Kline, currentPrice float64) bool {
	requiredPoints := s.RequiredDataPoints()
	if len(klines) < requiredPoints {
		s.logger.Debug(ctx, "Not enough kline data for strategy evaluation",
			map[string]interface{}{"available": len(klines), "required": requiredPoints})
		return false
	}

	// Calculate indicators
	shortTermMA, err := calculateMovingAverage(klines, s.cfg.ShortTermMAPeriod)
	if err != nil {
		s.logger.Error(ctx, err, "Failed to calculate short term MA")
		return false
	}

	longTermMA, err := calculateMovingAverage(klines, s.cfg.LongTermMAPeriod)
	if err != nil {
		s.logger.Error(ctx, err, "Failed to calculate long term MA")
		return false
	}

	ema, err := calculateEMA(klines, s.cfg.EMAPeriod)
	if err != nil {
		s.logger.Error(ctx, err, "Failed to calculate EMA")
		return false
	}

	rsi, err := calculateRSI(klines, s.cfg.RSIPeriod)
	if err != nil {
		s.logger.Error(ctx, err, "Failed to calculate RSI")
		return false
	}

	// Entry conditions
	isTrendingUp := currentPrice > shortTermMA && currentPrice > longTermMA && shortTermMA > longTermMA
	isNotOverbought := rsi < s.cfg.RSIOverbought
	isAboveEMA := currentPrice > ema // Added EMA condition based on previous logic

	if isTrendingUp && isNotOverbought && isAboveEMA {
		s.logger.Info(ctx, "Trade entry conditions met", map[string]interface{}{
			"currentPrice": currentPrice,
			"shortMA":      shortTermMA,
			"longMA":       longTermMA,
			"ema":          ema,
			"rsi":          rsi,
			"rsiLimit":     s.cfg.RSIOverbought,
		})
		return true
	}

	s.logger.Debug(ctx, "Trade entry conditions not met", map[string]interface{}{
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

// ShouldClosePosition implements the logic to decide if an open position should be closed.
// This is separate from SL/TP which might be handled by exchange order types.
// This could implement trailing stops or other exit conditions based on indicators.
func (s *Strategy) ShouldClosePosition(ctx context.Context, position *domain.Position, klines []*domain.Kline, currentPrice float64) (bool, domain.CloseReason) {
	// Placeholder: Implement exit strategy logic here if needed beyond basic SL/TP.
	// Example: Close if RSI crosses below 50 from above.
	// Example: Implement a trailing stop loss.

	// Check basic SL/TP (although exchange orders might handle this)
	if position.IsOpen() {
		if currentPrice <= position.StopLoss {
			s.logger.Info(ctx, "Stop loss condition met", map[string]interface{}{"positionID": position.ID, "currentPrice": currentPrice, "stopLoss": position.StopLoss})
			return true, domain.CloseReasonStopLoss
		}
		if currentPrice >= position.TakeProfit {
			s.logger.Info(ctx, "Take profit condition met", map[string]interface{}{"positionID": position.ID, "currentPrice": currentPrice, "takeProfit": position.TakeProfit})
			return true, domain.CloseReasonTakeProfit
		}
	}

	// No other conditions met
	return false, ""
}
