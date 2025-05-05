package main

import (
	"context"
	"log" // Use standard log only for initial fatal errors before logger is set up

	"cryptoMegaBot/config"
	"cryptoMegaBot/internal/adapters/binanceclient"
	"cryptoMegaBot/internal/adapters/logger"
	"cryptoMegaBot/internal/adapters/sqlite"
	"cryptoMegaBot/internal/app"
	"cryptoMegaBot/internal/strategy"
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

	// 3. Initialize Repository (Database Adapter)
	repo, err := sqlite.NewRepository(sqlite.Config{
		DBPath: cfg.DBPath,
		Logger: appLogger,
	})
	if err != nil {
		appLogger.Error(context.Background(), err, "FATAL: Failed to initialize database repository")
		log.Fatalf("FATAL: Failed to initialize database repository: %v", err) // Also log to stderr
	}
	defer func() {
		if err := repo.Close(); err != nil {
			appLogger.Error(context.Background(), err, "Error closing database repository")
		}
	}()
	appLogger.Info(context.Background(), "Database repository initialized")

	// 4. Initialize Exchange Client (Binance Adapter)
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

	// 5. Initialize Strategy
	strat, err := strategy.New(strategy.Config{
		ShortTermMAPeriod: cfg.StrategyShortMAPeriod,
		LongTermMAPeriod:  cfg.StrategyLongMAPeriod,
		EMAPeriod:         cfg.StrategyEMAPeriod,
		RSIPeriod:         cfg.StrategyRSIPeriod,
		RSIOverbought:     cfg.StrategyRSIOverbought,
		RSIOversold:       cfg.StrategyRSIOversold,
	}, appLogger)
	if err != nil {
		appLogger.Error(context.Background(), err, "FATAL: Failed to initialize trading strategy")
		log.Fatalf("FATAL: Failed to initialize trading strategy: %v", err)
	}
	appLogger.Info(context.Background(), "Trading strategy initialized")

	// 6. Initialize Application Service
	tradingService, err := app.NewTradingService(
		cfg,
		appLogger,
		binanceClient, // Pass the concrete implementation, service expects the interface
		repo,          // Pass the concrete implementation, service expects the interface
		repo,          // Pass the concrete implementation, service expects the interface
		strat,
	)
	if err != nil {
		appLogger.Error(context.Background(), err, "FATAL: Failed to initialize trading service")
		log.Fatalf("FATAL: Failed to initialize trading service: %v", err)
	}
	appLogger.Info(context.Background(), "Trading service initialized")

	// 7. Start the Service
	// Use context.Background() as the base context for the application run
	if err := tradingService.Start(context.Background()); err != nil {
		appLogger.Error(context.Background(), err, "Trading service exited with error")
		log.Fatalf("FATAL: Trading service exited with error: %v", err)
	}

	appLogger.Info(context.Background(), "Application finished gracefully.")
}
