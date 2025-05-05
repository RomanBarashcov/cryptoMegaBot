package backtesting

import (
	"context"
	"cryptoMegaBot/internal/domain"
	"cryptoMegaBot/internal/strategy/strategies"
	"fmt"
	"time"
)

// BacktestConfig holds configuration for backtesting
type BacktestConfig struct {
	StartTime    time.Time
	EndTime      time.Time
	InitialFunds float64
	PositionSize float64
	StopLoss     float64
	TakeProfit   float64
	Symbol       string
	Leverage     int
}

// BacktestResult holds the results of a backtest
type BacktestResult struct {
	TotalTrades        int
	WinningTrades      int
	LosingTrades       int
	WinRate            float64
	TotalProfit        float64
	MaxDrawdown        float64
	ProfitFactor       float64
	AverageWin         float64
	AverageLoss        float64
	SharpeRatio        float64
	FinalBalance       float64
	ReturnOnInvestment float64
	Trades             []*domain.Trade
}

// Backtest runs a backtest for a given strategy
func Backtest(ctx context.Context, strategy strategies.Strategy, klines []*domain.Kline, config BacktestConfig) (*BacktestResult, error) {
	if len(klines) < strategy.RequiredDataPoints() {
		return nil, fmt.Errorf("not enough data points for strategy")
	}

	result := &BacktestResult{
		FinalBalance: config.InitialFunds,
	}

	var currentPosition *domain.Position
	var peakBalance = config.InitialFunds
	var trades []*domain.Trade

	// Sort klines by time
	// Note: Assuming klines are already sorted by time

	// Iterate through klines
	for i := strategy.RequiredDataPoints(); i < len(klines); i++ {
		currentKline := klines[i]
		historicalKlines := klines[:i+1]

		// Check if we should close an existing position
		if currentPosition != nil {
			shouldClose, reason := strategy.ShouldClosePosition(ctx, currentPosition, historicalKlines, currentKline.Close)
			if shouldClose {
				// Calculate profit/loss
				pnl := calculatePNL(currentPosition, currentKline.Close)
				result.TotalProfit += pnl
				result.FinalBalance += pnl

				// Update trade statistics
				if pnl > 0 {
					result.WinningTrades++
					result.AverageWin = (result.AverageWin*float64(result.WinningTrades-1) + pnl) / float64(result.WinningTrades)
				} else {
					result.LosingTrades++
					result.AverageLoss = (result.AverageLoss*float64(result.LosingTrades-1) + pnl) / float64(result.LosingTrades)
				}

				// Update max drawdown
				if result.FinalBalance > peakBalance {
					peakBalance = result.FinalBalance
				}
				drawdown := (peakBalance - result.FinalBalance) / peakBalance
				if drawdown > result.MaxDrawdown {
					result.MaxDrawdown = drawdown
				}

				// Record trade
				trade := &domain.Trade{
					PositionID:  currentPosition.ID,
					Symbol:      config.Symbol,
					EntryPrice:  currentPosition.EntryPrice,
					ExitPrice:   currentKline.Close,
					Quantity:    currentPosition.Quantity,
					Leverage:    currentPosition.Leverage,
					PNL:         pnl,
					EntryTime:   currentPosition.EntryTime,
					ExitTime:    currentKline.OpenTime,
					CloseReason: reason,
				}
				trades = append(trades, trade)

				currentPosition = nil
			}
		}

		// Check if we should open a new position
		if currentPosition == nil && strategy.ShouldEnterTrade(ctx, historicalKlines, currentKline.Close) {
			currentPosition = &domain.Position{
				Symbol:     config.Symbol,
				EntryPrice: currentKline.Close,
				Quantity:   config.PositionSize,
				Leverage:   config.Leverage,
				StopLoss:   currentKline.Close * (1 - config.StopLoss),
				TakeProfit: currentKline.Close * (1 + config.TakeProfit),
				EntryTime:  currentKline.OpenTime,
				Status:     domain.StatusOpen,
			}
			result.TotalTrades++
		}
	}

	// Calculate final statistics
	result.WinRate = float64(result.WinningTrades) / float64(result.TotalTrades)
	if result.AverageLoss != 0 {
		result.ProfitFactor = result.AverageWin / -result.AverageLoss
	}
	result.ReturnOnInvestment = (result.FinalBalance - config.InitialFunds) / config.InitialFunds

	// Calculate Sharpe Ratio (assuming risk-free rate of 0 for simplicity)
	if len(trades) > 1 {
		var returns []float64
		for i := 1; i < len(trades); i++ {
			returns = append(returns, trades[i].PNL/trades[i-1].PNL-1)
		}
		result.SharpeRatio = calculateSharpeRatio(returns)
	}

	result.Trades = trades

	return result, nil
}

// calculatePNL calculates the profit/loss for a position
func calculatePNL(position *domain.Position, currentPrice float64) float64 {
	return (currentPrice - position.EntryPrice) * position.Quantity * float64(position.Leverage)
}

// calculateSharpeRatio calculates the Sharpe ratio for a series of returns
func calculateSharpeRatio(returns []float64) float64 {
	if len(returns) < 2 {
		return 0
	}

	// Calculate mean return
	var sum float64
	for _, r := range returns {
		sum += r
	}
	mean := sum / float64(len(returns))

	// Calculate standard deviation
	var variance float64
	for _, r := range returns {
		variance += (r - mean) * (r - mean)
	}
	variance /= float64(len(returns) - 1)
	stdDev := variance

	// Calculate Sharpe ratio (assuming risk-free rate of 0)
	if stdDev == 0 {
		return 0
	}
	return mean / stdDev
}
