package main

import (
	"context"
	"cryptoMegaBot/config"
	"cryptoMegaBot/internal/adapters/logger"
	"cryptoMegaBot/internal/domain"
	"cryptoMegaBot/internal/strategy/backtesting"
	"cryptoMegaBot/internal/strategy/strategies"
	"cryptoMegaBot/internal/utils"
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// KlineWithTimeframe wraps a domain.Kline with its timeframe information
type KlineWithTimeframe struct {
	Kline     *domain.Kline
	Timeframe string
}

func main() {
	// 1. Load Configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("FATAL: Failed to load configuration: %v", err)
	}

	appLogger := logger.NewStdLogger(cfg.LogLevel)

	// 2. Load klines from CSV for multiple timeframes
	timeframes := []string{"5m", "15m", "1h", "4h", "1d"}
	klinesMap := make(map[string][]*KlineWithTimeframe)

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, tf := range timeframes {
		wg.Add(1)
		go func(timeframe string) {
			defer wg.Done()

			filename := fmt.Sprintf("data/ETHUSDT_%s_20250207_to_20250507.csv", timeframe)
			klines, err := utils.ReadKlinesFromCSV(filename)
			if err != nil {
				appLogger.Error(context.Background(), err, "Error loading klines",
					map[string]interface{}{"timeframe": timeframe})
				return
			}

			// Add timeframe information to each kline
			klinesWithTF := make([]*KlineWithTimeframe, len(klines))
			for i, k := range klines {
				klinesWithTF[i] = &KlineWithTimeframe{
					Kline:     k,
					Timeframe: timeframe,
				}
			}

			mu.Lock()
			klinesMap[timeframe] = klinesWithTF
			mu.Unlock()

			appLogger.Info(context.Background(), "Loaded klines",
				map[string]interface{}{
					"timeframe": timeframe,
					"count":     len(klines),
				})
		}(tf)
	}

	wg.Wait()

	// Check if we have all timeframes
	for _, tf := range timeframes {
		if _, ok := klinesMap[tf]; !ok {
			appLogger.Error(context.Background(), fmt.Errorf("missing timeframe data"),
				"Missing klines for timeframe",
				map[string]interface{}{"timeframe": tf})
			log.Fatalf("Missing klines for timeframe: %s", tf)
		}
	}

	// Use 1h timeframe as the base for backtesting
	baseTimeframe := "1h"
	klines := make([]*domain.Kline, len(klinesMap[baseTimeframe]))
	for i, k := range klinesMap[baseTimeframe] {
		klines[i] = k.Kline
	}

	appLogger.Info(context.Background(), "Using base timeframe for backtesting",
		map[string]interface{}{
			"baseTimeframe": baseTimeframe,
			"count":         len(klines),
		})

	// 3. Set up configs with improved parameters
	tps := []float64{0.015, 0.02, 0.03} // 1.5%, 2.0%, 3.0% take profits

	// Wider stop loss based on ATR multiplier
	atrMultiplier := 2.5 // Use 2.5x ATR for stop loss (wider than before)

	// Default stop loss as fallback (will be overridden by dynamic ATR-based stop)
	sl := 0.01 // 1.0% stop loss - wider than before

	leverage := 3 // Reduced from 4x to 3x for more conservative approach
	initialFunds := 1000.0

	// 4. Create improved strategy with optimized parameters for day trading
	strategyConfig := strategies.MACrossoverConfig{
		// Core parameters
		FastMAPeriod:  8,             // Fast EMA period
		SlowMAPeriod:  21,            // Slow EMA period
		SignalPeriod:  9,             // Signal line period
		ATRPeriod:     14,            // ATR period
		ATRMultiplier: atrMultiplier, // ATR multiplier for stop loss

		// Multi-timeframe parameters - adjusted for day trading
		UseMultiTimeframe: true,  // Enable multi-timeframe analysis
		PrimaryTimeframe:  "15m", // Primary timeframe for trading decisions (changed from 1h)
		TrendTimeframe:    "1h",  // Higher timeframe for trend confirmation (changed from 4h)

		// Scalping parameters for more frequent trading
		UseScalpTimeframe: true, // Enable scalping timeframe
		ScalpTimeframe:    "5m", // 5-minute timeframe for scalping
		ScalpFastPeriod:   5,    // Fast MA period for scalping
		ScalpSlowPeriod:   13,   // Slow MA period for scalping

		// Day trading parameters - optimized for more frequent trading
		MaxDailyLosses:         2,             // Maximum number of losing trades per day
		MaxConsecutiveLosses:   2,             // Maximum consecutive losses before reducing size
		MaxHoldingTime:         2 * time.Hour, // Reduced maximum time to hold a position (from 4h to 2h)
		PartialProfitPct:       0.005,         // Take partial profits at 0.5% (reduced from 1%)
		TrailingActivePct:      0.002,         // Activate trailing stop at 0.2% (reduced from 0.3%)
		BreakEvenActivation:    0.002,         // Move to breakeven at 0.2% profit
		TrailingStopTightening: true,          // Enable progressive tightening of trailing stop

		// Risk management parameters
		InitialRiskPerTrade:       0.005, // 0.5% risk per trade
		DynamicLeverageAdjustment: true,  // Enable dynamic leverage adjustment

		// Market hours parameters
		TradingHoursOnly: false, // Not limiting to specific hours for backtesting
		MaxLeverageUsed:  4.0,   // Maximum leverage to use
	}

	strategy, err := strategies.NewImprovedMACrossover(strategyConfig, appLogger)
	if err != nil {
		appLogger.Error(context.Background(), err, "Failed to create strategy")
		log.Fatalf("Failed to create strategy: %v", err)
	}

	// 5. Run backtests for each take profit level
	for _, tp := range tps {
		config := backtesting.BacktestConfig{
			StartTime:    klines[0].OpenTime,
			EndTime:      klines[len(klines)-1].CloseTime,
			InitialFunds: initialFunds,
			PositionSize: 0.0, // Will be dynamically calculated based on volatility
			StopLoss:     sl,
			TakeProfit:   tp,
			Symbol:       "ETHUSDT",
			Leverage:     leverage,
		}

		// Use 15m timeframe as the base for day trading backtests
		baseTimeframe := "15m"
		klines = make([]*domain.Kline, len(klinesMap[baseTimeframe]))
		for i, k := range klinesMap[baseTimeframe] {
			klines[i] = k.Kline
		}

		appLogger.Info(context.Background(), "Using base timeframe for backtesting",
			map[string]interface{}{
				"baseTimeframe": baseTimeframe,
				"count":         len(klines),
			})

		// Modify the backtest to use dynamic position sizing
		result, err := runBacktestWithDynamicPositionSizing(
			context.Background(),
			strategy,
			klines,
			config,
			appLogger,
			atrMultiplier,
		)

		if err != nil {
			appLogger.Error(context.Background(), err, "Backtest error")
			continue
		}

		appLogger.Info(context.Background(), "Backtest result", map[string]interface{}{
			"Strategy": "MACrossover",
			"TP":       tp * 100,
			"Trades":   result.TotalTrades,
			"WinRate":  result.WinRate * 100,
			"PnL":      result.TotalProfit,
			"Sharpe":   result.SharpeRatio,
			"MaxDD":    result.MaxDrawdown,
			"AvgWin":   result.AverageWin,
			"AvgLoss":  result.AverageLoss,
		})

		// Write trades to CSV
		tradesFile := fmt.Sprintf("data/improved_backtest_trades_tp%.1f.csv", tp*100)
		err = utils.WriteTradesToCSV(result.Trades, tradesFile)
		if err != nil {
			appLogger.Error(context.Background(), err, "Error writing trades CSV")
		}
		appLogger.Info(context.Background(), "Trades saved to", map[string]interface{}{"filename": tradesFile})
	}
}

