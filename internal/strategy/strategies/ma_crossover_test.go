package strategies

import (
	"context"
	"cryptoMegaBot/internal/domain"
	"cryptoMegaBot/internal/ports"
	"testing"
	"time"
)

// MockLogger implements ports.Logger for testing
type MockLogger struct{}

func (m *MockLogger) Debug(ctx context.Context, msg string, fields ...map[string]interface{}) {}
func (m *MockLogger) Info(ctx context.Context, msg string, fields ...map[string]interface{})  {}
func (m *MockLogger) Warn(ctx context.Context, msg string, fields ...map[string]interface{})  {}
func (m *MockLogger) Error(ctx context.Context, err error, msg string, fields ...map[string]interface{}) {
}
func (m *MockLogger) Fatal(ctx context.Context, err error, msg string, fields ...map[string]interface{}) {
}

func TestNewMACrossover(t *testing.T) {
	tests := []struct {
		name        string
		config      MACrossoverConfig
		logger      ports.Logger
		expectError bool
	}{
		{
			name: "Valid configuration",
			config: MACrossoverConfig{
				ShortTermMAPeriod: 5,
				LongTermMAPeriod:  20,
				EMAPeriod:         10,
				RSIPeriod:         14,
				RSIOverbought:     70,
				RSIOversold:       30,
			},
			logger:      &MockLogger{},
			expectError: false,
		},
		{
			name: "Nil logger",
			config: MACrossoverConfig{
				ShortTermMAPeriod: 5,
				LongTermMAPeriod:  20,
				EMAPeriod:         10,
				RSIPeriod:         14,
				RSIOverbought:     70,
				RSIOversold:       30,
			},
			logger:      nil,
			expectError: true,
		},
		{
			name: "Invalid periods",
			config: MACrossoverConfig{
				ShortTermMAPeriod: 0,
				LongTermMAPeriod:  20,
				EMAPeriod:         10,
				RSIPeriod:         14,
				RSIOverbought:     70,
				RSIOversold:       30,
			},
			logger:      &MockLogger{},
			expectError: true,
		},
		{
			name: "Short term MA period >= Long term MA period",
			config: MACrossoverConfig{
				ShortTermMAPeriod: 20,
				LongTermMAPeriod:  20,
				EMAPeriod:         10,
				RSIPeriod:         14,
				RSIOverbought:     70,
				RSIOversold:       30,
			},
			logger:      &MockLogger{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, err := NewMACrossover(tt.config, tt.logger)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if strategy == nil {
				t.Error("Expected strategy instance but got nil")
			}
		})
	}
}

