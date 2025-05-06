package indicators

import (
	"context"
	"cryptoMegaBot/internal/domain"
	"testing"
	"time"
)

func TestRSI_Calculate(t *testing.T) {
	now := time.Now()
	klines := []*domain.Kline{
		{OpenTime: now.Add(-5 * time.Hour), Close: 100.0},
		{OpenTime: now.Add(-4 * time.Hour), Close: 102.0}, // +2
		{OpenTime: now.Add(-3 * time.Hour), Close: 101.0}, // -1
		{OpenTime: now.Add(-2 * time.Hour), Close: 103.0}, // +2
		{OpenTime: now.Add(-1 * time.Hour), Close: 102.0}, // -1
		{OpenTime: now, Close: 104.0},                     // +2
	}

	tests := []struct {
		name          string
		config        RSIConfig
		klines        []*domain.Kline
		expectedValue float64
		expectError   bool
	}{
		{
			name: "RSI with sufficient data",
			config: RSIConfig{
				IndicatorConfig: IndicatorConfig{Period: 3},
				Overbought:      70,
				Oversold:        30,
			},
			klines:        klines,
			expectedValue: 77.272727, // Actual RSI calculation using Wilder's smoothing
			expectError:   false,
		},
		{
			name: "Insufficient data",
			config: RSIConfig{
				IndicatorConfig: IndicatorConfig{Period: 7},
				Overbought:      70,
				Oversold:        30,
			},
			klines:        klines,
			expectedValue: 0,
			expectError:   true,
		},
		{
			name: "All gains",
			config: RSIConfig{
				IndicatorConfig: IndicatorConfig{Period: 3},
				Overbought:      70,
				Oversold:        30,
			},
			klines: []*domain.Kline{
				{OpenTime: now.Add(-3 * time.Hour), Close: 100.0},
				{OpenTime: now.Add(-2 * time.Hour), Close: 102.0},
				{OpenTime: now.Add(-1 * time.Hour), Close: 104.0},
				{OpenTime: now, Close: 106.0},
			},
			expectedValue: 100.0, // RSI should be 100 when there are only gains
			expectError:   false,
		},
		{
			name: "All losses",
			config: RSIConfig{
				IndicatorConfig: IndicatorConfig{Period: 3},
				Overbought:      70,
				Oversold:        30,
			},
			klines: []*domain.Kline{
				{OpenTime: now.Add(-3 * time.Hour), Close: 106.0},
				{OpenTime: now.Add(-2 * time.Hour), Close: 104.0},
				{OpenTime: now.Add(-1 * time.Hour), Close: 102.0},
				{OpenTime: now, Close: 100.0},
			},
			expectedValue: 0.0, // RSI should be 0 when there are only losses
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rsi := NewRSI(tt.config)
			value, err := rsi.Calculate(context.Background(), tt.klines)

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

			// Allow for small floating point differences
			if value-tt.expectedValue > 0.0001 || value-tt.expectedValue < -0.0001 {
				t.Errorf("Expected value %f, got %f", tt.expectedValue, value)
			}
		})
	}
}

func TestRSI_IsOverboughtOversold(t *testing.T) {
	config := RSIConfig{
		IndicatorConfig: IndicatorConfig{Period: 14},
		Overbought:      70,
		Oversold:        30,
	}

	tests := []struct {
		name         string
		value        float64
		isOverbought bool
		isOversold   bool
	}{
		{
			name:         "Overbought condition",
			value:        75.0,
			isOverbought: true,
			isOversold:   false,
		},
		{
			name:         "Oversold condition",
			value:        25.0,
			isOverbought: false,
			isOversold:   true,
		},
		{
			name:         "Neutral condition",
			value:        50.0,
			isOverbought: false,
			isOversold:   false,
		},
		{
			name:         "Exact overbought threshold",
			value:        70.0,
			isOverbought: true,
			isOversold:   false,
		},
		{
			name:         "Exact oversold threshold",
			value:        30.0,
			isOverbought: false,
			isOversold:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rsi := NewRSI(config)
			if overbought := rsi.IsOverbought(tt.value); overbought != tt.isOverbought {
				t.Errorf("IsOverbought(%f) = %v, want %v", tt.value, overbought, tt.isOverbought)
			}
			if oversold := rsi.IsOversold(tt.value); oversold != tt.isOversold {
				t.Errorf("IsOversold(%f) = %v, want %v", tt.value, oversold, tt.isOversold)
			}
		})
	}
}

func TestRSI_Name(t *testing.T) {
	rsi := NewRSI(RSIConfig{})
	if name := rsi.Name(); name != "RSI" {
		t.Errorf("Expected name 'RSI', got '%s'", name)
	}
}