// runBacktestWithDynamicPositionSizing runs a backtest with dynamic position sizing based on volatility
func runBacktestWithDynamicPositionSizing(
	ctx context.Context,
	strategy *strategies.MACrossover,
	klines []*domain.Kline,
	config backtesting.BacktestConfig,
	logger *logger.StdLogger,
	atrMultiplier float64,
) (*backtesting.BacktestResult, error) {
	if len(klines) < strategy.RequiredDataPoints() {
		return nil, fmt.Errorf("not enough data points for strategy")
	}

	result := &backtesting.BacktestResult{
		FinalBalance: config.InitialFunds,
	}

	var currentPosition *domain.Position
	var peakBalance = config.InitialFunds
	var trades []*domain.Trade

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
			// Calculate dynamic position size based on volatility
			positionSize := strategy.GetPositionSize(ctx, historicalKlines, config.InitialFunds)

			// Calculate dynamic stop loss based on ATR
			atr, err := strategy.GetATR(ctx, historicalKlines)
			if err != nil {
				logger.Error(ctx, err, "Failed to calculate ATR for stop loss")
				continue
			}

			// Use ATR-based stop loss or default stop loss, whichever is wider
			atrStopLoss := currentKline.Close * (1 - (atr * atrMultiplier / currentKline.Close))
			defaultStopLoss := currentKline.Close * (1 - config.StopLoss)
			stopLoss := math.Min(atrStopLoss, defaultStopLoss)

			currentPosition = &domain.Position{
				Symbol:               config.Symbol,
				EntryPrice:           currentKline.Close,
				Quantity:             positionSize,
				Leverage:             config.Leverage,
				StopLoss:             stopLoss,
				TakeProfit:           currentKline.Close * (1 + config.TakeProfit),
				EntryTime:            currentKline.OpenTime,
				Status:               domain.StatusOpen,
				TrailingStopPrice:    0, // Will be initialized when profit reaches threshold
				TrailingStopDistance: 0, // Will be set when trailing stop is activated
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

// calculatePNL calculates the profit/loss for a position including trading fees
func calculatePNL(position *domain.Position, currentPrice float64) float64 {
	// Trading fee (0.1% for maker/taker on Binance futures)
	const tradingFee = 0.001

	// Calculate raw PNL
	rawPnl := (currentPrice - position.EntryPrice) * position.Quantity * float64(position.Leverage)

	// Calculate fees (entry and exit)
	entryFee := position.EntryPrice * position.Quantity * tradingFee
	exitFee := currentPrice * position.Quantity * tradingFee
	totalFees := (entryFee + exitFee) * float64(position.Leverage)

	// Net PNL after fees
	return rawPnl - totalFees
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
	stdDev := math.Sqrt(variance)

	// Calculate Sharpe ratio (assuming risk-free rate of 0)
	if stdDev == 0 {
		return 0
	}
	return mean / stdDev
}
