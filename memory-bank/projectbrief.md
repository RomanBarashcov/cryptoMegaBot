# Project Brief: Crypto Trading Bot

## Project Overview
The Crypto Trading Bot is an automated trading system designed to trade ETH futures on Binance. The bot focuses on risk management and consistent profits through a predefined trading strategy.

## Core Requirements

### Functional Requirements
1. **Automated Trading**: Execute trades automatically based on predefined strategy
2. **Real-time Price Monitoring**: Connect to Binance via WebSocket for real-time price updates
3. **Position Management**: Open and close positions with stop-loss and take-profit orders
4. **Risk Management**: Implement strict risk controls including position sizing and daily trade limits
5. **Trade History**: Record and store all trading activity for analysis
6. **Configuration**: Allow customization of trading parameters via environment variables

### Technical Requirements
1. **Go Implementation**: Built using Go 1.16 or higher
2. **Database Storage**: Use SQLite for persistent storage of trade data
3. **Binance API Integration**: Connect to Binance Futures API for trading
4. **Docker Support**: Containerization for easy deployment
5. **Environment Configuration**: Use .env files for configuration

## Project Goals
1. Generate consistent profits through automated trading
2. Minimize risk through strict risk management rules
3. Provide transparency through detailed trade history
4. Allow customization of trading parameters
5. Ensure reliability and stability in operation

## Project Scope

### In Scope
- ETH futures trading on Binance
- Fixed position size trading (1 ETH per trade)
- Stop-loss and take-profit order management
- Daily trade limits
- Trade history recording and reporting
- Configuration via environment variables

### Out of Scope
- Trading multiple cryptocurrencies simultaneously
- Complex trading strategies requiring machine learning
- Web interface for monitoring (command-line only)
- Portfolio management across multiple assets
- Tax reporting features

## Success Criteria
1. Bot successfully executes trades according to strategy
2. Risk management rules are strictly enforced
3. Trade history is accurately recorded
4. Configuration options work as expected
5. System remains stable during operation
