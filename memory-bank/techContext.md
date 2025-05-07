# Technical Context: Crypto Trading Bot

## Technologies Used

### Programming Language
- **Go (Golang)** v1.16+
  - Statically typed, compiled language
  - Excellent concurrency support via goroutines and channels
  - Strong standard library
  - Good performance characteristics

### Database
- **SQLite** v3
  - Serverless, file-based relational database
  - ACID-compliant
  - Zero configuration required
  - Embedded within application

### External APIs
- **Binance Futures API**
  - REST API for account management and order execution
  - WebSocket API for real-time market data
  - Requires API key and secret for authentication
  - Rate limits apply to API calls

### Libraries & Dependencies
- **go-binance/v2/futures**
  - Official Go client for Binance Futures API
  - Handles authentication, request signing, and WebSocket connections
  
- **joho/godotenv**
  - Environment variable loading from .env files
  - Simplifies configuration management

- **mattn/go-sqlite3**
  - SQLite driver for Go's database/sql package
  - CGO-based implementation

### Containerization
- **Docker**
  - Application containerization
  - Defined in docker-compose.yml
  - Simplifies deployment and environment consistency

## Development Setup

### Prerequisites
- Go 1.16 or higher installed
- Git for version control
- SQLite3 development libraries
- Docker and Docker Compose (optional, for containerized deployment)

### Local Development Environment
1. Clone repository
2. Copy .env.example to .env and configure
3. Run `go mod download` to install dependencies
4. Create data directory with `mkdir -p data`
5. Run application with `go run main.go`

### Directory Structure
```
cryptoMegaBot/
├── cmd/                  # Application entry points (e.g., main bot, test runners)
│   ├── analyze_backtests/ # Backtest analysis tools
│   ├── backtest_runner/   # Backtesting runner
│   ├── fetch_klines/      # Data fetching utilities
│   └── test_runner/       # Test execution utilities
├── config/
│   └── config.go         # Configuration loading and validation
├── data/
│   └── trading_bot.db    # SQLite database file (if used locally)
├── internal/             # Internal application logic (not importable by others)
│   ├── adapters/         # Adapters for external dependencies (Ports implementation)
│   │   ├── binanceclient/ # Binance API client adapter
│   │   ├── logger/        # Logging adapter (e.g., StdLogger)
│   │   └── sqlite/        # SQLite repository adapter
│   ├── app/              # Application core service/use cases
│   │   └── service.go
│   ├── domain/           # Core domain models (Position, Trade, Kline, etc.)
│   ├── ports/            # Interfaces defining application ports (Repository, Exchange, Strategy, etc.)
│   ├── risk/             # Risk management logic
│   │   └── manager.go
│   └── strategy/         # Trading strategy components
│       ├── analytics/     # Performance calculation
│       ├── backtesting/   # Backtesting engine
│       ├── indicators/    # Technical indicators (MA, RSI, ATR, etc.)
│       ├── optimization/  # Strategy parameter optimization
│       └── strategies/    # Specific strategy implementations (e.g., MA Crossover, Improved MA Crossover)
├── memory-bank/          # Project context documentation (this folder)
│   └── ...
├── .env                  # Environment configuration (local, not in repo)
├── .env.example          # Example environment configuration
├── .gitignore            # Git ignore rules
├── docker-compose.yml    # Docker Compose configuration
├── go.mod                # Go module definition
├── go.sum                # Go module checksums
├── init.sql              # Database initialization script
├── Makefile              # Build/test automation commands
└── README.md             # Project documentation
```

### Build Process
1. Ensure dependencies are installed: `go mod download`
2. Build executable: `go build -o cryptoMegaBot`
3. Run executable: `./cryptoMegaBot`

### Docker Deployment
1. Build container: `docker-compose build`
2. Run container: `docker-compose up -d`
3. View logs: `docker-compose logs -f`
4. Stop container: `docker-compose down`

## Technical Constraints

### Performance Constraints
- **WebSocket Connection**: Must maintain stable WebSocket connection for real-time price updates
- **Order Execution Speed**: Critical for timely entry and exit from positions
- **Database Operations**: Must not block trading operations

### Security Constraints
- **API Key Protection**: API keys must be securely stored and never exposed
- **Database Security**: Trading data must be protected from unauthorized access
- **Error Handling**: Must not expose sensitive information in error messages

### Operational Constraints
- **Binance API Rate Limits**: Must respect API rate limits to avoid temporary bans
- **Network Reliability**: Must handle network interruptions gracefully
- **Database Locking**: SQLite has limitations with concurrent write operations

### Compatibility Constraints
- **Go Version**: Must be compatible with Go 1.16+
- **SQLite Version**: Must be compatible with SQLite 3
- **Binance API Version**: Must adapt to Binance API changes

## Dependencies

### Direct Dependencies
- **github.com/adshao/go-binance/v2/futures**: Binance Futures API client
  - Version: Latest
  - Purpose: Interact with Binance Futures API
  - License: MIT

- **github.com/joho/godotenv**: Environment variable loader
  - Version: Latest
  - Purpose: Load configuration from .env files
  - License: MIT

- **github.com/mattn/go-sqlite3**: SQLite driver
  - Version: Latest
  - Purpose: Database operations
  - License: MIT

### Indirect Dependencies
- Standard library packages:
  - context
  - database/sql
  - fmt
  - log
  - os
  - os/signal
  - strconv
  - sync
  - syscall
  - time

### External Services
- **Binance Futures API**
  - Dependency Type: External API
  - Criticality: High (application cannot function without it)
  - Fallback: None (core functionality)

## Tool Usage Patterns

### Configuration Management
- Environment variables loaded from .env file
- Default values provided for optional configuration
- Validation performed at startup
- Configuration object passed to components that need it

### Database Operations
- Repository pattern for data access
- Prepared statements for SQL queries
- Transactions for related operations
- Error handling with context information

### API Interaction
- Client initialized once at startup
- Authentication handled by go-binance library
- WebSocket for real-time data
- REST API for account operations and order execution

### Concurrency Management
- Mutex for protecting shared state
- Channels for communication between goroutines
- Context for operation cancellation
- Graceful shutdown handling

### Logging
- Structured logging with standard log package
- Error context included in log messages
- Different log levels based on configuration
- Critical errors logged before shutdown

### Error Handling
- Errors propagated up the call stack
- Context added at each level
- Recovery from panics in critical goroutines
- Graceful degradation when possible

### Test-Driven Development
- Tests written before implementing functionality
- Main test cases cover expected behavior
- Edge cases handle boundary conditions and error scenarios
- Red-Green-Refactor cycle followed:
  1. Write a failing test (Red)
  2. Implement minimum code to make test pass (Green)
  3. Refactor while keeping tests passing (Refactor)
- Tests serve as documentation of expected behavior
- Regression tests implemented for bug fixes

### Strategy Implementation
- Strategy pattern for trading algorithms
- Interface-based design for strategy interchangeability
- Technical indicators as reusable components
- Backtesting framework for strategy evaluation
- Multi-timeframe analysis for more robust trading decisions
- Day trading optimizations with specialized exit conditions

### Backtesting Framework
- Support for multiple timeframes
- Dynamic position sizing based on volatility
- ATR-based stop loss calculation
- Performance metrics calculation (win rate, profit factor, Sharpe ratio)
- Trade history recording for analysis
- Specialized analysis tools for day trading metrics
