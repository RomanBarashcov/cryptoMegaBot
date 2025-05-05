# Product Context: Crypto Trading Bot

## Purpose & Problem Statement

### Why This Project Exists
The Crypto Trading Bot exists to automate cryptocurrency trading on the Binance futures market, specifically for ETH (Ethereum). It addresses the challenges of:

1. **Emotional Trading**: Human traders often make poor decisions based on fear or greed. This bot removes emotion from trading decisions.

2. **24/7 Market Monitoring**: Cryptocurrency markets operate continuously. The bot provides constant monitoring without human fatigue.

3. **Execution Speed**: In volatile markets, execution speed matters. The bot can react to market conditions faster than manual trading.

4. **Consistency**: The bot applies the same strategy consistently, without deviation or second-guessing.

5. **Risk Management**: Enforces strict risk management rules that might be ignored by human traders in the heat of the moment.

### Problems It Solves

1. **Time Commitment**: Eliminates the need for constant market monitoring by traders
2. **Discipline Issues**: Prevents deviation from trading strategy due to emotional decisions
3. **Missed Opportunities**: Captures trading opportunities that might occur during off-hours
4. **Risk Control**: Enforces consistent position sizing and stop-loss implementation
5. **Record Keeping**: Maintains accurate and complete trading records for performance analysis

## User Experience Goals

### Target Users
- Cryptocurrency traders with Binance futures accounts
- Traders seeking automation for a specific ETH trading strategy
- Users with basic technical knowledge to set up and configure the bot

### User Experience Expectations
1. **Reliability**: The bot should operate consistently without unexpected crashes or errors
2. **Transparency**: Users should be able to view complete trading history and current positions
3. **Configurability**: Key trading parameters should be adjustable without code changes
4. **Safety**: Risk management rules should be strictly enforced to protect capital
5. **Simplicity**: Setup and operation should be straightforward with clear documentation

## Operational Model

### How It Should Work

1. **Initialization**:
   - Load configuration from environment variables
   - Connect to Binance API
   - Initialize database connection
   - Set leverage and other account parameters

2. **Market Monitoring**:
   - Establish WebSocket connection for real-time price updates
   - Process incoming price data
   - Update internal state with current market conditions

3. **Trading Logic**:
   - Evaluate entry conditions based on price trends and volatility
   - Check risk management rules (daily trade limit, open positions)
   - Execute entry orders when conditions are met
   - Set stop-loss and take-profit orders for position management

4. **Position Management**:
   - Monitor open positions
   - Close positions when stop-loss or take-profit levels are reached
   - Record trade outcomes in the database

5. **Reporting**:
   - Log trading activities
   - Maintain comprehensive trade history
   - Calculate and store performance metrics

### Key Workflows

1. **Trade Entry Workflow**:
   - Check if daily trade limit is reached
   - Verify no open position exists
   - Analyze market conditions
   - Calculate position size
   - Execute market order
   - Set stop-loss and take-profit orders
   - Record position in database

2. **Trade Exit Workflow**:
   - Monitor price in relation to stop-loss and take-profit levels
   - Execute market order when exit conditions are met
   - Calculate profit/loss
   - Update position status in database
   - Record completed trade in trade history

3. **Configuration Workflow**:
   - Edit .env file with desired parameters
   - Restart bot to apply new configuration
   - Verify configuration through logs

## Success Metrics

### How Success Is Measured

1. **Profitability**: Positive return on investment over time
2. **Risk Management**: Adherence to stop-loss rules and position sizing
3. **Reliability**: Uptime and consistent operation without errors
4. **Trade Execution**: Accurate entry and exit according to strategy rules
5. **Performance**: Speed of reaction to market conditions

### Expected Outcomes

1. **Consistent Small Profits**: Target of 1-3% profit per trade
2. **Limited Losses**: Stop-loss limiting losses to 0.25% per trade
3. **Overall Positive Return**: Net positive return over time despite some losing trades
4. **Automated Operation**: Minimal need for human intervention
5. **Accurate Record-Keeping**: Complete and accurate trade history for analysis
