package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"cryptoMegaBot/internal/adapters/logger" // Import the logger package for LogLevel
)

// Config holds all application configuration.
type Config struct {
	// Binance API
	APIKey    string
	SecretKey string
	IsTestnet bool

	// Trading Parameters
	Symbol    string
	Leverage  int
	Quantity  float64 // Default quantity if not using dynamic sizing
	MaxOrders int     // Max trades per day
	StopLoss  float64 // Stop loss percentage (e.g., 0.0025 for 0.25%)
	MinProfit float64 // Minimum profit target percentage (e.g., 0.01 for 1%)
	MaxProfit float64 // Maximum profit target percentage (e.g., 0.03 for 3%)

	// Strategy Parameters
	StrategyShortMAPeriod int     // e.g., 20
	StrategyLongMAPeriod  int     // e.g., 50
	StrategyEMAPeriod     int     // e.g., 20
	StrategyRSIPeriod     int     // e.g., 14
	StrategyRSIOverbought float64 // e.g., 70.0
	StrategyRSIOversold   float64 // e.g., 30.0

	// Database
	DBPath string

	// Logging
	LogLevel logger.LogLevel // Use the LogLevel type from the logger adapter

	// Connection Settings (Example for Binance client)
	ReconnectDelay       time.Duration
	MaxReconnectAttempts int

	// Other (Example)
	MinAvailableBalance float64 // Minimum available balance required for trading
}