func TestMACrossover_ShouldEnterTrade(t *testing.T) {
	now := time.Now()
	// Create a more realistic price movement with some pullbacks to keep RSI from being overbought
	klines := []*domain.Kline{
		{OpenTime: now.Add(-25 * time.Hour), Close: 100.0},
		{OpenTime: now.Add(-24 * time.Hour), Close: 101.0},
		{OpenTime: now.Add(-23 * time.Hour), Close: 99.0}, // Pullback
		{OpenTime: now.Add(-22 * time.Hour), Close: 102.0},
		{OpenTime: now.Add(-21 * time.Hour), Close: 101.0}, // Pullback
		{OpenTime: now.Add(-20 * time.Hour), Close: 103.0},
		{OpenTime: now.Add(-19 * time.Hour), Close: 102.0}, // Pullback
		{OpenTime: now.Add(-18 * time.Hour), Close: 104.0},
		{OpenTime: now.Add(-17 * time.Hour), Close: 103.0}, // Pullback
		{OpenTime: now.Add(-16 * time.Hour), Close: 105.0},
		{OpenTime: now.Add(-15 * time.Hour), Close: 104.0}, // Pullback
		{OpenTime: now.Add(-14 * time.Hour), Close: 106.0},
		{OpenTime: now.Add(-13 * time.Hour), Close: 105.0}, // Pullback
		{OpenTime: now.Add(-12 * time.Hour), Close: 107.0},
		{OpenTime: now.Add(-11 * time.Hour), Close: 106.0}, // Pullback
		{OpenTime: now.Add(-10 * time.Hour), Close: 108.0},
		{OpenTime: now.Add(-9 * time.Hour), Close: 107.0}, // Pullback
		{OpenTime: now.Add(-8 * time.Hour), Close: 109.0},
		{OpenTime: now.Add(-7 * time.Hour), Close: 108.0}, // Pullback
		{OpenTime: now.Add(-6 * time.Hour), Close: 110.0},
		{OpenTime: now.Add(-5 * time.Hour), Close: 109.0}, // Pullback
		{OpenTime: now.Add(-4 * time.Hour), Close: 111.0},
		{OpenTime: now.Add(-3 * time.Hour), Close: 110.0}, // Pullback
		{OpenTime: now.Add(-2 * time.Hour), Close: 112.0},
		{OpenTime: now.Add(-1 * time.Hour), Close: 111.0}, // Pullback
		{OpenTime: now, Close: 112.0},
	}

	config := MACrossoverConfig{
		ShortTermMAPeriod: 5,
		LongTermMAPeriod:  20,
		EMAPeriod:         10,
		RSIPeriod:         14,
		RSIOverbought:     70,
		RSIOversold:       30,
	}

	tests := []struct {
		name          string
		currentPrice  float64
		expectedEntry bool
	}{
		{
			name:          "Entry conditions met",
			currentPrice:  112.0,
			expectedEntry: true,
		},
		{
			name:          "Price below short MA",
			currentPrice:  105.0,
			expectedEntry: false,
		},
		{
			name:          "Price below long MA",
			currentPrice:  100.0,
			expectedEntry: false,
		},
		{
			name:          "Price below EMA",
			currentPrice:  105.0,
			expectedEntry: false,
		},
		{
			name:          "RSI overbought",
			currentPrice:  112.0,
			expectedEntry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, err := NewMACrossover(config, &MockLogger{})
			if err != nil {
				t.Fatalf("Failed to create strategy: %v", err)
			}

			// For the RSI overbought test case, modify the klines to create an overbought condition
			if tt.name == "RSI overbought" {
				for i := range klines {
					klines[i].Close = 150.0 // Set all prices high to trigger overbought RSI
				}
			}

			// Calculate indicators manually for debugging
			shortTermMA, _ := strategy.shortTermMA.Calculate(context.Background(), klines)
			longTermMA, _ := strategy.longTermMA.Calculate(context.Background(), klines)
			ema, _ := strategy.ema.Calculate(context.Background(), klines)
			rsi, _ := strategy.rsi.Calculate(context.Background(), klines)

			t.Logf("Test case: %s", tt.name)
			t.Logf("Current price: %.2f", tt.currentPrice)
			t.Logf("Short term MA: %.2f", shortTermMA)
			t.Logf("Long term MA: %.2f", longTermMA)
			t.Logf("EMA: %.2f", ema)
			t.Logf("RSI: %.2f", rsi)
			t.Logf("Is trending up: %v", tt.currentPrice > shortTermMA && tt.currentPrice > longTermMA && shortTermMA > longTermMA)
			t.Logf("Is not overbought: %v", !strategy.rsi.IsOverbought(rsi))
			t.Logf("Is above EMA: %v", tt.currentPrice > ema)

			shouldEnter := strategy.ShouldEnterTrade(context.Background(), klines, tt.currentPrice)
			if shouldEnter != tt.expectedEntry {
				t.Errorf("Expected entry %v, got %v", tt.expectedEntry, shouldEnter)
			}

			// Reset klines for next test case
			if tt.name == "RSI overbought" {
				for i := range klines {
					klines[i].Close = 100.0 + float64(i)
				}
			}
		})
	}
}

