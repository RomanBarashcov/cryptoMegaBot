package backtesting

import (
	"context"
	"cryptoMegaBot/internal/domain"
	"testing"
	"time"
)

// MockStrategy implements the Strategy interface for testing
type MockStrategy struct {
	shouldEnter bool
	shouldClose bool
	closeReason domain.CloseReason
}

func (m *MockStrategy) RequiredDataPoints() int {
	return 2
}

func (m *MockStrategy) Name() string {
	return "mock_strategy"
}

func (m *MockStrategy) ShouldEnterTrade(ctx context.Context, klines []*domain.Kline, currentPrice float64) bool {
	return m.shouldEnter
}

func (m *MockStrategy) ShouldClosePosition(ctx context.Context, position *domain.Position, klines []*domain.Kline, currentPrice float64) (bool, domain.CloseReason) {
	return m.shouldClose, m.closeReason
}

func TestBacktest(t *testing.T) {
	// Create test data
	now := time.Now()
	klines := []*domain.Kline{
		{OpenTime: now.Add(-2 * time.Hour), Close: 100.0},
		{OpenTime: now.Add(-1 * time.Hour), Close: 101.0},
		{OpenTime: now, Close: 102.0},
	}

	tests := []struct {
		name           string
		strategy       *MockStrategy
		config         BacktestConfig
		expectedTrades int
		expectedError  bool
	}{
		{
			name: "Successful backtest with one trade",
			strategy: &MockStrategy{
				shouldEnter: true,
				shouldClose: true,
				closeReason: domain.CloseReasonTakeProfit,
			},
			config: BacktestConfig{
				StartTime:    now.Add(-2 * time.Hour),
				EndTime:      now,
				InitialFunds: 1000.0,
				PositionSize: 1.0,
				StopLoss:     0.02,
				TakeProfit:   0.02,
				Symbol:       "BTCUSDT",
				Leverage:     1,
			},
			expectedTrades: 1,
			expectedError:  false,
		},
		{
			name: "Insufficient data points",
			strategy: &MockStrategy{
				shouldEnter: true,
			},
			config: BacktestConfig{
				StartTime:    now.Add(-2 * time.Hour),
				EndTime:      now,
				InitialFunds: 1000.0,
				PositionSize: 1.0,
				StopLoss:     0.02,
				TakeProfit:   0.02,
				Symbol:       "BTCUSDT",
				Leverage:     1,
			},
			expectedTrades: 0,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use different klines for insufficient data points test
			testKlines := klines
			if tt.name == "Insufficient data points" {
				testKlines = klines[:1] // Only provide 1 kline instead of 3
			}
			result, err := Backtest(context.Background(), tt.strategy, testKlines, tt.config)

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result.TotalTrades != tt.expectedTrades {
				t.Errorf("Expected %d trades, got %d", tt.expectedTrades, result.TotalTrades)
			}
		})
	}
}

func TestCalculatePNL(t *testing.T) {
	tests := []struct {
		name         string
		position     *domain.Position
		currentPrice float64
		expectedPNL  float64
	}{
		{
			name: "Profitable long position",
			position: &domain.Position{
				EntryPrice: 100.0,
				Quantity:   1.0,
				Leverage:   2,
			},
			currentPrice: 110.0,
			expectedPNL:  20.0, // (110 - 100) * 1 * 2
		},
		{
			name: "Losing long position",
			position: &domain.Position{
				EntryPrice: 100.0,
				Quantity:   1.0,
				Leverage:   2,
			},
			currentPrice: 90.0,
			expectedPNL:  -20.0, // (90 - 100) * 1 * 2
		},
		{
			name: "Zero PNL",
			position: &domain.Position{
				EntryPrice: 100.0,
				Quantity:   1.0,
				Leverage:   2,
			},
			currentPrice: 100.0,
			expectedPNL:  0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pnl := calculatePNL(tt.position, tt.currentPrice)
			if pnl != tt.expectedPNL {
				t.Errorf("Expected PNL %f, got %f", tt.expectedPNL, pnl)
			}
		})
	}
}

func TestCalculateSharpeRatio(t *testing.T) {
	tests := []struct {
		name          string
		returns       []float64
		expectedRatio float64
	}{
		{
			name:          "Positive returns",
			returns:       []float64{0.1, 0.2, 0.15},
			expectedRatio: 60.0, // mean: 0.15, std dev: 0.0025
		},
		{
			name:          "Negative returns",
			returns:       []float64{-0.1, -0.2, -0.15},
			expectedRatio: -60.0, // mean: -0.15, std dev: 0.0025
		},
		{
			name:          "Mixed returns",
			returns:       []float64{-0.1, 0.2, 0.0},
			expectedRatio: 1.428571, // mean: 0.033, std dev: 0.023
		},
		{
			name:          "Single return",
			returns:       []float64{0.1},
			expectedRatio: 0,
		},
		{
			name:          "Empty returns",
			returns:       []float64{},
			expectedRatio: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratio := calculateSharpeRatio(tt.returns)
			// Allow for small floating point differences
			if ratio-tt.expectedRatio > 0.0001 || ratio-tt.expectedRatio < -0.0001 {
				t.Errorf("Expected Sharpe ratio %f, got %f", tt.expectedRatio, ratio)
			}
		})
	}
}
