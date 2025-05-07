package main

import (
	"context"
	"cryptoMegaBot/config"
	"cryptoMegaBot/internal/adapters/binanceclient"
	"cryptoMegaBot/internal/adapters/logger"
	"cryptoMegaBot/internal/utils"
	"fmt"
	"log"
	"time"
)

func main() {
	// 1. Load Configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("FATAL: Failed to load configuration: %v", err) // Use standard log before logger is ready
	}

	// 2. Initialize Logger
	appLogger := logger.NewStdLogger(cfg.LogLevel)
	appLogger.Info(context.Background(), "Logger initialized", map[string]interface{}{"level": cfg.LogLevel.String()})

	// 3. Initialize Exchange Client (Binance Adapter)
	binanceClient, err := binanceclient.New(binanceclient.Config{
		APIKey:               cfg.APIKey,
		SecretKey:            cfg.SecretKey,
		UseTestnet:           cfg.IsTestnet,
		Logger:               appLogger,
		ReconnectDelay:       cfg.ReconnectDelay,
		MaxReconnectAttempts: cfg.MaxReconnectAttempts,
	})
	if err != nil {
		appLogger.Error(context.Background(), err, "FATAL: Failed to initialize Binance client")
		log.Fatalf("FATAL: Failed to initialize Binance client: %v", err)
	}
	appLogger.Info(context.Background(), "Binance client initialized")

	symbol := "ETHUSDT"
	interval := "1m"
	end := time.Now()
	start := end.AddDate(0, -3, 0) // 3 months ago

	fmt.Printf("Fetching klines for %s %s from %s to %s...\n", symbol, interval, start, end)
	klines, err := binanceClient.GetKlinesRange(context.Background(), symbol, interval, start, end)
	if err != nil {
		appLogger.Error(context.Background(), err, "Error fetching klines")
		log.Fatalf("Error fetching klines: %v", err)
	}
	appLogger.Info(context.Background(), "Fetched klines", map[string]interface{}{"count": len(klines)})

	filename := fmt.Sprintf("data/%s_%s_%s_to_%s.csv", symbol, interval, start.Format("20060102"), end.Format("20060102"))
	err = utils.WriteKlinesToCSV(klines, filename)
	if err != nil {
		appLogger.Error(context.Background(), err, "Error writing CSV")
		log.Fatalf("Error writing CSV: %v", err)
	}
	appLogger.Info(context.Background(), "Saved to", map[string]interface{}{"filename": filename})
}
