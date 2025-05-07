# Crypto Trading Bot

A cryptocurrency trading bot that automatically trades ETH futures on Binance with a focus on risk management and consistent profits.

## Features

- **Clean Architecture:** Built using Ports & Adapters for maintainability and testability.
- **Real-time Price Updates:** Utilizes Binance WebSocket API.
- **Automated Trading:** Executes trades based on configurable strategies.
- **Strategy Framework:**
    - Supports multiple trading strategies (MA Crossover and Improved MA Crossover implemented).
    - Includes technical indicators (Moving Averages, RSI, ATR).
    - Multi-timeframe analysis for more robust trading decisions.
    - Backtesting engine with multi-timeframe support.
    - Optimization framework for parameter tuning.
    - Performance analytics module.
- **Risk Management:**
    - Dedicated Risk Manager module.
    - Configurable stop-loss and take-profit orders.
    - Daily trade limits.
    - Dynamic position sizing based on volatility (in Improved MA Crossover).
    - Trailing stop-loss with progressive tightening.
- **Persistence:** Uses SQLite database via Repository pattern for trade history and positions.
- **Configuration:** Highly configurable via environment variables (`.env` file).
- **Concurrency:** Leverages Go's concurrency features for efficient operation.
- **Containerization:** Docker support via `docker-compose.yml`.
- **Testing:** Includes unit tests for core components (coverage ongoing).

## Trading Strategy Framework

The bot employs a flexible strategy framework allowing different algorithms to be implemented and selected.

- **Core Components:** Located in `internal/strategy`.
- **Available Indicators:** Moving Averages (SMA/EMA), Relative Strength Index (RSI), Average True Range (ATR). More can be added.
- **Available Strategies:**
  - **MA Crossover:** Basic moving average crossover strategy (`internal/strategy/strategies/ma_crossover.go`).
  - **Improved MA Crossover:** Enhanced strategy with day trading optimizations (`internal/strategy/strategies/improved_ma_crossover.go`).
    - Multi-timeframe analysis (primary, trend, and scalping timeframes)
    - Dynamic position sizing based on volatility
    - Enhanced trailing stop logic with progressive tightening
    - Advanced exit conditions (volatility drop, consolidation, market close)
    - Pullback detection for entry in established uptrends
    - Scalping opportunity detection for more frequent trading
- **Evaluation Tools:** 
  - Backtesting (`internal/strategy/backtesting`) with multi-timeframe support
  - Parameter optimization (`internal/strategy/optimization`) capabilities
  - Backtest analysis tools (`cmd/analyze_backtests`) for detailed performance metrics
- **Configuration:** Specific strategy parameters (like MA periods, RSI thresholds) are typically configured via environment variables (see `.env.example` and `config/config.go`).
- **Default Behavior (Configurable):**
    - Position Size: Dynamic based on volatility (in Improved MA Crossover) or fixed (configurable via `QUANTITY`).
    - Stop Loss: Dynamic based on ATR or configurable percentage via `STOP_LOSS` (e.g., 0.0025 for 0.25%).
    - Take Profit: Configurable range via `MIN_PROFIT`, `MAX_PROFIT`.
    - Daily Limit: Max trades per day via `MAX_ORDERS`.
    - Leverage: Configurable via `LEVERAGE` with dynamic adjustment based on market conditions (in Improved MA Crossover).

## Technical Requirements

- Go 1.16 or higher
- SQLite3
- Binance Futures account with API access

## Installation & Running

### Prerequisites
- Go (version specified in `go.mod`, e.g., 1.16+)
- Git
- SQLite3 development libraries (for `mattn/go-sqlite3`)
- Docker & Docker Compose (Optional, for containerized deployment)

### Local Setup

1.  **Clone:**
    ```bash
    git clone https://github.com/yourusername/cryptoMegaBot.git # Replace with actual repo URL
    cd cryptoMegaBot
    ```
2.  **Dependencies:**
    ```bash
    go mod download
    ```
3.  **Configuration:**
    ```bash
    cp .env.example .env
    # Edit .env with your Binance API keys and desired parameters
    ```
4.  **Data Directory:** (If using local SQLite)
    ```bash
    mkdir -p data
    ```
