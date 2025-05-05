# Crypto Trading Bot

A cryptocurrency trading bot that automatically trades ETH futures on Binance with a focus on risk management and consistent profits.

## Features

- Real-time price updates via WebSocket
- Automated trading with fixed position size (1 ETH)
- Risk management with stop-loss and take-profit orders
- Daily trade limit (5 trades per day)
- 4x leverage for increased potential returns
- SQLite database for trade history
- Configurable trading parameters

## Trading Strategy

- Fixed position size: 1 ETH per trade
- Target profit: 1-3% per trade ($40-$120 at current ETH prices)
- Stop loss: 0.25% ($10 at current ETH prices)
- Maximum 5 trades per day
- 4x leverage for increased potential returns
- Trades only when:
  - Price is trending up
  - Volatility is moderate
  - No open positions
  - Daily trade limit not reached

## Technical Requirements

- Go 1.16 or higher
- SQLite3
- Binance Futures account with API access

## Installation

1. Clone the repository:
```bash
git clone https://github.com/yourusername/cryptoMegaBot.git
cd cryptoMegaBot
```

2. Install dependencies:
```bash
go mod download
```

3. Create and configure your environment file:
```bash
cp .env.example .env
```
Edit `.env` with your Binance API keys and preferred trading parameters.

4. Create data directory:
```bash
mkdir -p data
```

5. Run the bot:
```bash
go run main.go
```

## Configuration

The bot can be configured using environment variables in the `.env` file:

- `BINANCE_API_KEY`: Your Binance API key
- `BINANCE_API_SECRET`: Your Binance API secret
- `SYMBOL`: Trading pair (default: ETHUSDT)
- `LEVERAGE`: Trading leverage (default: 4)
- `QUANTITY`: Position size in ETH (default: 1.0)
- `MAX_ORDERS`: Maximum trades per day (default: 5)
- `MIN_PROFIT`: Minimum profit target (default: 0.01 or 1%)
- `MAX_PROFIT`: Maximum profit target (default: 0.03 or 3%)
- `STOP_LOSS`: Stop loss percentage (default: 0.0025 or 0.25%)
- `DB_PATH`: Path to SQLite database (default: ./data/trading_bot.db)
- `LOG_LEVEL`: Logging level (default: info)

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