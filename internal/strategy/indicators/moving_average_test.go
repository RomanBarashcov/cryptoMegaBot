package indicators

import (
	"context"
	"cryptoMegaBot/internal/domain"
	"testing"
	"time"
)

func TestMovingAverage_Calculate(t *testing.T) {
	now := time.Now()
	klines := []*domain.Kline{
		{OpenTime: now.Add(-4 * time.Hour), Close: 100.0},
		{OpenTime: now.Add(-3 * time.Hour), Close: 102.0},
		{OpenTime: now.Add(-2 * time.Hour), Close: 101.0},
		{OpenTime: now.Add(-1 * time.Hour), Close: 103.0},
		{OpenTime: now, Close: 104.0},
	}

	tests := []struct {
		name          string
		config        MovingAverageConfig
		klines        []*domain.Kline
		expectedValue float64
		expectError   bool
	}{
		{
			name: "SMA with sufficient data",
			config: MovingAverageConfig{
				IndicatorConfig: IndicatorConfig{Period: 3},
				Type:            SimpleMovingAverage,
			},
			klines:        klines,
			expectedValue: 102.666667, // (101 + 103 + 104) / 3
			expectError:   false,
		},
		{
			name: "EMA with sufficient data",
			config: MovingAverageConfig{
				IndicatorConfig: IndicatorConfig{Period: 3},
				Type:            ExponentialMovingAverage,
			},
			klines:        klines,
			expectedValue: 103.0, // Actual EMA calculation
			expectError:   false,
		},
		{
			name: "Insufficient data",
			config: MovingAverageConfig{
				IndicatorConfig: IndicatorConfig{Period: 6},
				Type:            SimpleMovingAverage,
			},
			klines:        klines,
			expectedValue: 0,
			expectError:   true,
		},
		{
			name: "Invalid MA type",
			config: MovingAverageConfig{
				IndicatorConfig: IndicatorConfig{Period: 3},
				Type:            "INVALID",
			},
			klines:        klines,
			expectedValue: 0,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ma := NewMovingAverage(tt.config)
			value, err := ma.Calculate(context.Background(), tt.klines)

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

func TestMovingAverage_Name(t *testing.T) {
	tests := []struct {
		name     string
		config   MovingAverageConfig
		expected string
	}{
		{
			name: "SMA name",
			config: MovingAverageConfig{
				Type: SimpleMovingAverage,
			},
			expected: "SMA",
		},
		{
			name: "EMA name",
			config: MovingAverageConfig{
				Type: ExponentialMovingAverage,
			},
			expected: "EMA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ma := NewMovingAverage(tt.config)
			if name := ma.Name(); name != tt.expected {
				t.Errorf("Expected name %s, got %s", tt.expected, name)
			}
		})
	}
}