func TestMACrossover_ShouldClosePosition(t *testing.T) {
	now := time.Now()
	position := &domain.Position{
		ID:         1,
		EntryPrice: 100.0,
		StopLoss:   95.0,
		TakeProfit: 110.0,
		EntryTime:  now.Add(-1 * time.Hour),
		Status:     domain.StatusOpen,
	}

	klines := []*domain.Kline{
		{OpenTime: now.Add(-2 * time.Hour), Close: 100.0},
		{OpenTime: now.Add(-1 * time.Hour), Close: 100.0},
		{OpenTime: now, Close: 100.0},
	}

	config := MACrossoverConfig{
		ShortTermMAPeriod: 5,
		LongTermMAPeriod:  20,
		EMAPeriod:         10,
		RSIPeriod:         14,
		RSIOverbought:     70,
		RSIOversold:       30,
	}

	tests := []struct {
		name           string
		currentPrice   float64
		expectedClose  bool
		expectedReason domain.CloseReason
	}{
		{
			name:           "Stop loss triggered",
			currentPrice:   94.0,
			expectedClose:  true,
			expectedReason: domain.CloseReasonStopLoss,
		},
		{
			name:           "Take profit triggered",
			currentPrice:   111.0,
			expectedClose:  true,
			expectedReason: domain.CloseReasonTakeProfit,
		},
		{
			name:           "No close conditions met",
			currentPrice:   100.0,
			expectedClose:  false,
			expectedReason: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, err := NewMACrossover(config, &MockLogger{})
			if err != nil {
				t.Fatalf("Failed to create strategy: %v", err)
			}

			shouldClose, reason := strategy.ShouldClosePosition(context.Background(), position, klines, tt.currentPrice)
			if shouldClose != tt.expectedClose {
				t.Errorf("Expected close %v, got %v", tt.expectedClose, shouldClose)
			}
			if reason != tt.expectedReason {
				t.Errorf("Expected reason %s, got %s", tt.expectedReason, reason)
			}
		})
	}
}

func TestMACrossover_Name(t *testing.T) {
	config := MACrossoverConfig{
		ShortTermMAPeriod: 5,
		LongTermMAPeriod:  20,
		EMAPeriod:         10,
		RSIPeriod:         14,
		RSIOverbought:     70,
		RSIOversold:       30,
	}

	strategy, err := NewMACrossover(config, &MockLogger{})
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}

	expectedName := "Moving Average Crossover"
	if name := strategy.Name(); name != expectedName {
		t.Errorf("Expected name %s, got %s", expectedName, name)
	}
}

func TestMACrossover_RequiredDataPoints(t *testing.T) {
	tests := []struct {
		name           string
		config         MACrossoverConfig
		expectedPoints int
	}{
		{
			name: "Long term MA has max period",
			config: MACrossoverConfig{
				ShortTermMAPeriod: 5,
				LongTermMAPeriod:  20,
				EMAPeriod:         10,
				RSIPeriod:         14,
				RSIOverbought:     70,
				RSIOversold:       30,
			},
			expectedPoints: 21, // Long term MA period + 1
		},
		{
			name: "EMA has max period",
			config: MACrossoverConfig{
				ShortTermMAPeriod: 5,
				LongTermMAPeriod:  10,
				EMAPeriod:         20,
				RSIPeriod:         14,
				RSIOverbought:     70,
				RSIOversold:       30,
			},
			expectedPoints: 21, // EMA period + 1
		},
		{
			name: "RSI has max period",
			config: MACrossoverConfig{
				ShortTermMAPeriod: 5,
				LongTermMAPeriod:  10,
				EMAPeriod:         15,
				RSIPeriod:         20,
				RSIOverbought:     70,
				RSIOversold:       30,
			},
			expectedPoints: 21, // RSI period + 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, err := NewMACrossover(tt.config, &MockLogger{})
			if err != nil {
				t.Fatalf("Failed to create strategy: %v", err)
			}

			if points := strategy.RequiredDataPoints(); points != tt.expectedPoints {
				t.Errorf("Expected %d data points, got %d", tt.expectedPoints, points)
			}
		})
	}
}
