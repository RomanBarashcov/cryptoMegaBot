# Active Context: Crypto Trading Bot

## Current Work Focus

### Primary Focus Areas
1. **Strategy Refinement & Development**
   - Tuning existing strategies (e.g., MA Crossover) using backtesting/optimization frameworks.
   - Developing and testing more advanced trading strategies.
   - Implementing improved MA Crossover strategy with day trading optimizations.
   - Investigating volume analysis and multi-timeframe approaches.

2. **Risk Management Enhancement**
   - Implementing dynamic position sizing within the Risk Manager.
   - Considering and potentially implementing circuit breakers for market anomalies.
   - Refining existing risk controls (stop-loss, daily limits).
   - Adding volatility-based position sizing.

3. **Robustness & Testing**
   - Improving comprehensive API error handling and recovery mechanisms.
   - Increasing unit test coverage across all modules.
   - Implementing integration tests with API mocks.
   - Enhancing the logging system for better diagnostics.
   - Conducting extensive backtesting with multiple timeframes.

### In-Progress Tasks (Likely)
- Implementing improved MA Crossover strategy with day trading optimizations.
- Implementing dynamic position sizing algorithm within the Risk Manager.
- Enhancing API error handling routines.
- Increasing unit test coverage for specific modules.
- Developing/testing new trading strategies.
- Refining the basic logging system towards a more comprehensive solution.

## Recent Changes

### Code Changes (Recent Major Updates)
1. Implemented improved MA Crossover strategy with day trading optimizations:
   - Added multi-timeframe analysis (primary, trend, and scalping timeframes)
   - Implemented dynamic position sizing based on volatility
   - Added day trading specific exit conditions (volatility drop, consolidation, market close)
   - Enhanced trailing stop logic with progressive tightening
   - Added pullback detection for entry in established uptrends
   - Implemented scalping opportunity detection for more frequent trading
2. Enhanced backtesting framework to support multiple timeframes and dynamic position sizing.
3. Added analysis tools for evaluating backtest results with focus on day trading metrics.
4. Implemented Strategy framework (`internal/strategy`) including indicators (MA, RSI), backtesting, optimization, and analytics.
5. Implemented dedicated Risk Manager module (`internal/risk`).
6. Implemented basic Logging adapter (`internal/adapters/logger`).
7. Established partial Unit Test coverage across core modules.
8. Implemented robust WebSocket reconnection with exponential backoff.
9. Added position synchronization at startup.
10. Refined configuration validation.

### Architecture Changes
1. Migrated to Clean Architecture / Ports & Adapters structure (`internal/domain`, `internal/app`, `internal/ports`, `internal/adapters`).
2. Implemented interface-driven design for decoupling and testability.
3. Centralized core application logic in `internal/app/service.go`.
4. Introduced dedicated adapters for external dependencies (Binance client, SQLite repository, Logger).
5. Established clear boundaries between application layers.
6. Implemented mutex/sync primitives for concurrency control where needed.
7. Added graceful shutdown handling.

### Configuration Changes
1. Added testnet support.
2. Added configurable leverage setting.
3. Added development mode flag.
4. Refined configuration loading and validation (`config/config.go`).
5. Added day trading specific configuration parameters.

## Next Steps

### Short-term Goals (Reflecting Current Focus)
1. **Complete Improved MA Crossover Implementation**: Finalize and test the improved strategy with day trading optimizations.
2. **Implement Dynamic Position Sizing**: Integrate logic into the Risk Manager.
3. **Enhance Error Handling**: Move from basic to comprehensive API error handling and recovery.
4. **Increase Test Coverage**: Focus on achieving higher unit test coverage and implementing integration tests.
5. **Refine Logging**: Improve structure and detail of the logging system.
6. **Develop Advanced Strategy**: Implement a strategy beyond MA Crossover (e.g., incorporating volume or other indicators).

### Medium-term Goals
1. **Notification System**: Implement alerts (Email, Telegram) for critical events and status updates.
2. **Advanced Risk Features**: Implement circuit breakers, potentially drawdown limits within Risk Manager.
3. **Performance Optimization**: Profile and optimize database operations, memory usage, and execution latency.
4. **Configuration Enhancements**: Consider configuration profiles or hot-reloading if needed.
5. **Explore Further Indicators/Strategies**: Investigate more complex market analysis techniques.