// LoadConfig loads configuration from environment variables (.env file).
func LoadConfig() (*Config, error) {
	// Load .env file, but don't fail if it doesn't exist (allow pure env vars)
	_ = godotenv.Load()

	cfg := &Config{}
	var err error
	var errs []string // Collect validation errors

	// Binance API
	cfg.APIKey = getEnv("BINANCE_API_KEY", "")
	cfg.SecretKey = getEnv("BINANCE_API_SECRET", "")
	cfg.IsTestnet = getEnvAsBool("IS_TESTNET", true) // Default to testnet for safety

	// Basic API Key validation (can be enhanced)
	if cfg.APIKey == "" {
		errs = append(errs, "BINANCE_API_KEY must be set")
	}
	if cfg.SecretKey == "" {
		errs = append(errs, "BINANCE_API_SECRET must be set")
	}

	// Trading Parameters
	cfg.Symbol = getEnv("SYMBOL", "ETHUSDT")
	if cfg.Symbol == "" {
		errs = append(errs, "SYMBOL must be set")
	}

	cfg.Leverage, err = getEnvAsIntRequired("LEVERAGE", 4)
	if err != nil {
		errs = append(errs, fmt.Sprintf("invalid LEVERAGE: %v", err))
	} else if cfg.Leverage <= 0 {
		errs = append(errs, "LEVERAGE must be positive")
	}

	cfg.Quantity, err = getEnvAsFloatRequired("QUANTITY", 1.0)
	if err != nil {
		errs = append(errs, fmt.Sprintf("invalid QUANTITY: %v", err))
	} else if cfg.Quantity <= 0 {
		errs = append(errs, "QUANTITY must be positive")
	}

	cfg.MaxOrders, err = getEnvAsIntRequired("MAX_ORDERS", 5)
	if err != nil {
		errs = append(errs, fmt.Sprintf("invalid MAX_ORDERS: %v", err))
	} else if cfg.MaxOrders < 0 {
		errs = append(errs, "MAX_ORDERS cannot be negative")
	}

	cfg.StopLoss, err = getEnvAsFloatRequired("STOP_LOSS", 0.0025)
	if err != nil {
		errs = append(errs, fmt.Sprintf("invalid STOP_LOSS: %v", err))
	} else if cfg.StopLoss <= 0 || cfg.StopLoss >= 1.0 {
		errs = append(errs, "STOP_LOSS must be between 0.0 and 1.0 (exclusive)")
	}

	// Load Min/Max Profit targets
	cfg.MinProfit, err = getEnvAsFloatRequired("MIN_PROFIT", 0.01) // Default 1%
	if err != nil {
		errs = append(errs, fmt.Sprintf("invalid MIN_PROFIT: %v", err))
	} else if cfg.MinProfit <= 0 {
		errs = append(errs, "MIN_PROFIT must be positive")
	}

	cfg.MaxProfit, err = getEnvAsFloatRequired("MAX_PROFIT", 0.03) // Default 3%
	if err != nil {
		errs = append(errs, fmt.Sprintf("invalid MAX_PROFIT: %v", err))
	} else if cfg.MaxProfit <= 0 {
		errs = append(errs, "MAX_PROFIT must be positive")
	}

	if cfg.MinProfit >= cfg.MaxProfit {
		errs = append(errs, "MIN_PROFIT must be less than MAX_PROFIT")
	}

	// Strategy Parameters (using defaults if not set)
	cfg.StrategyShortMAPeriod = getEnvAsInt("STRATEGY_SHORT_MA_PERIOD", 20)
	cfg.StrategyLongMAPeriod = getEnvAsInt("STRATEGY_LONG_MA_PERIOD", 50)
	cfg.StrategyEMAPeriod = getEnvAsInt("STRATEGY_EMA_PERIOD", 20)
	cfg.StrategyRSIPeriod = getEnvAsInt("STRATEGY_RSI_PERIOD", 14)
	cfg.StrategyRSIOverbought = getEnvAsFloat("STRATEGY_RSI_OVERBOUGHT", 70.0)
	cfg.StrategyRSIOversold = getEnvAsFloat("STRATEGY_RSI_OVERSOLD", 30.0)

	// Validate strategy periods
	if cfg.StrategyShortMAPeriod <= 0 || cfg.StrategyLongMAPeriod <= 0 || cfg.StrategyEMAPeriod <= 0 || cfg.StrategyRSIPeriod <= 0 {
		errs = append(errs, "strategy periods (MA, EMA, RSI) must be positive")
	}
	if cfg.StrategyShortMAPeriod >= cfg.StrategyLongMAPeriod {
		errs = append(errs, "STRATEGY_SHORT_MA_PERIOD must be less than STRATEGY_LONG_MA_PERIOD")
	}
	if cfg.StrategyRSIOverbought <= cfg.StrategyRSIOversold || cfg.StrategyRSIOverbought > 100 || cfg.StrategyRSIOversold < 0 {
		errs = append(errs, "invalid RSI thresholds (Overbought must be > Oversold, between 0-100)")
	}

	// Database
	cfg.DBPath = getEnv("DB_PATH", "./data/trading_bot.db")
	if cfg.DBPath == "" {
		errs = append(errs, "DB_PATH must be set")
	}

	// Logging
	logLevelStr := getEnv("LOG_LEVEL", "INFO")
	cfg.LogLevel = logger.ParseLevel(logLevelStr) // Use the parser from the logger package

	// Connection Settings
	reconnectDelaySeconds := getEnvAsInt("RECONNECT_DELAY_SECONDS", 5)
	if reconnectDelaySeconds <= 0 {
		errs = append(errs, "RECONNECT_DELAY_SECONDS must be positive")
	}
	cfg.ReconnectDelay = time.Duration(reconnectDelaySeconds) * time.Second

	cfg.MaxReconnectAttempts = getEnvAsInt("MAX_RECONNECT_ATTEMPTS", 10)
	if cfg.MaxReconnectAttempts < 0 {
		errs = append(errs, "MAX_RECONNECT_ATTEMPTS cannot be negative")
	}

	// Other
	cfg.MinAvailableBalance, err = getEnvAsFloatRequired("MIN_AVAILABLE_BALANCE", 100.0)
	if err != nil {
		errs = append(errs, fmt.Sprintf("invalid MIN_AVAILABLE_BALANCE: %v", err))
	} else if cfg.MinAvailableBalance < 0 {
		errs = append(errs, "MIN_AVAILABLE_BALANCE cannot be negative")
	}

	// Combine validation errors
	if len(errs) > 0 {
		return nil, fmt.Errorf("configuration validation failed: %s", strings.Join(errs, "; "))
	}

	return cfg, nil
}

// --- Env Var Helpers ---

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		// Log warning? For non-required fields, default is often acceptable.
		return defaultValue
	}
	return value
}

func getEnvAsIntRequired(key string, defaultValue int) (int, error) {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		// Use default if env var is not set at all
		return defaultValue, nil
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		// Return error if env var is set but invalid
		return 0, fmt.Errorf("invalid integer value '%s' for key %s: %w", valueStr, key, err)
	}
	return value, nil
}

func getEnvAsFloat(key string, defaultValue float64) float64 {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsFloatRequired(key string, defaultValue float64) (float64, error) {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue, nil
	}
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid float value '%s' for key %s: %w", valueStr, key, err)
	}
	return value, nil
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
