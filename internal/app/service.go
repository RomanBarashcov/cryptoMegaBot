package app

import (
	"context"
	"errors" // Need for error checking in cancelOrderWarn
	"fmt"
	"os"
	"os/signal"
	"strconv" // Need for formatting quantity/prices
	"sync"
	"syscall"
	"time"

	"cryptoMegaBot/config"
	"cryptoMegaBot/internal/domain"
	"cryptoMegaBot/internal/ports"
)

const (
	maxKlineCacheSize = 500 // Limit cache size to avoid memory issues
)

// TradingService orchestrates the trading bot's operations.
type TradingService struct {
	cfg        *config.Config
	logger     ports.Logger
	exchange   ports.ExchangeClient
	posRepo    ports.PositionRepository
	tradeRepo  ports.TradeRepository
	strategy   ports.Strategy
	klineCache []*domain.Kline // Simple cache for strategy calculations

	// State fields
	mu              sync.Mutex // Protects access to state fields below
	currentPosition *domain.Position
	tradesToday     int
}

// NewTradingService creates a new application service instance.
func NewTradingService(
	cfg *config.Config,
	logger ports.Logger,
	exchange ports.ExchangeClient,
	posRepo ports.PositionRepository,
	tradeRepo ports.TradeRepository,
	strat ports.Strategy,
) (*TradingService, error) {

	// Validate dependencies
	if cfg == nil || logger == nil || exchange == nil || posRepo == nil || tradeRepo == nil || strat == nil {
		return nil, fmt.Errorf("missing required dependencies for TradingService")
	}

	// Validate config values needed by service
	if cfg.Quantity <= 0 {
		return nil, fmt.Errorf("configuration Quantity must be positive")
	}
	if cfg.StopLoss <= 0 || cfg.StopLoss >= 1 {
		return nil, fmt.Errorf("configuration StopLoss must be between 0 and 1")
	}
	if cfg.MaxProfit <= 0 { // Using MaxProfit based on user feedback
		return nil, fmt.Errorf("configuration MaxProfit must be positive")
	}
	if cfg.MaxOrders <= 0 {
		return nil, fmt.Errorf("configuration MaxOrders must be positive")
	}

	return &TradingService{
		cfg:        cfg,
		logger:     logger,
		exchange:   exchange,
		posRepo:    posRepo,
		tradeRepo:  tradeRepo,
		strategy:   strat,
		klineCache: make([]*domain.Kline, 0, maxKlineCacheSize), // Initialize cache
	}, nil
}