## Active Decisions and Considerations

### Technical Decisions
1. **SQLite vs. PostgreSQL**
   - Decision: Using SQLite for simplicity and embedded operation
   - Trade-off: Limited concurrency but sufficient for single-instance bot
   - Consideration: May need to migrate to PostgreSQL if scaling becomes necessary

2. **WebSocket vs. REST API for Price Updates**
   - Decision: Using WebSocket for real-time price updates
   - Benefit: Lower latency and reduced API call count
   - Implementation: Added robust reconnection handling with exponential backoff

3. **Fixed vs. Dynamic Position Sizing**
   - Decision: Implementing dynamic position sizing based on volatility and risk parameters
   - Benefit: Better risk management and adaptation to market conditions
   - Implementation: Volatility-based sizing with adjustments for consecutive losses

4. **Multi-timeframe Analysis**
   - Decision: Implementing multi-timeframe analysis for better trend confirmation
   - Benefit: More robust trading decisions with higher timeframe trend confirmation
   - Implementation: Primary, trend, and scalping timeframes with specific indicators for each

### Open Questions
1. How to best implement comprehensive error handling and recovery for extended API outages?
2. What specific metrics (beyond basic PnL) in the Analytics module are most crucial for strategy evaluation? (Covered partially by `internal/strategy/analytics`)
3. How to effectively use the optimization framework (`internal/strategy/optimization`) to balance trade frequency vs. profit?
4. Is the current strategy framework sufficient, or is exploring ML components warranted later?
5. What is the optimal approach for integration testing with mocked external APIs (Binance)?
6. How to effectively evaluate day trading strategies across different market conditions?

## Important Patterns and Preferences

### Code Organization
- Adheres to Clean Architecture / Ports & Adapters pattern.
- Clear separation via `internal/domain`, `internal/app`, `internal/ports`, `internal/adapters`.
- Heavy use of interfaces (`internal/ports`) for dependency inversion and testability.
- `cmd/` contains application entry points; `main.go` orchestrates setup and dependency injection.

### Error Handling
- Propagate errors using `fmt.Errorf("context: %w", err)` pattern.
- Basic logging of errors at handling points (needs enhancement).
- Custom error types defined in `internal/ports/errors.go`.
- Panic recovery considered for critical goroutines.
- Comprehensive handling/recovery is an area for improvement.

### Test-Driven Development (TDD)
- Always write tests first before implementing new functionality.
- Start with main test cases covering expected behavior.
- Add edge cases to handle boundary conditions and error scenarios.
- Follow the Red-Green-Refactor cycle:
  1. Write a failing test (Red)
  2. Implement the minimum code to make the test pass (Green)
  3. Refactor the code while keeping tests passing (Refactor)
- Use tests to document expected behavior and serve as living documentation.
- Implement regression tests when fixing bugs to prevent recurrence.

### Configuration Management
- Use environment variables for all configurable parameters
- Provide sensible defaults for optional parameters
- Validate configuration at startup
- Document all configuration options

### Database Operations
- Abstracted via Repository pattern (`internal/ports/repository.go`, `internal/adapters/sqlite/repository.go`).
- Use prepared statements where applicable via `database/sql`.
- Basic error handling implemented in adapter.
- Transactions used for related operations.
- Schema managed via `init.sql`.

## Learnings and Project Insights

### Technical Insights
1. WebSocket connections require careful management for stability
2. SQLite performs well for this use case but requires careful concurrency handling
3. Go's concurrency model works well for this type of application
4. Error handling in distributed systems requires careful consideration
5. Multi-timeframe analysis provides more robust trading decisions

### Trading Insights
1. Small, consistent profits compound effectively over time
2. Strict risk management is essential for long-term success
3. Trading strategy must adapt to changing market conditions
4. Emotional detachment is a key advantage of automated trading
5. Day trading requires different optimization parameters than longer-term strategies

### Project Management Insights
1. Clear documentation is essential for configuration and operation
2. Incremental development with focused goals works well
3. Regular testing in testnet environment prevents costly mistakes
4. Monitoring and logging are as important as the trading logic itself
5. Backtesting across multiple timeframes provides more realistic performance estimates
