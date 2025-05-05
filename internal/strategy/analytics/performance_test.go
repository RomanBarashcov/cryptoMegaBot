package analytics

import (
	"cryptoMegaBot/internal/domain"
	"testing"
	"time"
)

func TestAnalyzePerformance(t *testing.T) {
	// Create test data
	initialBalance := 10000.0
	trades := []*domain.Trade{
		{
			PositionID:  1,
			Symbol:      "BTCUSDT",
			EntryPrice:  50000,
			ExitPrice:   55000,
			Quantity:    0.1,
			Leverage:    2,
			PNL:         1000,
			EntryTime:   time.Now().Add(-24 * time.Hour),
			ExitTime:    time.Now(),
			CloseReason: domain.CloseReasonTakeProfit,
		},
		{
			PositionID:  2,
			Symbol:      "BTCUSDT",
			EntryPrice:  55000,
			ExitPrice:   50000,
			Quantity:    0.1,
			Leverage:    2,
			PNL:         -1000,
			EntryTime:   time.Now().Add(-12 * time.Hour),
			ExitTime:    time.Now().Add(-6 * time.Hour),
			CloseReason: domain.CloseReasonStopLoss,
		},
	}

	// Run analysis
	metrics := AnalyzePerformance(trades, initialBalance)

	// Verify basic metrics
	if metrics.TotalTrades != 2 {
		t.Errorf("Expected 2 total trades, got %d", metrics.TotalTrades)
	}
	if metrics.WinningTrades != 1 {
		t.Errorf("Expected 1 winning trade, got %d", metrics.WinningTrades)
	}
	if metrics.LosingTrades != 1 {
		t.Errorf("Expected 1 losing trade, got %d", metrics.LosingTrades)
	}
	if metrics.WinRate != 0.5 {
		t.Errorf("Expected 0.5 win rate, got %f", metrics.WinRate)
	}
	if metrics.TotalProfit != 0 {
		t.Errorf("Expected 0 total profit, got %f", metrics.TotalProfit)
	}
	if metrics.FinalBalance != initialBalance {
		t.Errorf("Expected final balance of %f, got %f", initialBalance, metrics.FinalBalance)
	}

	// Verify advanced metrics
	if metrics.MaxConsecutiveWins != 1 {
		t.Errorf("Expected 1 max consecutive wins, got %d", metrics.MaxConsecutiveWins)
	}
	if metrics.MaxConsecutiveLosses != 1 {
		t.Errorf("Expected 1 max consecutive losses, got %d", metrics.MaxConsecutiveLosses)
	}
	if metrics.AverageWin != 1000 {
		t.Errorf("Expected 1000 average win, got %f", metrics.AverageWin)
	}
	if metrics.AverageLoss != -1000 {
		t.Errorf("Expected -1000 average loss, got %f", metrics.AverageLoss)
	}
	if metrics.ProfitFactor != 1.0 {
		t.Errorf("Expected 1.0 profit factor, got %f", metrics.ProfitFactor)
	}
	if metrics.RiskRewardRatio != 1.0 {
		t.Errorf("Expected 1.0 risk reward ratio, got %f", metrics.RiskRewardRatio)
	}

	// Verify equity curve
	if len(metrics.EquityCurve) != 2 {
		t.Errorf("Expected 2 equity curve points, got %d", len(metrics.EquityCurve))
	}

	// Verify monthly returns
	monthlyReturns := metrics.GetMonthlyReturns()
	if len(monthlyReturns) != 1 {
		t.Errorf("Expected 1 monthly return, got %d", len(monthlyReturns))
	}
}

func TestAnalyzePerformanceEmptyTrades(t *testing.T) {
	metrics := AnalyzePerformance([]*domain.Trade{}, 10000.0)
	if metrics.TotalTrades != 0 {
		t.Errorf("Expected 0 total trades, got %d", metrics.TotalTrades)
	}
	if metrics.FinalBalance != 10000.0 {
		t.Errorf("Expected final balance of 10000.0, got %f", metrics.FinalBalance)
	}
}

func TestAnalyzePerformanceDrawdown(t *testing.T) {
	initialBalance := 10000.0
	trades := []*domain.Trade{
		{
			PositionID:  1,
			Symbol:      "BTCUSDT",
			EntryPrice:  50000,
			ExitPrice:   55000,
			Quantity:    0.1,
			Leverage:    2,
			PNL:         1000,
			EntryTime:   time.Now().Add(-24 * time.Hour),
			ExitTime:    time.Now().Add(-18 * time.Hour),
			CloseReason: domain.CloseReasonTakeProfit,
		},
		{
			PositionID:  2,
			Symbol:      "BTCUSDT",
			EntryPrice:  55000,
			ExitPrice:   45000,
			Quantity:    0.2,
			Leverage:    2,
			PNL:         -2200,
			EntryTime:   time.Now().Add(-12 * time.Hour),
			ExitTime:    time.Now().Add(-6 * time.Hour),
			CloseReason: domain.CloseReasonStopLoss,
		},
	}

	metrics := AnalyzePerformance(trades, initialBalance)

	// Verify drawdown metrics
	if metrics.MaxDrawdown != 0.2 {
		t.Errorf("Expected 0.2 max drawdown, got %f", metrics.MaxDrawdown)
	}
	if len(metrics.Drawdowns) != 1 {
		t.Errorf("Expected 1 drawdown period, got %d", len(metrics.Drawdowns))
	}
	if metrics.Drawdowns[0].Depth != 0.2 {
		t.Errorf("Expected 0.2 drawdown depth, got %f", metrics.Drawdowns[0].Depth)
	}
}

func TestAnalyzePerformanceConsecutiveTrades(t *testing.T) {
	initialBalance := 10000.0
	trades := []*domain.Trade{
		{
			PositionID:  1,
			Symbol:      "BTCUSDT",
			EntryPrice:  50000,
			ExitPrice:   55000,
			Quantity:    0.1,
			Leverage:    2,
			PNL:         1000,
			EntryTime:   time.Now().Add(-24 * time.Hour),
			ExitTime:    time.Now().Add(-18 * time.Hour),
			CloseReason: domain.CloseReasonTakeProfit,
		},
		{
			PositionID:  2,
			Symbol:      "BTCUSDT",
			EntryPrice:  55000,
			ExitPrice:   60000,
			Quantity:    0.1,
			Leverage:    2,
			PNL:         1000,
			EntryTime:   time.Now().Add(-12 * time.Hour),
			ExitTime:    time.Now().Add(-6 * time.Hour),
			CloseReason: domain.CloseReasonTakeProfit,
		},
	}

	metrics := AnalyzePerformance(trades, initialBalance)

	// Verify consecutive trades metrics
	if metrics.MaxConsecutiveWins != 2 {
		t.Errorf("Expected 2 max consecutive wins, got %d", metrics.MaxConsecutiveWins)
	}
	if metrics.MaxConsecutiveLosses != 0 {
		t.Errorf("Expected 0 max consecutive losses, got %d", metrics.MaxConsecutiveLosses)
	}
	if metrics.WinRate != 1.0 {
		t.Errorf("Expected 1.0 win rate, got %f", metrics.WinRate)
	}
}