// Start begins the trading bot's main loop.
func (s *TradingService) Start(ctx context.Context) error {
	s.logger.Info(ctx, "Starting Trading Service...")

	// Create a context that can be canceled by signals
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		s.logger.Info(ctx, "Received shutdown signal", map[string]interface{}{"signal": sig.String()})
		cancel() // Cancel the main context
	}()

	// --- Initialization Steps ---
	// 1. Set server time (important for API calls)
	if err := s.exchange.SetServerTime(ctx); err != nil {
		s.logger.Error(ctx, err, "Failed to synchronize server time")
		// Decide if this is fatal. For trading, it likely is.
		return fmt.Errorf("failed to set server time: %w", err)
	}
	s.logger.Info(ctx, "Server time synchronized")

	// 2. Check if futures trading is enabled
	pos, err := s.exchange.GetPositionRisk(ctx, s.cfg.Symbol)
	if err != nil {
		s.logger.Error(ctx, err, "Failed to check futures trading status", map[string]interface{}{"symbol": s.cfg.Symbol})
		return fmt.Errorf("failed to check futures trading status: %w", err)
	}

	// 3. Set Leverage only if current leverage is different
	currentLeverage := 1 // Default leverage
	if pos != nil {
		currentLeverage = pos.Leverage
	}

	if currentLeverage != s.cfg.Leverage {
		// Try to set leverage
		if err := s.exchange.SetLeverage(ctx, s.cfg.Symbol, s.cfg.Leverage); err != nil {
			s.logger.Error(ctx, err, "Failed to set leverage", map[string]interface{}{
				"symbol":          s.cfg.Symbol,
				"currentLeverage": currentLeverage,
				"targetLeverage":  s.cfg.Leverage,
			})
			// Continue with current leverage instead of failing
			s.logger.Warn(ctx, "Continuing with current leverage", map[string]interface{}{
				"symbol":   s.cfg.Symbol,
				"leverage": currentLeverage,
			})
			// Update config to use current leverage
			s.cfg.Leverage = currentLeverage
		} else {
			s.logger.Info(ctx, "Leverage set successfully", map[string]interface{}{
				"symbol":   s.cfg.Symbol,
				"leverage": s.cfg.Leverage,
			})
		}
	} else {
		s.logger.Info(ctx, "Leverage already set correctly", map[string]interface{}{
			"symbol":   s.cfg.Symbol,
			"leverage": currentLeverage,
		})
	}

	// 4. Sync existing position state (if any)
	s.logger.Info(ctx, "Synchronizing initial state...")
	openPos, err := s.posRepo.FindOpenBySymbol(ctx, s.cfg.Symbol)
	if err != nil {
		// Log error but continue, assuming no open position if DB fails? Or make it fatal?
		// Let's make it fatal for now, as state is critical.
		s.logger.Error(ctx, err, "Failed to check for existing open position")
		s.logger.Info(ctx, "No existing open position found")
		return fmt.Errorf("failed to query open position: %w", err)
	}
	if openPos != nil {
		s.currentPosition = openPos
		s.logger.Info(ctx, "Found existing open position", map[string]interface{}{"positionID": openPos.ID, "entryPrice": openPos.EntryPrice, "takeProfit": openPos.TakeProfit, "stopLoss": openPos.StopLoss})
		// TODO: Potentially sync SL/TP order status with exchange here? This is complex.
		// For now, assume SL/TP orders placed previously are still active if the position is open.
		// A more robust solution would involve querying open orders.
	} else {
		s.logger.Info(ctx, "No existing open position found")
	}

	tradesCount, err := s.tradeRepo.CountTodayBySymbol(ctx, s.cfg.Symbol)
	if err != nil {
		// Make this fatal as well, trade limit is important.
		s.logger.Error(ctx, err, "Failed to count trades for today")
		return fmt.Errorf("failed to count today's trades: %w", err)
	}
	s.tradesToday = tradesCount
	s.logger.Info(ctx, "Initial state synchronized", map[string]interface{}{"tradesToday": s.tradesToday})

	// 5. Load initial klines for strategy
	requiredPoints := s.strategy.RequiredDataPoints()
	s.logger.Info(ctx, "Loading initial klines for strategy", map[string]interface{}{"requiredPoints": requiredPoints})
	initialKlines, err := s.exchange.GetKlines(ctx, s.cfg.Symbol, "1m", requiredPoints)
	if err != nil {
		s.logger.Error(ctx, err, "Failed to load initial klines for strategy")
		return fmt.Errorf("failed to load initial klines: %w", err)
	}
	if len(initialKlines) < requiredPoints {
		err := fmt.Errorf("not enough initial klines loaded (%d) to meet strategy requirement (%d)", len(initialKlines), requiredPoints)
		s.logger.Error(ctx, err, "Insufficient historical data")
		return err
	}
	s.klineCache = initialKlines // Assuming GetKlines returns []*domain.Kline
	s.logger.Info(ctx, "Loaded initial klines", map[string]interface{}{"count": len(s.klineCache)})

	// --- Start WebSocket Stream ---
	wsDoneCh, wsStopCh, err := s.exchange.StreamKlines(ctx, s.cfg.Symbol, "1m", s.handleKlineEvent, s.handleWsError)
	if err != nil {
		s.logger.Error(ctx, err, "Failed to start WebSocket stream")
		return fmt.Errorf("failed to start WebSocket stream: %w", err)
	}
	s.logger.Info(ctx, "WebSocket stream started", map[string]interface{}{"symbol": s.cfg.Symbol, "interval": "1m"})

	// --- Main Loop ---
	// The main work happens in handleKlineEvent triggered by the WebSocket stream.
	// We just need to wait for the context to be canceled or the WebSocket to finish.

	select {
	case <-ctx.Done():
		s.logger.Info(ctx, "Main context cancelled, initiating shutdown...")
		// Signal WebSocket to stop
		select {
		case wsStopCh <- struct{}{}:
			s.logger.Info(ctx, "Stop signal sent to WebSocket stream")
		default:
			s.logger.Warn(ctx, "Failed to send stop signal to WebSocket (already closed?)")
		}
		// Wait briefly for WebSocket to close gracefully
		select {
		case <-wsDoneCh:
			s.logger.Info(ctx, "WebSocket stream shut down gracefully")
		case <-time.After(5 * time.Second): // Timeout for WS shutdown
			s.logger.Warn(ctx, "Timeout waiting for WebSocket stream to shut down")
		}
	case <-wsDoneCh:
		// WebSocket closed unexpectedly (e.g., max reconnect attempts failed)
		s.logger.Error(ctx, fmt.Errorf("websocket stream closed unexpectedly"), "WebSocket stream stopped")
		// The service should probably exit here.
		return fmt.Errorf("websocket stream stopped unexpectedly")
	}

	s.logger.Info(ctx, "Trading Service stopped.")
	return nil
}

