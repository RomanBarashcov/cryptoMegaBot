package risk

import (
	"context"
	"cryptoMegaBot/internal/domain"
	"fmt"
	"math"
	"time"
)

// RiskConfig holds configuration for risk management
type RiskConfig struct {
	MaxPositionSize     float64
	MaxLeverage         int
	MaxDrawdown         float64
	MaxDailyLoss        float64
	MaxOpenPositions    int
	PositionSizePercent float64
	StopLossPercent     float64
	TakeProfitPercent   float64
}

// RiskManager implements risk management functionality
type RiskManager struct {
	config RiskConfig
	stats  *RiskStats
}

// RiskStats holds risk management statistics
type RiskStats struct {
	DailyPnL        float64
	CurrentDrawdown float64
	OpenPositions   int
	TotalExposure   float64
	DailyTrades     int
	MaxDailyTrades  int
	LastResetTime   int64
}

// NewRiskManager creates a new risk manager instance
func NewRiskManager(config RiskConfig) *RiskManager {
	return &RiskManager{
		config: config,
		stats: &RiskStats{
			MaxDailyTrades: 100, // Default value, can be configured
		},
	}
}

// ValidatePosition validates if a new position can be opened
func (r *RiskManager) ValidatePosition(ctx context.Context, position *domain.Position, accountBalance float64) error {
	// Check position size
	if position.Quantity > r.config.MaxPositionSize {
		return fmt.Errorf("position size %f exceeds maximum allowed %f", position.Quantity, r.config.MaxPositionSize)
	}

	// Check leverage
	if position.Leverage > r.config.MaxLeverage {
		return fmt.Errorf("leverage %d exceeds maximum allowed %d", position.Leverage, r.config.MaxLeverage)
	}

	// Check number of open positions
	if r.stats.OpenPositions >= r.config.MaxOpenPositions {
		return fmt.Errorf("number of open positions %d exceeds maximum allowed %d", r.stats.OpenPositions, r.config.MaxOpenPositions)
	}

	// Check daily loss limit
	positionValue := position.Quantity * position.EntryPrice * float64(position.Leverage)
	if r.stats.DailyPnL-positionValue*r.config.StopLossPercent < -r.config.MaxDailyLoss*accountBalance {
		return fmt.Errorf("potential daily loss would exceed maximum allowed")
	}

	// Check total exposure
	newTotalExposure := r.stats.TotalExposure + positionValue
	if newTotalExposure > accountBalance*float64(r.config.MaxOpenPositions) {
		return fmt.Errorf("total exposure would exceed maximum allowed")
	}

	return nil
}

// UpdateStats updates risk management statistics
func (r *RiskManager) UpdateStats(ctx context.Context, trade *domain.Trade, accountBalance float64) {
	// Update daily PnL
	r.stats.DailyPnL += trade.PNL

	// Update drawdown
	if trade.PNL < 0 {
		r.stats.CurrentDrawdown = math.Max(r.stats.CurrentDrawdown, -trade.PNL/accountBalance)
	}

	// Update open positions count
	if trade.CloseReason == "" {
		r.stats.OpenPositions++
		r.stats.TotalExposure += trade.Quantity * trade.EntryPrice * float64(trade.Leverage)
	} else {
		r.stats.OpenPositions--
		r.stats.TotalExposure -= trade.Quantity * trade.EntryPrice * float64(trade.Leverage)
	}

	// Update daily trades count
	r.stats.DailyTrades++
}

// ResetDailyStats resets daily statistics
func (r *RiskManager) ResetDailyStats(ctx context.Context) {
	r.stats.DailyPnL = 0
	r.stats.DailyTrades = 0
	r.stats.LastResetTime = time.Now().Unix()
}

// GetPositionSize calculates the appropriate position size based on risk parameters
func (r *RiskManager) GetPositionSize(ctx context.Context, accountBalance float64, currentPrice float64) float64 {
	// Calculate position size based on account balance and risk parameters
	positionSize := accountBalance * r.config.PositionSizePercent / currentPrice

	// Ensure position size doesn't exceed maximum allowed
	return math.Min(positionSize, r.config.MaxPositionSize)
}

// GetStopLoss calculates the stop loss price for a position
func (r *RiskManager) GetStopLoss(ctx context.Context, entryPrice float64, isLong bool) float64 {
	if isLong {
		return entryPrice * (1 - r.config.StopLossPercent)
	}
	return entryPrice * (1 + r.config.StopLossPercent)
}

// GetTakeProfit calculates the take profit price for a position
func (r *RiskManager) GetTakeProfit(ctx context.Context, entryPrice float64, isLong bool) float64 {
	if isLong {
		return entryPrice * (1 + r.config.TakeProfitPercent)
	}
	return entryPrice * (1 - r.config.TakeProfitPercent)
}

// CheckRiskLimits checks if any risk limits have been exceeded
func (r *RiskManager) CheckRiskLimits(ctx context.Context, accountBalance float64) error {
	// Check drawdown limit
	if r.stats.CurrentDrawdown > r.config.MaxDrawdown {
		return fmt.Errorf("current drawdown %f exceeds maximum allowed %f", r.stats.CurrentDrawdown, r.config.MaxDrawdown)
	}

	// Check daily loss limit
	if r.stats.DailyPnL < -r.config.MaxDailyLoss*accountBalance {
		return fmt.Errorf("daily loss %f exceeds maximum allowed %f", r.stats.DailyPnL, -r.config.MaxDailyLoss*accountBalance)
	}

	// Check daily trades limit
	if r.stats.DailyTrades >= r.stats.MaxDailyTrades {
		return fmt.Errorf("daily trades %d exceeds maximum allowed %d", r.stats.DailyTrades, r.stats.MaxDailyTrades)
	}

	return nil
}

// GetStats returns the current risk management statistics
func (r *RiskManager) GetStats() *RiskStats {
	return r.stats
}
