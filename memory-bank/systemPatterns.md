# System Patterns: Crypto Trading Bot

## System Architecture

### High-Level Architecture
The Crypto Trading Bot follows a modular architecture with clear separation of concerns:

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│                 │     │                 │     │                 │
│  Configuration  │────▶│  Trading Bot    │◀───▶│  Binance API    │
│                 │     │                 │     │                 │
└─────────────────┘     └────────┬────────┘     └─────────────────┘
                                 │
                                 ▼
                        ┌─────────────────┐
                        │                 │
                        │    Database     │
                        │                 │
                        └─────────────────┘
```

### Component Breakdown
1. **Configuration Module**: Handles loading and validation of environment variables
2. **Trading Bot Core**: Implements trading logic and orchestrates system components
3. **Database Layer**: Manages persistence of trading data and position information
4. **Binance API Client**: Handles communication with Binance Futures API

## Key Technical Decisions

### Language Choice: Go
- **Rationale**: Go provides excellent concurrency support via goroutines, which is essential for handling WebSocket connections and multiple simultaneous operations
- **Benefits**: Strong typing, good performance, built-in concurrency primitives
- **Trade-offs**: Less extensive library ecosystem compared to some languages, steeper learning curve than scripting languages

### Database: SQLite
- **Rationale**: Lightweight, serverless database that doesn't require separate installation
- **Benefits**: Simple setup, file-based storage, ACID compliance
- **Trade-offs**: Limited concurrency support, not suitable for distributed systems

### API Integration: go-binance Library
- **Rationale**: Official Go client for Binance API with comprehensive feature support
- **Benefits**: Well-maintained, handles WebSocket connections, supports all required API endpoints
- **Trade-offs**: Dependency on external library maintenance

### Configuration: Environment Variables
- **Rationale**: Standard approach for configuration that works well with containerization
- **Benefits**: Easy to change without code modifications, works well with Docker
- **Trade-offs**: Limited type safety, requires documentation for users

## Design Patterns

### Singleton Pattern
- **Usage**: Database connection, Binance API client
- **Implementation**: Single instance created at startup and reused throughout application
- **Benefit**: Ensures resource sharing and prevents connection overhead

### Repository Pattern
- **Usage**: Database access layer
- **Implementation**: Database struct with methods for data access operations
- **Benefit**: Abstracts database operations and provides clean interface for data access

### Observer Pattern
- **Usage**: WebSocket price updates
- **Implementation**: WebSocket client subscribes to price updates and notifies trading logic
- **Benefit**: Decouples price monitoring from trading decisions

### Strategy Pattern
- **Usage**: Trading strategy implementation
- **Implementation**: Encapsulated trading logic that can be modified or replaced
- **Benefit**: Allows for future extension with different trading strategies

## Component Relationships

### Configuration → Trading Bot
- Configuration module provides parameters to trading bot at initialization
- Trading bot validates configuration before starting operations

### Trading Bot → Binance API
- Trading bot calls Binance API for:
  - Account information
  - Market data
  - Order placement
  - WebSocket connections

### Trading Bot → Database
- Trading bot persists:
  - Open positions
  - Closed positions
  - Trade history
  - Performance metrics

### Binance API → Trading Bot
- Binance API provides:
  - Real-time price updates via WebSocket
  - Order execution confirmations
  - Account balance information

## Critical Implementation Paths

### Trade Execution Path
1. Price update received from WebSocket
2. Trading conditions evaluated
3. Position size calculated based on account balance
4. Market order created via Binance API
5. Stop-loss and take-profit orders placed
6. Position recorded in database

### Position Management Path
1. Price updates monitored continuously
2. Current price compared to stop-loss and take-profit levels
3. When threshold reached, market order executed to close position
4. Position status updated in database
5. Trade outcome recorded in trade history

### Error Handling Path
1. Error detected (API failure, database error, etc.)
2. Error logged with context information
3. Recovery attempted if possible
4. If critical error, graceful shutdown initiated
5. State preserved for restart

## Data Flow

### Market Data Flow
```
Binance WebSocket → Price Update Handler → Trading Logic → Position Management
```

### Order Flow
```
Trading Logic → Order Creation → Binance API → Order Confirmation → Database Update
```

### Configuration Flow
```
Environment Variables → Config Loader → Validation → Trading Bot Configuration
```

## Concurrency Model

### Goroutines
- WebSocket handler runs in dedicated goroutine
- Trading logic executes in main goroutine
- Database operations are synchronized with mutex to prevent race conditions

### Synchronization
- Mutex protects shared state in trading bot
- Channel-based communication for shutdown signals
- Context propagation for operation cancellation
