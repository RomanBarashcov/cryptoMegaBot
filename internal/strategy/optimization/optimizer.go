package optimization

import (
	"context"
	"cryptoMegaBot/internal/domain"
	"cryptoMegaBot/internal/ports"
	"cryptoMegaBot/internal/strategy/analytics"
	"cryptoMegaBot/internal/strategy/backtesting"
	"cryptoMegaBot/internal/strategy/strategies"
	"math"
	"sync"
)

// ParameterRange defines a range for a parameter to optimize
type ParameterRange struct {
	Name  string
	Min   float64
	Max   float64
	Step  float64
	IsInt bool
}

// OptimizationResult holds the results of a parameter optimization
type OptimizationResult struct {
	Parameters map[string]float64
	Metrics    *analytics.PerformanceMetrics
	Score      float64
}

// OptimizerConfig holds configuration for the optimizer
type OptimizerConfig struct {
	ParameterRanges []ParameterRange
	InitialFunds    float64
	PositionSize    float64
	StopLoss        float64
	TakeProfit      float64
	Symbol          string
	Leverage        int
	StartTime       int64
	EndTime         int64
	ScoreFunction   func(*analytics.PerformanceMetrics) float64
}

// Optimizer implements strategy parameter optimization
type Optimizer struct {
	config OptimizerConfig
}

// NewOptimizer creates a new optimizer instance
func NewOptimizer(config OptimizerConfig) *Optimizer {
	return &Optimizer{
		config: config,
	}
}

