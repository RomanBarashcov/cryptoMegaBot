package risk

import (
	"context"
	"cryptoMegaBot/internal/domain"
	"testing"
	"time"
)

func TestRiskManager(t *testing.T) {
	// Create risk manager configuration
	config := RiskConfig{
		MaxPositionSize:     1.0,
		MaxLeverage:         5,
		MaxDrawdown:         0.1,
		MaxDailyLoss:        0.05,
		MaxOpenPositions:    3,
		PositionSizePercent: 0.1,
		StopLossPercent:     0.02,
		TakeProfitPercent:   0.04,
	}

	// Create risk manager
	manager := NewRiskManager(config)

	// Test position validation
	position := &domain.Position{
		Symbol:     "BTCUSDT",
		EntryPrice: 50000,
		Quantity:   0.5,
		Leverage:   3,
		Status:     domain.StatusOpen,
	}

	// Test valid position
	err := manager.ValidatePosition(context.Background(), position, 100000)
	if err != nil {
		t.Errorf("Expected no error for valid position, got %v", err)
	}

	// Test position size limit
	position.Quantity = 2.0
	err = manager.ValidatePosition(context.Background(), position, 100000)
	if err == nil {
		t.Error("Expected error for exceeding position size limit")
	}

	// Test leverage limit
	position.Quantity = 0.5
	position.Leverage = 10
	err = manager.ValidatePosition(context.Background(), position, 100000)
	if err == nil {
		t.Error("Expected error for exceeding leverage limit")
	}

	// Test position size calculation
	positionSize := manager.GetPositionSize(context.Background(), 100000, 50000)
	expectedSize := 100000 * 0.1 / 50000
	if positionSize != expectedSize {
		t.Errorf("Expected position size %f, got %f", expectedSize, positionSize)
	}

	// Test stop loss calculation
	stopLoss := manager.GetStopLoss(context.Background(), 50000, true)
	expectedStopLoss := 50000 * (1 - 0.02)
	if stopLoss != expectedStopLoss {
		t.Errorf("Expected stop loss %f, got %f", expectedStopLoss, stopLoss)
	}

	// Test take profit calculation
	takeProfit := manager.GetTakeProfit(context.Background(), 50000, true)
	expectedTakeProfit := 50000 * (1 + 0.04)
	if takeProfit != expectedTakeProfit {
		t.Errorf("Expected take profit %f, got %f", expectedTakeProfit, takeProfit)
	}
}

func TestRiskManagerStats(t *testing.T) {
	config := RiskConfig{
		MaxPositionSize:     1.0,
		MaxLeverage:         5,
		MaxDrawdown:         0.1,
		MaxDailyLoss:        0.05,
		MaxOpenPositions:    3,
		PositionSizePercent: 0.1,
		StopLossPercent:     0.02,
		TakeProfitPercent:   0.04,
	}

	manager := NewRiskManager(config)

	// Test stats update
	trade := &domain.Trade{
		PositionID:  1,
		Symbol:      "BTCUSDT",
		EntryPrice:  50000,
		ExitPrice:   49000,
		Quantity:    0.1,
		Leverage:    2,
		PNL:         -200,
		EntryTime:   time.Now().Add(-1 * time.Hour),
		ExitTime:    time.Now(),
		CloseReason: domain.CloseReasonStopLoss,
	}

	manager.UpdateStats(context.Background(), trade, 100000)

	// Verify stats
	stats := manager.GetStats()
	if stats.DailyPnL != -200 {
		t.Errorf("Expected daily PnL %f, got %f", -200.0, stats.DailyPnL)
	}
	if stats.CurrentDrawdown != 0.002 {
		t.Errorf("Expected current drawdown %f, got %f", 0.002, stats.CurrentDrawdown)
	}
	if stats.DailyTrades != 1 {
		t.Errorf("Expected 1 daily trade, got %d", stats.DailyTrades)
	}

	// Test risk limits
	err := manager.CheckRiskLimits(context.Background(), 100000)
	if err != nil {
		t.Errorf("Expected no error for within limits, got %v", err)
	}

	// Test daily loss limit
	manager.stats.DailyPnL = -6000
	err = manager.CheckRiskLimits(context.Background(), 100000)
	if err == nil {
		t.Error("Expected error for exceeding daily loss limit")
	}

	// Test drawdown limit
	manager.stats.DailyPnL = 0
	manager.stats.CurrentDrawdown = 0.15
	err = manager.CheckRiskLimits(context.Background(), 100000)
	if err == nil {
		t.Error("Expected error for exceeding drawdown limit")
	}

	// Test daily trades limit
	manager.stats.DailyPnL = 0
	manager.stats.CurrentDrawdown = 0
	manager.stats.DailyTrades = 101
	err = manager.CheckRiskLimits(context.Background(), 100000)
	if err == nil {
		t.Error("Expected error for exceeding daily trades limit")
	}
}

func TestRiskManagerReset(t *testing.T) {
	config := RiskConfig{
		MaxPositionSize:     1.0,
		MaxLeverage:         5,
		MaxDrawdown:         0.1,
		MaxDailyLoss:        0.05,
		MaxOpenPositions:    3,
		PositionSizePercent: 0.1,
		StopLossPercent:     0.02,
		TakeProfitPercent:   0.04,
	}

	manager := NewRiskManager(config)

	// Set some stats
	manager.stats.DailyPnL = 1000
	manager.stats.DailyTrades = 50

	// Reset stats
	manager.ResetDailyStats(context.Background())

	// Verify reset
	stats := manager.GetStats()
	if stats.DailyPnL != 0 {
		t.Errorf("Expected daily PnL 0, got %f", stats.DailyPnL)
	}
	if stats.DailyTrades != 0 {
		t.Errorf("Expected 0 daily trades, got %d", stats.DailyTrades)
	}
	if stats.LastResetTime == 0 {
		t.Error("Expected LastResetTime to be set")
	}
}
