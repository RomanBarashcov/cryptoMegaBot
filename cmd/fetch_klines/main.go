package main

import (
	"context"
	"cryptoMegaBot/config"
	"cryptoMegaBot/internal/adapters/binanceclient"
	"cryptoMegaBot/internal/adapters/logger"
	"cryptoMegaBot/internal/domain"
	"cryptoMegaBot/internal/ports"
	"cryptoMegaBot/internal/utils"
	"fmt"
	"log"
	"sync"
	"time"
)

// fetchTimeframe fetches klines for a specific symbol and interval
func fetchTimeframe(
	ctx context.Context,
	client *binanceclient.Client,
	symbol string,
	interval string,
	start time.Time,
	end time.Time,
	logger ports.Logger,
	wg *sync.WaitGroup,
	results chan<- *fetchResult,
	errors chan<- error,
) {
	defer wg.Done()

	logger.Info(ctx, "Fetching klines", map[string]interface{}{
		"symbol":   symbol,
		"interval": interval,
		"start":    start.Format("2006-01-02"),
		"end":      end.Format("2006-01-02"),
	})

	klines, err := client.GetKlinesRange(ctx, symbol, interval, start, end)
	if err != nil {
		logger.Error(ctx, err, "Error fetching klines", map[string]interface{}{
			"symbol":   symbol,
			"interval": interval,
		})
		errors <- fmt.Errorf("error fetching %s %s: %w", symbol, interval, err)
		return
	}

	logger.Info(ctx, "Fetched klines successfully", map[string]interface{}{
		"symbol":   symbol,
		"interval": interval,
		"count":    len(klines),
	})

	results <- &fetchResult{
		symbol:   symbol,
		interval: interval,
		klines:   klines,
	}
}

type fetchResult struct {
	symbol   string
	interval string
	klines   []*domain.Kline
}

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

	// 4. Define parameters
	symbol := "ETHUSDT"
	intervals := []string{"5m", "15m", "1h", "4h", "1d"}
	end := time.Now()
	start := end.AddDate(0, -3, 0) // 3 months ago

	// 5. Create channels for results and errors
	resultsChan := make(chan *fetchResult, len(intervals))
	errorsChan := make(chan error, len(intervals))

	// 6. Use WaitGroup to wait for all goroutines
	var wg sync.WaitGroup
	ctx := context.Background()

	// 7. Start goroutines for each interval
	for _, interval := range intervals {
		wg.Add(1)
		go fetchTimeframe(ctx, binanceClient, symbol, interval, start, end, appLogger, &wg, resultsChan, errorsChan)
	}

	// 8. Start a goroutine to close channels when all fetches are done
	go func() {
		wg.Wait()
		close(resultsChan)
		close(errorsChan)
	}()

	// 9. Collect errors
	var fetchErrors []error
	go func() {
		for err := range errorsChan {
			fetchErrors = append(fetchErrors, err)
		}
	}()

	// 10. Process results as they come in
	for result := range resultsChan {
		filename := fmt.Sprintf("data/%s_%s_%s_to_%s.csv",
			result.symbol,
			result.interval,
			start.Format("20060102"),
			end.Format("20060102"),
		)

		err := utils.WriteKlinesToCSV(result.klines, filename)
		if err != nil {
			appLogger.Error(ctx, err, "Error writing CSV", map[string]interface{}{
				"filename": filename,
			})
			continue
		}

		appLogger.Info(ctx, "Saved klines to CSV", map[string]interface{}{
			"symbol":   result.symbol,
			"interval": result.interval,
			"count":    len(result.klines),
			"filename": filename,
		})
	}

	// 11. Check if there were any errors
	if len(fetchErrors) > 0 {
		appLogger.Error(ctx, fetchErrors[0], fmt.Sprintf("Encountered %d errors during fetching", len(fetchErrors)))
		for i, err := range fetchErrors {
			appLogger.Error(ctx, err, fmt.Sprintf("Error %d", i+1))
		}
	} else {
		appLogger.Info(ctx, "Successfully fetched all timeframes", map[string]interface{}{
			"symbol":    symbol,
			"intervals": intervals,
		})
	}
}