// handleKlineEvent processes incoming kline data from the WebSocket.
// This is the core logic loop triggered by new price data.
func (s *TradingService) handleKlineEvent(kline *domain.Kline) {
	// Use a background context for handlers for now, consider request-scoped if needed later
	ctx := context.Background()
	currentPrice := kline.Close // Use the closing price of the latest kline

	s.logger.Debug(ctx, "Received kline event", map[string]interface{}{
		"symbol":    kline.Symbol,
		"interval":  kline.Interval,
		"closeTime": kline.CloseTime,
		"close":     currentPrice,
		"isFinal":   kline.IsFinal,
	})

	// Only process final klines to avoid acting on incomplete data
	if !kline.IsFinal {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Update kline cache
	s.klineCache = append(s.klineCache, kline)
	// Trim cache if it exceeds max size
	if len(s.klineCache) > maxKlineCacheSize {
		// Keep the most recent maxKlineCacheSize elements
		s.klineCache = s.klineCache[len(s.klineCache)-maxKlineCacheSize:]
	}

	// --- Check Close Conditions ---
	if s.currentPosition != nil {
		// Check strategy-based exit conditions first
		shouldClose, reason := s.strategy.ShouldClosePosition(ctx, s.currentPosition, s.klineCache, currentPrice)
		if shouldClose {
			s.logger.Info(ctx, "Strategy indicates position should be closed", map[string]interface{}{"positionID": s.currentPosition.ID, "reason": reason})
			// Attempt to close the position
			err := s.closePosition(ctx, currentPrice, reason)
			if err != nil {
				s.logger.Error(ctx, err, "Failed to close position based on strategy signal", map[string]interface{}{"positionID": s.currentPosition.ID})
				// Decide how to handle failure: retry? alert? For now, just log.
			}
			// Whether close succeeded or failed, we don't check for entry in the same event
			return
		}
		// Note: SL/TP might be handled by exchange orders directly.
		// If ShouldClosePosition also checks SL/TP, this covers it.
		// If SL/TP are purely exchange-based, we might need order update events.
	}

	// --- Check Entry Conditions ---
	if s.currentPosition == nil { // Only check entry if no position is open
		canTradeNow, reason := s.canTrade(ctx)
		if !canTradeNow {
			s.logger.Debug(ctx, "Cannot trade now", map[string]interface{}{"reason": reason})
			return
		}

		// Check strategy entry conditions
		if s.strategy.ShouldEnterTrade(ctx, s.klineCache, currentPrice) {
			s.logger.Info(ctx, "Strategy indicates a trade should be entered")
			// Attempt to enter a position (assuming LONG for now)
			err := s.enterPosition(ctx, currentPrice)
			if err != nil {
				s.logger.Error(ctx, err, "Failed to enter position based on strategy signal")
				// Decide how to handle failure. Log for now.
			}
			// Whether entry succeeded or failed, processing for this event is done.
			return
		}
	}
}

// handleWsError handles errors reported by the WebSocket stream.
func (s *TradingService) handleWsError(err error) {
	ctx := context.Background() // Use a background context for handlers
	s.logger.Error(ctx, err, "WebSocket stream error reported")
	// Decide on action: e.g., trigger shutdown if error is persistent or critical.
	// The reconnection logic is handled within the adapter. This handler
	// is for errors reported *during* a connection or persistent connection failures.
	// Consider adding logic here to potentially cancel the main context if errors are critical.
}

// --- Private helper methods for trading actions ---

// canTrade checks if the bot is currently allowed to open a new position.
// NOTE: This method assumes the mutex `s.mu` is already locked by the caller (`handleKlineEvent`).
func (s *TradingService) canTrade(ctx context.Context) (bool, string) {
	// 1. Check if position is already open
	if s.currentPosition != nil {
		return false, fmt.Sprintf("position %d already open", s.currentPosition.ID)
	}

	// 2. Check daily trade limit
	// We need to refresh tradesToday count from DB in case the bot restarted mid-day
	// Or, trust the in-memory count if it's guaranteed to be accurate since start.
	// Let's trust in-memory for now, but add a TODO to consider DB refresh.
	// TODO: Consider refreshing tradesToday from DB periodically or on error?
	if s.tradesToday >= s.cfg.MaxOrders {
		return false, fmt.Sprintf("daily trade limit reached (%d/%d)", s.tradesToday, s.cfg.MaxOrders)
	}

	// 3. Check minimum balance (Optional but recommended)
	// balance, err := s.exchange.GetAccountBalance(ctx, "USDT") // Assuming USDT balance
	// if err != nil {
	// 	s.logger.Error(ctx, err, "Failed to get account balance for canTrade check")
	// 	return false, "failed to get balance" // Fail safe if balance check fails
	// }
	// minBalance := s.cfg.Quantity * currentPrice / float64(s.cfg.Leverage) // Rough estimate
	// if balance < minBalance { // Add some buffer?
	// 	return false, fmt.Sprintf("insufficient balance (%.2f) for estimated cost (%.2f)", balance, minBalance)
	// }

	return true, "" // All checks passed
}

// formatPrice formats a float64 price into a string suitable for the Binance API.
// TODO: Determine the correct precision required by the Binance API for the specific symbol.
func formatPrice(price float64) string {
	// Example: Format to 2 decimal places. Adjust precision as needed.
	return strconv.FormatFloat(price, 'f', 2, 64)
}

// formatQuantity formats a float64 quantity into a string suitable for the Binance API.
// TODO: Determine the correct precision required by the Binance API for the specific symbol.
func formatQuantity(quantity float64) string {
	// Example: Format to 3 decimal places for ETH. Adjust precision as needed.
	return strconv.FormatFloat(quantity, 'f', 3, 64)
}

func (s *TradingService) enterPosition(ctx context.Context, entryPrice float64) error {
	op := "enterPosition"
	s.logger.Info(ctx, op+": Attempting to enter position", map[string]interface{}{"entryPrice": entryPrice})

	// --- Calculations ---
	// 1. Quantity (Fixed from config)
	quantity := s.cfg.Quantity
	quantityStr := formatQuantity(quantity)

	// 2. SL/TP Prices (Assuming LONG position based on strategy description)
	// Strategy: Enter on uptrend -> LONG only for now.
	// TODO: Add logic for SHORT positions if strategy requires it.
	side := domain.Buy // Correct constant
	slPrice := entryPrice * (1 - s.cfg.StopLoss)
	tpPrice := entryPrice * (1 + s.cfg.MaxProfit) // Using MaxProfit as per user feedback
	slPriceStr := formatPrice(slPrice)
	tpPriceStr := formatPrice(tpPrice)

	s.logger.Info(ctx, op+": Calculated parameters", map[string]interface{}{
		"side":       side,
		"quantity":   quantityStr,
		"stopLoss":   slPriceStr,
		"takeProfit": tpPriceStr,
	})

	// --- Order Placement ---
	var entryOrder, slOrder, tpOrder *ports.OrderResponse
	var err error

	// 3. Place entry market order
	s.logger.Info(ctx, op+": Placing entry market order...")
	entryOrder, err = s.exchange.PlaceMarketOrder(ctx, s.cfg.Symbol, side, quantityStr)
	if err != nil {
		s.logger.Error(ctx, err, op+": Failed to place entry market order")
		return fmt.Errorf("entry market order failed: %w", err)
	}
	// Use the actual filled price if available, otherwise fallback to kline price
	actualEntryPrice := entryOrder.AvgPrice
	if actualEntryPrice == 0 {
		s.logger.Warn(ctx, op+": Entry order AvgPrice is 0, using kline close price as fallback", map[string]interface{}{"orderID": entryOrder.OrderID, "fallbackPrice": entryPrice})
		actualEntryPrice = entryPrice
		// Recalculate SL/TP based on fallback price? Or stick with original? Sticking for now.
	} else {
		s.logger.Info(ctx, op+": Entry order filled", map[string]interface{}{"orderID": entryOrder.OrderID, "avgPrice": actualEntryPrice})
	}

	// 4. Place SL order (opposite side)
	slSide := domain.Sell // Correct constant, opposite of entry
	s.logger.Info(ctx, op+": Placing stop loss market order...")
	slOrder, err = s.exchange.PlaceStopMarketOrder(ctx, s.cfg.Symbol, slSide, quantityStr, slPriceStr)
	if err != nil {
		s.logger.Error(ctx, err, op+": Failed to place stop loss order")
		// Critical failure: We have an open position without a stop loss.
		// Attempt to close the position immediately as a safety measure.
		s.logger.Warn(ctx, op+": Attempting emergency close due to SL placement failure...")
		closeErr := s.emergencyClose(ctx, actualEntryPrice, quantityStr, side)
		if closeErr != nil {
			s.logger.Error(ctx, closeErr, op+": EMERGENCY CLOSE FAILED")
			// This is a very bad state. Manual intervention likely required.
			// Consider alerting mechanisms here.
		}
		return fmt.Errorf("stop loss order failed after entry: %w (emergency close attempted)", err)
	}
	s.logger.Info(ctx, op+": Stop loss order placed", map[string]interface{}{"orderID": slOrder.OrderID, "stopPrice": slPriceStr})

	// 5. Place TP order (opposite side)
	tpSide := domain.Sell // Correct constant, opposite of entry
	s.logger.Info(ctx, op+": Placing take profit market order...")
	tpOrder, err = s.exchange.PlaceTakeProfitMarketOrder(ctx, s.cfg.Symbol, tpSide, quantityStr, tpPriceStr)
	if err != nil {
		s.logger.Error(ctx, err, op+": Failed to place take profit order")
		// Less critical than SL failure, but still problematic.
		// Cancel the SL order and close the position.
		s.logger.Warn(ctx, op+": Attempting emergency close due to TP placement failure...")
		cancelErr := s.cancelOrderWarn(ctx, s.cfg.Symbol, slOrder.OrderID, "SL")
		if cancelErr != nil {
			// Log but proceed with close attempt
			s.logger.Error(ctx, cancelErr, op+": Failed to cancel SL order during TP failure cleanup")
		}
		closeErr := s.emergencyClose(ctx, actualEntryPrice, quantityStr, side)
		if closeErr != nil {
			s.logger.Error(ctx, closeErr, op+": EMERGENCY CLOSE FAILED after TP failure")
		}
		return fmt.Errorf("take profit order failed after entry: %w (emergency close attempted)", err)
	}
	s.logger.Info(ctx, op+": Take profit order placed", map[string]interface{}{"orderID": tpOrder.OrderID, "stopPrice": tpPriceStr})

	// --- Persistence and State Update ---
	// 6. Create domain.Position object
	newPosition := &domain.Position{
		Symbol:            s.cfg.Symbol,
		EntryPrice:        actualEntryPrice, // Use actual filled price
		Quantity:          quantity,
		Leverage:          s.cfg.Leverage,
		StopLoss:          slPrice,
		TakeProfit:        tpPrice,
		EntryTime:         time.Now().UTC(), // Use current time
		Status:            domain.StatusOpen,
		StopLossOrderID:   ptrToString(strconv.FormatInt(slOrder.OrderID, 10)), // Store order IDs
		TakeProfitOrderID: ptrToString(strconv.FormatInt(tpOrder.OrderID, 10)),
	}

	// 7. Save position via posRepo.Create
	posID, err := s.posRepo.Create(ctx, newPosition)
	if err != nil {
		s.logger.Error(ctx, err, op+": Failed to save new position to repository")
		// This is also problematic. We have orders placed but no DB record.
		// Attempt to cancel orders and close position.
		s.logger.Warn(ctx, op+": Attempting emergency close due to DB save failure...")
		cancelSlErr := s.cancelOrderWarn(ctx, s.cfg.Symbol, slOrder.OrderID, "SL")
		cancelTpErr := s.cancelOrderWarn(ctx, s.cfg.Symbol, tpOrder.OrderID, "TP")
		closeErr := s.emergencyClose(ctx, actualEntryPrice, quantityStr, side)
		// Log all errors
		if cancelSlErr != nil {
			s.logger.Error(ctx, cancelSlErr, op+": Failed to cancel SL order during DB failure cleanup")
		}
		if cancelTpErr != nil {
			s.logger.Error(ctx, cancelTpErr, op+": Failed to cancel TP order during DB failure cleanup")
		}
		if closeErr != nil {
			s.logger.Error(ctx, closeErr, op+": EMERGENCY CLOSE FAILED after DB failure")
		}
		return fmt.Errorf("failed to save position to DB after placing orders: %w (emergency close attempted)", err)
	}
	newPosition.ID = posID // Set the ID returned by the database
	s.logger.Info(ctx, op+": New position saved to DB", map[string]interface{}{"positionID": newPosition.ID})

	// 8. Update internal state
	s.currentPosition = newPosition
	s.tradesToday++
	s.logger.Info(ctx, op+": Internal state updated", map[string]interface{}{"tradesToday": s.tradesToday})

	return nil // Position successfully entered
}

func (s *TradingService) closePosition(ctx context.Context, exitPrice float64, reason domain.CloseReason) error {
	op := "closePosition"
	if s.currentPosition == nil {
		s.logger.Warn(ctx, op+": Attempted to close position, but no position is currently open")
		return fmt.Errorf("no open position to close")
	}

	positionToClose := s.currentPosition
	s.logger.Info(ctx, op+": Attempting to close position", map[string]interface{}{
		"positionID": positionToClose.ID,
		"exitPrice":  exitPrice,
		"reason":     reason,
	})

	// --- Order Placement and Cleanup ---
	// 1. Determine closing side (opposite of entry)
	// Assuming LONG entry for now
	closeSide := domain.Sell // Correct constant
	quantityStr := formatQuantity(positionToClose.Quantity)

	// 2. Place market order to close
	s.logger.Info(ctx, op+": Placing closing market order...")
	closeOrder, err := s.exchange.PlaceMarketOrder(ctx, s.cfg.Symbol, closeSide, quantityStr)
	if err != nil {
		s.logger.Error(ctx, err, op+": Failed to place closing market order", map[string]interface{}{"positionID": positionToClose.ID})
		// If closing fails, the position remains open. SL/TP orders should still be active.
		// Log the error and return. Manual intervention might be needed if it persists.
		return fmt.Errorf("failed to place closing market order for position %d: %w", positionToClose.ID, err)
	}
	actualExitPrice := closeOrder.AvgPrice
	if actualExitPrice == 0 {
		s.logger.Warn(ctx, op+": Close order AvgPrice is 0, using kline close price as fallback", map[string]interface{}{"orderID": closeOrder.OrderID, "fallbackPrice": exitPrice})
		actualExitPrice = exitPrice
	}
	s.logger.Info(ctx, op+": Closing market order placed successfully", map[string]interface{}{"orderID": closeOrder.OrderID, "avgPrice": actualExitPrice})

	// 3. Cancel existing SL/TP orders (Important!)
	// Use helper to log warnings instead of failing the whole close operation if cancellation fails
	if positionToClose.StopLossOrderID != nil {
		slOrderID, _ := strconv.ParseInt(*positionToClose.StopLossOrderID, 10, 64)
		_ = s.cancelOrderWarn(ctx, s.cfg.Symbol, slOrderID, "SL")
	}
	if positionToClose.TakeProfitOrderID != nil {
		tpOrderID, _ := strconv.ParseInt(*positionToClose.TakeProfitOrderID, 10, 64)
		_ = s.cancelOrderWarn(ctx, s.cfg.Symbol, tpOrderID, "TP")
	}

	// --- Persistence and State Update ---
	// 4. Calculate PNL
	// Simple PNL calculation (assuming LONG position)
	// TODO: Refine PNL calculation (consider fees, funding rates if applicable)
	pnl := (actualExitPrice - positionToClose.EntryPrice) * positionToClose.Quantity
	s.logger.Info(ctx, op+": Calculated PNL", map[string]interface{}{"positionID": positionToClose.ID, "pnl": pnl})

	// 5. Update domain.Position object
	positionToClose.ExitPrice = actualExitPrice
	positionToClose.ExitTime = time.Now().UTC()
	positionToClose.Status = domain.StatusClosed
	positionToClose.PNL = pnl
	positionToClose.CloseReason = reason

	// 6. Save updated position via posRepo.Update
	err = s.posRepo.Update(ctx, positionToClose)
	if err != nil {
		// Log error and return it since this is a critical operation
		s.logger.Error(ctx, err, op+": Failed to update closed position in repository", map[string]interface{}{"positionID": positionToClose.ID})
		return fmt.Errorf("failed to update closed position in repository: %w", err)
	}
	s.logger.Info(ctx, op+": Closed position updated in DB", map[string]interface{}{"positionID": positionToClose.ID})

	// 7. Update internal state
	s.currentPosition = nil
	s.logger.Info(ctx, op+": Position closed successfully, internal state updated", map[string]interface{}{"positionID": positionToClose.ID})

	return nil // Position successfully closed
}

// emergencyClose places a market order to close the current exposure.
// Assumes entrySide was the side used to open the position.
// Used when SL/TP placement fails after entry.
func (s *TradingService) emergencyClose(ctx context.Context, entryPrice float64, quantityStr string, entrySide domain.OrderSide) error {
	op := "emergencyClose"
	closeSide := domain.Sell      // Correct constant
	if entrySide == domain.Sell { // Correct constant
		closeSide = domain.Buy // Correct constant
	}
	s.logger.Warn(ctx, op+": Placing emergency closing order", map[string]interface{}{"side": closeSide, "quantity": quantityStr})
	_, err := s.exchange.PlaceMarketOrder(ctx, s.cfg.Symbol, closeSide, quantityStr)
	if err != nil {
		s.logger.Error(ctx, err, op+": FAILED TO PLACE EMERGENCY CLOSE ORDER")
		return fmt.Errorf("emergency close order placement failed: %w", err)
	}
	s.logger.Info(ctx, op+": Emergency close order placed successfully")
	// Note: This does not update DB state, as the position might not have been saved yet.
	// It's purely a safety mechanism on the exchange side.
	return nil
}

// cancelOrderWarn attempts to cancel an order and logs a warning on failure.
func (s *TradingService) cancelOrderWarn(ctx context.Context, symbol string, orderID int64, orderType string) error {
	op := "cancelOrderWarn"
	s.logger.Info(ctx, op+": Attempting to cancel order", map[string]interface{}{"symbol": symbol, "orderID": orderID, "type": orderType})
	_, err := s.exchange.CancelOrder(ctx, symbol, orderID)
	if err != nil {
		// Ignore "Order does not exist" errors, as it might have already been filled or cancelled.
		if errors.Is(err, ports.ErrOrderNotFound) {
			s.logger.Warn(ctx, op+": Order not found, likely already filled or cancelled", map[string]interface{}{"orderID": orderID, "type": orderType})
			return nil // Not an error in this context
		}
		s.logger.Error(ctx, err, op+": Failed to cancel order", map[string]interface{}{"orderID": orderID, "type": orderType})
		return err // Return other errors
	}
	s.logger.Info(ctx, op+": Order cancelled successfully", map[string]interface{}{"orderID": orderID, "type": orderType})
	return nil
}

// ptrToString converts a string to a pointer to a string.
func ptrToString(s string) *string {
	return &s
}