5.  **Build & Run (using Makefile):**
    ```bash
    # Build the executable
    make build

    # Run the built executable
    ./cryptoMegaBot
    ```
    *Alternatively, run directly (slower):*
    ```bash
    make run
    # Or: go run cmd/main.go (adjust path to main entry point if needed)
    ```

### Docker Setup

1.  **Configuration:** Ensure `.env` is created and configured as above.
2.  **Build & Run:**
    ```bash
    docker-compose build
    docker-compose up -d
    ```
3.  **View Logs:**
    ```bash
    docker-compose logs -f
    ```
4.  **Stop:**
    ```bash
    docker-compose down
    ```

### Backtesting

The bot includes a comprehensive backtesting framework for strategy evaluation:

1. **Fetch Historical Data:**
   ```bash
   go run cmd/fetch_klines/main.go
   ```
   This will download historical klines for the configured symbol and timeframes.

2. **Run Backtest:**
   ```bash
   go run cmd/backtest_runner/main.go
   ```
   This will run the backtest using the configured strategy and parameters.

3. **Analyze Results:**
   ```bash
   go run cmd/analyze_backtests/main.go
   ```
   This will analyze the backtest results and provide detailed performance metrics.

## Configuration

Configuration is managed via environment variables, typically loaded from an `.env` file using `godotenv`. See `.env.example` for a full list of available parameters. Key variables include:

- **API Credentials:**
    - `BINANCE_API_KEY`: Your Binance API key.
    - `BINANCE_API_SECRET`: Your Binance API secret.
- **Trading Parameters:**
    - `SYMBOL`: Trading pair (e.g., `ETHUSDT`).
    - `LEVERAGE`: Desired leverage.
    - `QUANTITY`: Position size (e.g., in ETH for ETHUSDT).
- **Risk Management:**
    - `MAX_ORDERS`: Maximum trades per day.
    - `STOP_LOSS`: Stop loss percentage (e.g., `0.0025` for 0.25%).
    - `MIN_PROFIT`, `MAX_PROFIT`: Take profit range percentages.
- **Strategy Parameters:** (Specific variables depend on the chosen strategy)
    - `STRATEGY_NAME`: Identifier for the strategy to use (e.g., `ma_crossover`, `improved_ma_crossover`).
    - **MA Crossover Parameters:**
      - `MA_SHORT_PERIOD`: Period for the fast moving average.
      - `MA_LONG_PERIOD`: Period for the slow moving average.
      - `RSI_PERIOD`: Period for the RSI indicator.
    - **Improved MA Crossover Parameters:**
      - `FAST_MA_PERIOD`: Period for the fast moving average.
      - `SLOW_MA_PERIOD`: Period for the slow moving average.
      - `SIGNAL_PERIOD`: Period for the signal line.
      - `ATR_PERIOD`: Period for the ATR indicator.
      - `ATR_MULTIPLIER`: Multiplier for ATR-based stops.
      - `USE_MULTI_TIMEFRAME`: Whether to use multi-timeframe analysis.
      - `PRIMARY_TIMEFRAME`: Primary timeframe for trading decisions.
      - `TREND_TIMEFRAME`: Higher timeframe for trend confirmation.
      - `USE_SCALP_TIMEFRAME`: Whether to use scalping timeframe.
      - `SCALP_TIMEFRAME`: Shorter timeframe for scalping opportunities.
      - `MAX_DAILY_LOSSES`: Maximum number of losing trades per day.
      - `MAX_HOLDING_TIME`: Maximum time to hold a position.
- **Technical:**
    - `DB_PATH`: Path to SQLite database file.
    - `LOG_LEVEL`: Logging verbosity (e.g., `debug`, `info`, `warn`, `error`).
    - `TESTNET_ENABLED`: Set to `true` to use Binance Testnet.

## Risk Warning

This bot is designed for educational purposes. Cryptocurrency trading carries significant risks:

- Potential loss of entire investment
- High volatility in crypto markets
- Leverage can amplify both gains and losses
- Technical issues may cause unexpected behavior

Always:
- Use only funds you can afford to lose
- Monitor the bot's performance regularly
- Keep your API keys secure
- Test thoroughly in paper trading mode first

## License

MIT License
