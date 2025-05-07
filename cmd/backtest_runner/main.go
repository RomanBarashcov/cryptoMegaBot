package main

import (
	"context"
	"cryptoMegaBot/config"
	"cryptoMegaBot/internal/adapters/logger"
	"cryptoMegaBot/internal/strategy/backtesting"
	"cryptoMegaBot/internal/strategy/strategies"
	"cryptoMegaBot/internal/utils"
	"fmt"
	"log"
)

func main() {
	// 1. Load Configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("FATAL: Failed to load configuration: %v", err) // Use standard log before logger is ready
	}

	appLogger := logger.NewStdLogger(cfg.LogLevel)

	// 2. Load klines from CSV
	klines, err := utils.ReadKlinesFromCSV("data/ETHUSDT_1m_20250207_to_20250507.csv")
	if err != nil {
		appLogger.Error(context.Background(), err, "Error loading klines")
		log.Fatalf("Error loading klines: %v", err)
	}
	appLogger.Info(context.Background(), "Loaded klines", map[string]interface{}{"count": len(klines)})
	// 3. Set up configs
	tps := []float64{0.005, 0.01, 0.015}
	sl := 0.0025 // or test other values
	leverage := 4
	initialFunds := 1000.0
	positionSize := 1.0

	// 4. Choose strategy (example: MA crossover)
	strategy, _ := strategies.NewMACrossover(
		strategies.MACrossoverConfig{
			ShortTermMAPeriod: 5,
			LongTermMAPeriod:  20,
			EMAPeriod:         10,
			RSIPeriod:         14,
			RSIOverbought:     70,
			RSIOversold:       30,
		},
		appLogger, // logger
	)

	// 5 Run backtests
	for _, tp := range tps {
		config := backtesting.BacktestConfig{
			StartTime:    klines[0].OpenTime,
			EndTime:      klines[len(klines)-1].CloseTime,
			InitialFunds: initialFunds,
			PositionSize: positionSize,
			StopLoss:     sl,
			TakeProfit:   tp,
			Symbol:       "ETHUSDT",
			Leverage:     leverage,
		}
		result, err := backtesting.Backtest(context.Background(), strategy, klines, config)
		if err != nil {
			appLogger.Error(context.Background(), err, "Backtest error")
			continue
		}
		appLogger.Info(context.Background(), "Backtest result", map[string]interface{}{
			"TP":      tp * 100,
			"Trades":  result.TotalTrades,
			"WinRate": result.WinRate * 100,
			"PnL":     result.TotalProfit,
			"Sharpe":  result.SharpeRatio,
		})
		// Write trades to CSV for each TP
		tradesFile := fmt.Sprintf("data/backtest_trades_tp%.1f.csv", tp*100)
		err = utils.WriteTradesToCSV(result.Trades, tradesFile)
		if err != nil {
			appLogger.Error(context.Background(), err, "Error writing trades CSV")
		}
		appLogger.Info(context.Background(), "Trades saved to", map[string]interface{}{"filename": tradesFile})
	}
}
