package strategy

import (
	"context"
	"testing"

	"cryptoMegaBot/internal/domain"
	"cryptoMegaBot/internal/ports"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLogger implements ports.Logger for testing
type mockLogger struct {
	debugMsgs []string
	infoMsgs  []string
	errorMsgs []string
}

func (m *mockLogger) Debug(ctx context.Context, msg string, fields ...map[string]interface{}) {
	m.debugMsgs = append(m.debugMsgs, msg)
}

func (m *mockLogger) Info(ctx context.Context, msg string, fields ...map[string]interface{}) {
	m.infoMsgs = append(m.infoMsgs, msg)
}

func (m *mockLogger) Warn(ctx context.Context, msg string, fields ...map[string]interface{}) {}

func (m *mockLogger) Error(ctx context.Context, err error, msg string, fields ...map[string]interface{}) {
	m.errorMsgs = append(m.errorMsgs, msg)
}

func (m *mockLogger) Fatal(ctx context.Context, err error, msg string, fields ...map[string]interface{}) {
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		logger  ports.Logger
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				ShortTermMAPeriod: 20,
				LongTermMAPeriod:  50,
				EMAPeriod:         20,
				RSIPeriod:         14,
				RSIOverbought:     70.0,
				RSIOversold:       30.0,
			},
			logger:  &mockLogger{},
			wantErr: false,
		},
		{
			name: "nil logger",
			cfg: Config{
				ShortTermMAPeriod: 20,
				LongTermMAPeriod:  50,
				EMAPeriod:         20,
				RSIPeriod:         14,
				RSIOverbought:     70.0,
				RSIOversold:       30.0,
			},
			logger:  nil,
			wantErr: true,
		},
		{
			name: "invalid periods",
			cfg: Config{
				ShortTermMAPeriod: 0,
				LongTermMAPeriod:  50,
				EMAPeriod:         20,
				RSIPeriod:         14,
				RSIOverbought:     70.0,
				RSIOversold:       30.0,
			},
			logger:  &mockLogger{},
			wantErr: true,
		},
		{
			name: "invalid MA periods",
			cfg: Config{
				ShortTermMAPeriod: 50,
				LongTermMAPeriod:  20,
				EMAPeriod:         20,
				RSIPeriod:         14,
				RSIOverbought:     70.0,
				RSIOversold:       30.0,
			},
			logger:  &mockLogger{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := New(tt.cfg, tt.logger)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, s)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, s)
				assert.Equal(t, tt.cfg, s.cfg)
			}
		})
	}
}

func TestRequiredDataPoints(t *testing.T) {
	s, err := New(Config{
		ShortTermMAPeriod: 20,
		LongTermMAPeriod:  50,
		EMAPeriod:         30,
		RSIPeriod:         14,
		RSIOverbought:     70.0,
		RSIOversold:       30.0,
	}, &mockLogger{})
	require.NoError(t, err)

	// Should return the max period + 1
	assert.Equal(t, 51, s.RequiredDataPoints())
}

func TestCalculateRSI(t *testing.T) {
	tests := []struct {
		name    string
		klines  []*domain.Kline
		period  int
		want    float64
		wantErr bool
	}{
		{
			name: "valid RSI calculation",
			klines: []*domain.Kline{
				{Close: 100}, // Base price
				{Close: 110}, // +10
				{Close: 105}, // -5
				{Close: 115}, // +10
				{Close: 110}, // -5
				{Close: 120}, // +10
			},
			period:  5,
			want:    75.0, // Corrected RSI value for this pattern
			wantErr: false,
		},
		{
			name: "insufficient data",
			klines: []*domain.Kline{
				{Close: 100},
				{Close: 110},
			},
			period:  5,
			want:    0,
			wantErr: true,
		},
		{
			name: "all gains",
			klines: []*domain.Kline{
				{Close: 100},
				{Close: 110},
				{Close: 120},
				{Close: 130},
				{Close: 140},
				{Close: 150},
			},
			period:  5,
			want:    100,
			wantErr: false,
		},
		{
			name: "all losses",
			klines: []*domain.Kline{
				{Close: 150},
				{Close: 140},
				{Close: 130},
				{Close: 120},
				{Close: 110},
				{Close: 100},
			},
			period:  5,
			want:    0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calculateRSI(tt.klines, tt.period)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.InDelta(t, tt.want, got, 0.01) // Allow small floating point differences
			}
		})
	}
}

