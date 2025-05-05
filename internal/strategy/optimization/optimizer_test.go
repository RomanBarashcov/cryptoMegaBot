package optimization

import (
	"context"
	"cryptoMegaBot/internal/domain"
	"cryptoMegaBot/internal/strategy/analytics"
	"cryptoMegaBot/internal/strategy/strategies"
	"testing"
	"time"
)

// MockStrategy implements the Strategy interface for testing
type MockStrategy struct {
	*strategies.BaseStrategy
	shouldEnter bool
	shouldClose bool
	closeReason domain.CloseReason
}

func NewMockStrategy(shouldEnter, shouldClose bool, closeReason domain.CloseReason) *MockStrategy {
	return &MockStrategy{
		BaseStrategy: strategies.NewBaseStrategy(nil),
		shouldEnter:  shouldEnter,
		shouldClose:  shouldClose,
		closeReason:  closeReason,
	}
}

func (m *MockStrategy) ShouldEnterTrade(ctx context.Context, klines []*domain.Kline, currentPrice float64) bool {
	return m.shouldEnter
}

func (m *MockStrategy) ShouldClosePosition(ctx context.Context, position *domain.Position, klines []*domain.Kline, currentPrice float64) (bool, domain.CloseReason) {
	return m.shouldClose, m.closeReason
}

func (m *MockStrategy) RequiredDataPoints() int {
	return 1
}

func (m *MockStrategy) Name() string {
	return "MockStrategy"
}

func TestOptimizer(t *testing.T) {
	// Create test data
	klines := []*domain.Kline{
		{
			OpenTime:  time.Now().Add(-24 * time.Hour),
			Open:      50000,
			High:      55000,
			Low:       45000,
			Close:     52000,
			Volume:    100,
			CloseTime: time.Now().Add(-23 * time.Hour),
		},
		{
			OpenTime:  time.Now().Add(-12 * time.Hour),
			Open:      52000,
			High:      56000,
			Low:       51000,
			Close:     54000,
			Volume:    150,
			CloseTime: time.Now().Add(-11 * time.Hour),
		},
	}

	// Create optimizer configuration
	config := OptimizerConfig{
		ParameterRanges: []ParameterRange{
			{
				Name:  "param1",
				Min:   1,
				Max:   3,
				Step:  1,
				IsInt: true,
			},
			{
				Name:  "param2",
				Min:   0.1,
				Max:   0.3,
				Step:  0.1,
				IsInt: false,
			},
		},
		InitialFunds:  10000,
		PositionSize:  0.1,
		StopLoss:      0.1,
		TakeProfit:    0.2,
		Symbol:        "BTCUSDT",
		Leverage:      2,
		StartTime:     time.Now().Add(-24 * time.Hour).Unix(),
		EndTime:       time.Now().Unix(),
		ScoreFunction: DefaultScoreFunction,
	}

	// Create optimizer
	optimizer := NewOptimizer(config)

	// Create mock strategy
	strategy := NewMockStrategy(true, true, domain.CloseReasonTakeProfit)

	// Run optimization
	results, err := optimizer.Optimize(context.Background(), strategy, klines)
	if err != nil {
		t.Fatalf("Optimization failed: %v", err)
	}

	// Verify results
	if len(results) == 0 {
		t.Error("Expected non-empty optimization results")
	}

	// Verify parameter combinations
	expectedCombinations := 9 // 3 values for param1 * 3 values for param2
	if len(results) != expectedCombinations {
		t.Errorf("Expected %d parameter combinations, got %d", expectedCombinations, len(results))
	}

	// Verify results are sorted by score
	for i := 1; i < len(results); i++ {
		if results[i-1].Score < results[i].Score {
			t.Error("Results are not sorted by score in descending order")
		}
	}
}

func TestGenerateParameterCombinations(t *testing.T) {
	config := OptimizerConfig{
		ParameterRanges: []ParameterRange{
			{
				Name:  "param1",
				Min:   1,
				Max:   2,
				Step:  1,
				IsInt: true,
			},
			{
				Name:  "param2",
				Min:   0.1,
				Max:   0.2,
				Step:  0.1,
				IsInt: false,
			},
		},
	}

	optimizer := NewOptimizer(config)
	combinations := optimizer.generateParameterCombinations()

	// Verify number of combinations
	expectedCombinations := 4 // 2 values for param1 * 2 values for param2
	if len(combinations) != expectedCombinations {
		t.Errorf("Expected %d parameter combinations, got %d", expectedCombinations, len(combinations))
	}

	// Verify parameter values
	expectedValues := map[string][]float64{
		"param1": {1, 2},
		"param2": {0.1, 0.2},
	}

	for _, combination := range combinations {
		for paramName, expectedValues := range expectedValues {
			value, exists := combination[paramName]
			if !exists {
				t.Errorf("Parameter %s not found in combination", paramName)
			}
			found := false
			for _, expectedValue := range expectedValues {
				if value == expectedValue {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Unexpected value %f for parameter %s", value, paramName)
			}
		}
	}
}

func TestDefaultScoreFunction(t *testing.T) {
	metrics := &analytics.PerformanceMetrics{
		WinRate:            0.6,
		ProfitFactor:       2.0,
		MaxDrawdown:        0.2,
		ReturnOnInvestment: 0.5,
		RiskRewardRatio:    2.0,
	}

	score := DefaultScoreFunction(metrics)

	// Verify score calculation
	expectedScore := 0.6*0.3 + 2.0*0.2 + 0.8*0.2 + 0.5*0.2 + 2.0*0.1
	if score != expectedScore {
		t.Errorf("Expected score %f, got %f", expectedScore, score)
	}
}