// Optimize performs parameter optimization for a strategy
func (o *Optimizer) Optimize(ctx context.Context, strategy strategies.Strategy, klines []*domain.Kline) ([]OptimizationResult, error) {
	// Generate parameter combinations
	combinations := o.generateParameterCombinations()
	results := make([]OptimizationResult, 0, len(combinations))

	// Create a channel to receive results
	resultChan := make(chan OptimizationResult, len(combinations))
	var wg sync.WaitGroup

	// Limit concurrency to avoid system overload
	// Use number of CPU cores or a reasonable maximum
	maxConcurrency := 4 // Adjust based on available CPU cores
	semaphore := make(chan struct{}, maxConcurrency)

	// Process each parameter combination
	for _, params := range combinations {
		wg.Add(1)
		go func(params map[string]float64) {
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() {
				// Release semaphore
				<-semaphore
				wg.Done()
			}()

			// Create strategy instance with current parameters
			strategyInstance, err := o.createStrategyWithParams(strategy, params)
			if err != nil {
				return
			}

			// Run backtest with a subset of data for faster optimization
			// Use every 5th kline to speed up testing while maintaining pattern recognition
			sampledKlines := sampleKlines(klines, 5)

			backtestConfig := backtesting.BacktestConfig{
				StartTime:    sampledKlines[0].OpenTime,
				EndTime:      sampledKlines[len(sampledKlines)-1].CloseTime,
				InitialFunds: o.config.InitialFunds,
				PositionSize: o.config.PositionSize,
				StopLoss:     o.config.StopLoss,
				TakeProfit:   o.config.TakeProfit,
				Symbol:       o.config.Symbol,
				Leverage:     o.config.Leverage,
			}

			result, err := backtesting.Backtest(ctx, strategyInstance, sampledKlines, backtestConfig)
			if err != nil {
				return
			}

			// Calculate performance metrics
			metrics := analytics.AnalyzePerformance(result.Trades, o.config.InitialFunds)

			// Calculate score
			score := o.config.ScoreFunction(metrics)

			// Send result
			resultChan <- OptimizationResult{
				Parameters: params,
				Metrics:    metrics,
				Score:      score,
			}
		}(params)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for result := range resultChan {
		results = append(results, result)
	}

	// Sort results by score
	sortResultsByScore(results)

	return results, nil
}

// sampleKlines returns a subset of klines by taking every nth kline
func sampleKlines(klines []*domain.Kline, n int) []*domain.Kline {
	if n <= 1 {
		return klines
	}

	result := make([]*domain.Kline, 0, len(klines)/n+1)
	for i := 0; i < len(klines); i += n {
		result = append(result, klines[i])
	}
	return result
}

// generateParameterCombinations generates all possible parameter combinations
func (o *Optimizer) generateParameterCombinations() []map[string]float64 {
	var combinations []map[string]float64
	var currentCombination map[string]float64

	var generate func(int)
	generate = func(paramIndex int) {
		if paramIndex == len(o.config.ParameterRanges) {
			// Create a copy of the current combination
			combination := make(map[string]float64)
			for k, v := range currentCombination {
				combination[k] = v
			}
			combinations = append(combinations, combination)
			return
		}

		param := o.config.ParameterRanges[paramIndex]
		value := param.Min
		for value <= param.Max+param.Step/2 { // Add small epsilon to handle floating point comparison
			if param.IsInt {
				value = math.Round(value)
			}
			currentCombination[param.Name] = value
			generate(paramIndex + 1)
			value += param.Step
		}
	}

	currentCombination = make(map[string]float64)
	generate(0)
	return combinations
}

// createStrategyWithParams creates a strategy instance with the given parameters
func (o *Optimizer) createStrategyWithParams(strategy strategies.Strategy, params map[string]float64) (strategies.Strategy, error) {
	// Get the strategy name to determine which type it is
	strategyName := strategy.Name()

	// Get the logger from the original strategy
	var logger ports.Logger
	if macStrategy, ok := strategy.(*strategies.MACrossover); ok {
		logger = macStrategy.GetLogger()
	} else {
		// If we can't get the logger, return the original strategy
		return strategy, nil
	}

	// Check if it's the Improved MA Crossover strategy
	if strategyName == "Improved Moving Average Crossover" {
		// Create a new MACrossover strategy with the optimized parameters
		config := strategies.MACrossoverConfig{
			FastMAPeriod:  int(params["FastMAPeriod"]),
			SlowMAPeriod:  int(params["SlowMAPeriod"]),
			SignalPeriod:  int(params["SignalPeriod"]),
			ATRPeriod:     int(params["ATRPeriod"]),
			ATRMultiplier: params["ATRMultiplier"],
		}

		// Add optional multi-timeframe parameters if they exist
		if _, ok := params["UseMultiTimeframe"]; ok {
			config.UseMultiTimeframe = params["UseMultiTimeframe"] > 0.5 // Convert to boolean
		}

		// Add optional scalping parameters if they exist
		if _, ok := params["UseScalpTimeframe"]; ok {
			config.UseScalpTimeframe = params["UseScalpTimeframe"] > 0.5 // Convert to boolean
		}
		if _, ok := params["ScalpFastPeriod"]; ok {
			config.ScalpFastPeriod = int(params["ScalpFastPeriod"])
		}
		if _, ok := params["ScalpSlowPeriod"]; ok {
			config.ScalpSlowPeriod = int(params["ScalpSlowPeriod"])
		}

		// Add optional day trading parameters if they exist
		if _, ok := params["MaxDailyLosses"]; ok {
			config.MaxDailyLosses = int(params["MaxDailyLosses"])
		}
		if _, ok := params["MaxConsecutiveLosses"]; ok {
			config.MaxConsecutiveLosses = int(params["MaxConsecutiveLosses"])
		}
		if _, ok := params["PartialProfitPct"]; ok {
			config.PartialProfitPct = params["PartialProfitPct"]
		}
		if _, ok := params["TrailingActivePct"]; ok {
			config.TrailingActivePct = params["TrailingActivePct"]
		}
		if _, ok := params["BreakEvenActivation"]; ok {
			config.BreakEvenActivation = params["BreakEvenActivation"]
		}
		if _, ok := params["TrailingStopTightening"]; ok {
			config.TrailingStopTightening = params["TrailingStopTightening"] > 0.5 // Convert to boolean
		}
		if _, ok := params["InitialRiskPerTrade"]; ok {
			config.InitialRiskPerTrade = params["InitialRiskPerTrade"]
		}
		if _, ok := params["DynamicLeverageAdjustment"]; ok {
			config.DynamicLeverageAdjustment = params["DynamicLeverageAdjustment"] > 0.5 // Convert to boolean
		}
		if _, ok := params["MaxLeverageUsed"]; ok {
			config.MaxLeverageUsed = params["MaxLeverageUsed"]
		}

		// Create a new strategy instance with the optimized parameters
		newStrategy, err := strategies.NewImprovedMACrossover(config, logger)
		if err != nil {
			return strategy, err
		}
		return newStrategy, nil
	}

	// If it's not a recognized strategy type, return the original
	return strategy, nil
}

// sortResultsByScore sorts optimization results by score in descending order
func sortResultsByScore(results []OptimizationResult) {
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Score < results[j].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

// DefaultScoreFunction provides a default scoring function for optimization
func DefaultScoreFunction(metrics *analytics.PerformanceMetrics) float64 {
	// This is a simple scoring function that can be customized
	// It combines several metrics into a single score
	score := 0.0

	// Weight different metrics
	score += metrics.WinRate * 0.3
	score += metrics.ProfitFactor * 0.2
	score += (1 - metrics.MaxDrawdown) * 0.2
	score += metrics.ReturnOnInvestment * 0.2
	score += metrics.RiskRewardRatio * 0.1

	return score
}