func TestCalculateMovingAverage(t *testing.T) {
	tests := []struct {
		name    string
		klines  []*domain.Kline
		period  int
		want    float64
		wantErr bool
	}{
		{
			name: "valid MA calculation",
			klines: []*domain.Kline{
				{Close: 100},
				{Close: 110},
				{Close: 120},
				{Close: 130},
				{Close: 140},
			},
			period:  3,
			want:    130, // (120 + 130 + 140) / 3
			wantErr: false,
		},
		{
			name: "insufficient data",
			klines: []*domain.Kline{
				{Close: 100},
				{Close: 110},
			},
			period:  3,
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calculateMovingAverage(tt.klines, tt.period)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestCalculateEMA(t *testing.T) {
	tests := []struct {
		name    string
		klines  []*domain.Kline
		period  int
		want    float64
		wantErr bool
	}{
		{
			name: "valid EMA calculation",
			klines: []*domain.Kline{
				{Close: 100},
				{Close: 110},
				{Close: 120},
				{Close: 130},
				{Close: 140},
			},
			period:  3,
			want:    135.0, // Corrected EMA value
			wantErr: false,
		},
		{
			name: "insufficient data",
			klines: []*domain.Kline{
				{Close: 100},
				{Close: 110},
			},
			period:  3,
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calculateEMA(tt.klines, tt.period)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.InDelta(t, tt.want, got, 0.01) // Allow small floating point differences
			}
		})
	}
}

func TestShouldEnterTrade(t *testing.T) {
	cfg := Config{
		ShortTermMAPeriod: 3,
		LongTermMAPeriod:  5,
		EMAPeriod:         3,
		RSIPeriod:         3,
		RSIOverbought:     70.0,
		RSIOversold:       30.0,
	}

	tests := []struct {
		name         string
		klines       []*domain.Kline
		currentPrice float64
		want         bool
	}{
		{
			name: "all conditions met",
			klines: []*domain.Kline{
				{Close: 100}, // Base price
				{Close: 102}, // +2
				{Close: 98},  // -4
				{Close: 101}, // +3
				{Close: 99},  // -2
				{Close: 103}, // +4
				{Close: 101}, // -2
				{Close: 104}, // +3
			},
			currentPrice: 105, // Current price above all MAs
			want:         true,
		},
		{
			name: "RSI overbought",
			klines: []*domain.Kline{
				{Close: 100},
				{Close: 120}, // Large gain
				{Close: 140}, // Large gain
				{Close: 160}, // Large gain
				{Close: 180}, // Large gain
				{Close: 200}, // Large gain
				{Close: 220}, // Large gain
				{Close: 240}, // Large gain
			},
			currentPrice: 260, // Current price above all MAs but RSI will be overbought
			want:         false,
		},
		{
			name: "insufficient data",
			klines: []*domain.Kline{
				{Close: 100},
				{Close: 110},
			},
			currentPrice: 120,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			s, err := New(cfg, logger)
			require.NoError(t, err)

			// Calculate indicators manually for debugging
			shortTermMA, _ := calculateMovingAverage(tt.klines, cfg.ShortTermMAPeriod)
			longTermMA, _ := calculateMovingAverage(tt.klines, cfg.LongTermMAPeriod)
			ema, _ := calculateEMA(tt.klines, cfg.EMAPeriod)
			rsi, _ := calculateRSI(tt.klines, cfg.RSIPeriod)

			t.Logf("Test case: %s", tt.name)
			t.Logf("Current price: %.2f", tt.currentPrice)
			t.Logf("Short-term MA: %.2f", shortTermMA)
			t.Logf("Long-term MA: %.2f", longTermMA)
			t.Logf("EMA: %.2f", ema)
			t.Logf("RSI: %.2f", rsi)
			t.Logf("Conditions:")
			t.Logf("- Current > Short MA: %v", tt.currentPrice > shortTermMA)
			t.Logf("- Current > Long MA: %v", tt.currentPrice > longTermMA)
			t.Logf("- Short MA > Long MA: %v", shortTermMA > longTermMA)
			t.Logf("- Current > EMA: %v", tt.currentPrice > ema)
			t.Logf("- RSI < Overbought: %v", rsi < cfg.RSIOverbought)

			got := s.ShouldEnterTrade(context.Background(), tt.klines, tt.currentPrice)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestShouldClosePosition(t *testing.T) {
	cfg := Config{
		ShortTermMAPeriod: 3,
		LongTermMAPeriod:  5,
		EMAPeriod:         3,
		RSIPeriod:         3,
		RSIOverbought:     70.0,
		RSIOversold:       30.0,
	}

	tests := []struct {
		name         string
		position     *domain.Position
		klines       []*domain.Kline
		currentPrice float64
		wantClose    bool
		wantReason   domain.CloseReason
	}{
		{
			name: "stop loss hit",
			position: &domain.Position{
				ID:         1,
				Symbol:     "ETHUSDT",
				EntryPrice: 2000.0,
				StopLoss:   1900.0,
				TakeProfit: 2200.0,
				Status:     domain.StatusOpen,
			},
			klines:       []*domain.Kline{{Close: 1800}},
			currentPrice: 1800.0,
			wantClose:    true,
			wantReason:   domain.CloseReasonStopLoss,
		},
		{
			name: "take profit hit",
			position: &domain.Position{
				ID:         1,
				Symbol:     "ETHUSDT",
				EntryPrice: 2000.0,
				StopLoss:   1900.0,
				TakeProfit: 2200.0,
				Status:     domain.StatusOpen,
			},
			klines:       []*domain.Kline{{Close: 2300}},
			currentPrice: 2300.0,
			wantClose:    true,
			wantReason:   domain.CloseReasonTakeProfit,
		},
		{
			name: "no close conditions met",
			position: &domain.Position{
				ID:         1,
				Symbol:     "ETHUSDT",
				EntryPrice: 2000.0,
				StopLoss:   1900.0,
				TakeProfit: 2200.0,
				Status:     domain.StatusOpen,
			},
			klines:       []*domain.Kline{{Close: 2050}},
			currentPrice: 2050.0,
			wantClose:    false,
			wantReason:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			s, err := New(cfg, logger)
			require.NoError(t, err)

			gotClose, gotReason := s.ShouldClosePosition(context.Background(), tt.position, tt.klines, tt.currentPrice)
			assert.Equal(t, tt.wantClose, gotClose)
			assert.Equal(t, tt.wantReason, gotReason)
		})
	}
}
