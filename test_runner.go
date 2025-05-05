package main

import (
	"context"
	"flag"
	"log"
	"os"
	"testing"
	"time"

	"cryptoMegaBot/config"
	"cryptoMegaBot/internal/adapters/binanceclient"
	"cryptoMegaBot/internal/adapters/logger"
	"cryptoMegaBot/internal/adapters/sqlite"
	"cryptoMegaBot/internal/strategy"
)

var (
	// Test configuration flags
	testConfigPath = flag.String("config", "config.yaml", "Path to test configuration file")
	runIntegration = flag.Bool("integration", false, "Run integration tests")
	runUnit        = flag.Bool("unit", true, "Run unit tests")
	verbose        = flag.Bool("v", false, "Verbose test output")
)

// setupTestEnvironment initializes the test environment with necessary dependencies
func setupTestEnvironment(t *testing.T) (*config.Config, *logger.StdLogger, *sqlite.Repository, *binanceclient.Client, *strategy.Strategy) {
	// Load test configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load test configuration: %v", err)
	}

	// Initialize logger
	appLogger := logger.NewStdLogger(cfg.LogLevel)
	appLogger.Info(context.Background(), "Test logger initialized", map[string]interface{}{"level": cfg.LogLevel.String()})

	// Initialize test database
	repo, err := sqlite.NewRepository(sqlite.Config{
		DBPath: ":memory:", // Use in-memory database for tests
		Logger: appLogger,
	})
	if err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}

	// Initialize mock exchange client for unit tests
	binanceClient, err := binanceclient.New(binanceclient.Config{
		APIKey:               cfg.APIKey,
		SecretKey:            cfg.SecretKey,
		UseTestnet:           true, // Always use testnet for tests
		Logger:               appLogger,
		ReconnectDelay:       cfg.ReconnectDelay,
		MaxReconnectAttempts: cfg.MaxReconnectAttempts,
	})
	if err != nil {
		t.Fatalf("Failed to initialize test exchange client: %v", err)
	}

	// Initialize strategy
	strat, err := strategy.New(strategy.Config{
		ShortTermMAPeriod: cfg.StrategyShortMAPeriod,
		LongTermMAPeriod:  cfg.StrategyLongMAPeriod,
		EMAPeriod:         cfg.StrategyEMAPeriod,
		RSIPeriod:         cfg.StrategyRSIPeriod,
		RSIOverbought:     cfg.StrategyRSIOverbought,
		RSIOversold:       cfg.StrategyRSIOversold,
	}, appLogger)
	if err != nil {
		t.Fatalf("Failed to initialize test strategy: %v", err)
	}

	return cfg, appLogger, repo, binanceClient, strat
}

// cleanupTestEnvironment performs cleanup after tests
func cleanupTestEnvironment(t *testing.T, repo *sqlite.Repository) {
	if err := repo.Close(); err != nil {
		t.Errorf("Error closing test database: %v", err)
	}
}

func TestMain(m *testing.M) {
	// Parse command line flags
	flag.Parse()

	// Set test timeout based on test mode
	if testing.Short() {
		// For short tests, use a shorter timeout
		timeout := 30 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		// Create a channel to receive the test result
		done := make(chan int)
		go func() {
			done <- m.Run()
		}()

		// Wait for either the tests to complete or the timeout
		select {
		case <-ctx.Done():
			log.Fatal("Test timeout exceeded")
		case code := <-done:
			os.Exit(code)
		}
	} else {
		// For full test suite, use a longer timeout
		timeout := 5 * time.Minute
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		// Create a channel to receive the test result
		done := make(chan int)
		go func() {
			done <- m.Run()
		}()

		// Wait for either the tests to complete or the timeout
		select {
		case <-ctx.Done():
			log.Fatal("Test timeout exceeded")
		case code := <-done:
			os.Exit(code)
		}
	}
}

// Example test function showing how to use the test environment
func ExampleTestEnvironment(t *testing.T) {
	// Setup test environment
	cfg, logger, repo, exchange, strat := setupTestEnvironment(t)
	defer cleanupTestEnvironment(t, repo)

	// Your test code here
	t.Logf("Running test with config: %+v", cfg)
	t.Logf("Logger initialized: %+v", logger)
	t.Logf("Repository initialized: %+v", repo)
	t.Logf("Exchange client initialized: %+v", exchange)
	t.Logf("Strategy initialized: %+v", strat)
}
