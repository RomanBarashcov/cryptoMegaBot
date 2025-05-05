# Crypto Trading Bot

A cryptocurrency trading bot that automatically trades ETH futures on Binance with a focus on risk management and consistent profits.

## Features

- **Clean Architecture:** Built using Ports & Adapters for maintainability and testability.
- **Real-time Price Updates:** Utilizes Binance WebSocket API.
- **Automated Trading:** Executes trades based on configurable strategies.
- **Strategy Framework:**
    - Supports multiple trading strategies (e.g., MA Crossover implemented).
    - Includes technical indicators (Moving Averages, RSI).
    - Backtesting engine for strategy evaluation.
    - Optimization framework for parameter tuning.
    - Performance analytics module.
- **Risk Management:**
    - Dedicated Risk Manager module.
    - Configurable stop-loss and take-profit orders.
    - Daily trade limits.
    - Position sizing (currently fixed default, dynamic planned).
- **Persistence:** Uses SQLite database via Repository pattern for trade history and positions.
- **Configuration:** Highly configurable via environment variables (`.env` file).
- **Concurrency:** Leverages Go's concurrency features for efficient operation.
- **Containerization:** Docker support via `docker-compose.yml`.
- **Testing:** Includes unit tests for core components (coverage ongoing).

## Trading Strategy Framework

The bot employs a flexible strategy framework allowing different algorithms to be implemented and selected.

- **Core Components:** Located in `internal/strategy`.
- **Available Indicators:** Moving Averages (SMA/EMA), Relative Strength Index (RSI). More can be added.
- **Example Strategy:** A Moving Average Crossover strategy (`internal/strategy/strategies/ma_crossover.go`) is provided as an example.
- **Evaluation Tools:** Includes backtesting (`internal/strategy/backtesting`) and parameter optimization (`internal/strategy/optimization`) capabilities.
- **Configuration:** Specific strategy parameters (like MA periods, RSI thresholds) are typically configured via environment variables (see `.env.example` and `config/config.go`).
- **Default Behavior (Configurable):**
    - Position Size: Fixed (e.g., 1.0 ETH), configurable via `QUANTITY`. Dynamic sizing is planned.
    - Stop Loss: Configurable percentage via `STOP_LOSS` (e.g., 0.0025 for 0.25%).
    - Take Profit: Configurable range via `MIN_PROFIT`, `MAX_PROFIT`.
    - Daily Limit: Max trades per day via `MAX_ORDERS`.
    - Leverage: Configurable via `LEVERAGE`.

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
- **Strategy Parameters:** (Specific variables depend on the chosen strategy, e.g., MA periods for MA Crossover)
    - `STRATEGY_NAME`: Identifier for the strategy to use (e.g., `ma_crossover`).
    - *(Strategy-specific params like `MA_SHORT_PERIOD`, `MA_LONG_PERIOD`, `RSI_PERIOD`, etc.)*
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
