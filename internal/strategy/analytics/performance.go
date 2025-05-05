package analytics

import (
	"cryptoMegaBot/internal/domain"
	"math"
	"sort"
	"time"
)

// PerformanceMetrics holds comprehensive performance metrics for a strategy
type PerformanceMetrics struct {
	// Basic Metrics
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

	// Advanced Metrics
	MaxConsecutiveWins   int
	MaxConsecutiveLosses int
	AverageTradeDuration time.Duration
	ProfitToMaxDrawdown  float64
	RecoveryFactor       float64
	Expectancy           float64
	RiskRewardRatio      float64
	MonthlyReturns       map[string]float64
	Drawdowns            []Drawdown
	EquityCurve          []EquityPoint
}

// Drawdown represents a drawdown period
type Drawdown struct {
	StartTime  time.Time
	EndTime    time.Time
	StartValue float64
	EndValue   float64
	Depth      float64
	Duration   time.Duration
}

// EquityPoint represents a point on the equity curve
type EquityPoint struct {
	Time     time.Time
	Value    float64
	Drawdown float64
}

// AnalyzePerformance calculates comprehensive performance metrics from trades
func AnalyzePerformance(trades []*domain.Trade, initialBalance float64) *PerformanceMetrics {
	metrics := &PerformanceMetrics{
		FinalBalance:   initialBalance,
		MonthlyReturns: make(map[string]float64),
		Drawdowns:      make([]Drawdown, 0),
		EquityCurve:    make([]EquityPoint, 0),
	}

	if len(trades) == 0 {
		return metrics
	}

	// Sort trades by entry time
	sort.Slice(trades, func(i, j int) bool {
		return trades[i].EntryTime.Before(trades[j].EntryTime)
	})

	var currentBalance = initialBalance
	var peakBalance = initialBalance
	var currentDrawdown *Drawdown
	var consecutiveWins, consecutiveLosses int
	var maxConsecutiveWins, maxConsecutiveLosses int

	// Process each trade
	for _, trade := range trades {
		// Update basic metrics
		metrics.TotalTrades++
		if trade.PNL > 0 {
			metrics.WinningTrades++
			consecutiveWins++
			consecutiveLosses = 0
			metrics.AverageWin = (metrics.AverageWin*float64(metrics.WinningTrades-1) + trade.PNL) / float64(metrics.WinningTrades)
		} else {
			metrics.LosingTrades++
			consecutiveLosses++
			consecutiveWins = 0
			metrics.AverageLoss = (metrics.AverageLoss*float64(metrics.LosingTrades-1) + trade.PNL) / float64(metrics.LosingTrades)
		}

		// Update consecutive wins/losses
		if consecutiveWins > maxConsecutiveWins {
			maxConsecutiveWins = consecutiveWins
		}
		if consecutiveLosses > maxConsecutiveLosses {
			maxConsecutiveLosses = consecutiveLosses
		}

		// Update balance and equity curve
		currentBalance += trade.PNL
		metrics.TotalProfit += trade.PNL
		metrics.FinalBalance = currentBalance

		// Update monthly returns
		monthKey := trade.ExitTime.Format("2006-01")
		metrics.MonthlyReturns[monthKey] += trade.PNL

		// Update drawdown tracking
		if currentBalance > peakBalance {
			peakBalance = currentBalance
			if currentDrawdown != nil {
				currentDrawdown.EndTime = trade.ExitTime
				currentDrawdown.EndValue = currentBalance
				currentDrawdown.Duration = currentDrawdown.EndTime.Sub(currentDrawdown.StartTime)
				metrics.Drawdowns = append(metrics.Drawdowns, *currentDrawdown)
				currentDrawdown = nil
			}
		} else {
			drawdown := (peakBalance - currentBalance) / peakBalance
			if currentDrawdown == nil {
				currentDrawdown = &Drawdown{
					StartTime:  trade.ExitTime,
					StartValue: peakBalance,
					Depth:      drawdown,
				}
			} else {
				currentDrawdown.Depth = math.Max(currentDrawdown.Depth, drawdown)
			}
			if drawdown > metrics.MaxDrawdown {
				metrics.MaxDrawdown = drawdown
			}
		}

		// Add equity curve point
		metrics.EquityCurve = append(metrics.EquityCurve, EquityPoint{
			Time:     trade.ExitTime,
			Value:    currentBalance,
			Drawdown: (peakBalance - currentBalance) / peakBalance,
		})
	}

	// Close any open drawdown
	if currentDrawdown != nil {
		currentDrawdown.EndTime = trades[len(trades)-1].ExitTime
		currentDrawdown.EndValue = currentBalance
		currentDrawdown.Duration = currentDrawdown.EndTime.Sub(currentDrawdown.StartTime)
		metrics.Drawdowns = append(metrics.Drawdowns, *currentDrawdown)
	}

	// Calculate final metrics
	if metrics.TotalTrades > 0 {
		metrics.WinRate = float64(metrics.WinningTrades) / float64(metrics.TotalTrades)
		if metrics.AverageLoss != 0 {
			metrics.ProfitFactor = metrics.AverageWin / -metrics.AverageLoss
		}
		metrics.ReturnOnInvestment = (metrics.FinalBalance - initialBalance) / initialBalance
		metrics.MaxConsecutiveWins = maxConsecutiveWins
		metrics.MaxConsecutiveLosses = maxConsecutiveLosses

		// Calculate average trade duration
		var totalDuration time.Duration
		for _, trade := range trades {
			totalDuration += trade.ExitTime.Sub(trade.EntryTime)
		}
		metrics.AverageTradeDuration = totalDuration / time.Duration(len(trades))

		// Calculate profit to max drawdown ratio
		if metrics.MaxDrawdown > 0 {
			metrics.ProfitToMaxDrawdown = metrics.TotalProfit / (initialBalance * metrics.MaxDrawdown)
		}

		// Calculate recovery factor
		if metrics.MaxDrawdown > 0 {
			metrics.RecoveryFactor = metrics.TotalProfit / (initialBalance * metrics.MaxDrawdown)
		}

		// Calculate expectancy
		metrics.Expectancy = (metrics.WinRate * metrics.AverageWin) + ((1 - metrics.WinRate) * metrics.AverageLoss)

		// Calculate risk-reward ratio
		if metrics.AverageLoss != 0 {
			metrics.RiskRewardRatio = metrics.AverageWin / -metrics.AverageLoss
		}
	}

	return metrics
}

// GetMonthlyReturns returns the monthly returns as a sorted slice
func (m *PerformanceMetrics) GetMonthlyReturns() []MonthlyReturn {
	returns := make([]MonthlyReturn, 0, len(m.MonthlyReturns))
	for month, profit := range m.MonthlyReturns {
		date, _ := time.Parse("2006-01", month)
		returns = append(returns, MonthlyReturn{
			Month:  date,
			Return: profit,
		})
	}
	sort.Slice(returns, func(i, j int) bool {
		return returns[i].Month.Before(returns[j].Month)
	})
	return returns
}

// MonthlyReturn represents a monthly return value
type MonthlyReturn struct {
	Month  time.Time
	Return float64
}
