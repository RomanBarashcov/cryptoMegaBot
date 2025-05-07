package binanceclient

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"cryptoMegaBot/internal/domain"
	"cryptoMegaBot/internal/ports"

	"github.com/adshao/go-binance/v2/common"
	"github.com/adshao/go-binance/v2/futures"
)

const (
	// Base URLs
	baseURLProduction = "https://fapi.binance.com"
	baseURLTestnet    = "https://testnet.binancefuture.com"
)

// Client implements the ports.ExchangeClient interface using the go-binance library.
type Client struct {
	futuresClient        *futures.Client
	logger               ports.Logger
	reconnectDelay       time.Duration
	maxReconnectAttempts int
}

// Config holds configuration specific to the Binance client adapter.
type Config struct {
	APIKey               string
	SecretKey            string
	UseTestnet           bool
	Logger               ports.Logger
	ReconnectDelay       time.Duration // Reconnect delay (e.g., 1 * time.Second)
	MaxReconnectAttempts int           // Max attempts before giving up
}

// New creates a new Binance client adapter.
func New(cfg Config) (*Client, error) {
	if cfg.Logger == nil {
		return nil, fmt.Errorf("logger is required for Binance client")
	}
	if cfg.APIKey == "" || cfg.SecretKey == "" {
		cfg.Logger.Warn(context.Background(), "APIKey or SecretKey is empty. Client will only work for public endpoints.")
		// Allow creation for public endpoints, but log warning.
		// Authentication errors will occur if private endpoints are called.
	}

	client := futures.NewClient(cfg.APIKey, cfg.SecretKey)

	// Set BaseURL directly instead of using global futures.UseTestnet
	if cfg.UseTestnet {
		client.BaseURL = baseURLTestnet
		cfg.Logger.Info(context.Background(), "Binance client configured for Testnet", map[string]interface{}{"baseURL": client.BaseURL})
	} else {
		client.BaseURL = baseURLProduction
		cfg.Logger.Info(context.Background(), "Binance client configured for Production", map[string]interface{}{"baseURL": client.BaseURL})
	}

	// Default reconnect settings if not provided
	reconnectDelay := cfg.ReconnectDelay
	if reconnectDelay <= 0 {
		reconnectDelay = 1 * time.Second
	}
	maxAttempts := cfg.MaxReconnectAttempts
	if maxAttempts <= 0 {
		maxAttempts = 10
	}

	return &Client{
		futuresClient:        client,
		logger:               cfg.Logger,
		reconnectDelay:       reconnectDelay,
		maxReconnectAttempts: maxAttempts,
	}, nil
}

// handleError translates common Binance API errors into standardized ports errors.
func (c *Client) handleError(ctx context.Context, err error, operation string) error {
	if err == nil {
		return nil
	}

	fields := map[string]interface{}{"operation": operation, "originalError": err.Error()}

	var apiErr *common.APIError
	if errors.As(err, &apiErr) {
		fields["apiErrorCode"] = apiErr.Code
		fields["apiErrorMessage"] = apiErr.Message

		// Map specific Binance error codes to custom errors
		var mappedErr error
		switch apiErr.Code {
		case -1003: // Too many requests
			mappedErr = ports.ErrRateLimited
		case -1021: // Timestamp for this request is outside of the recvWindow
			mappedErr = ports.ErrTimeout // Or a specific timing error
		case -1022: // Signature for this request is not valid
			mappedErr = ports.ErrAuthenticationFailed
		case -1101, -1102, -1103, -1104, -1105, -1106, -1111, -1115, -1116, -1117, -1120, -1121, -1125, -1127, -1128, -1130: // Parameter/Request format errors
			mappedErr = ports.ErrInvalidRequest
		case -2010: // New order rejected
			mappedErr = ports.ErrOrderPlacementFailed
		case -2011: // Cancel order rejected
			mappedErr = ports.ErrOrderCancelFailed
		case -2013: // Order does not exist
			mappedErr = ports.ErrOrderNotFound
		case -2014: // API-key format invalid
			mappedErr = ports.ErrInvalidAPIKeys
		case -2015: // Invalid API-key, IP, or permissions for action
			mappedErr = ports.ErrInvalidAPIKeys // Could also be PermissionDenied
		case -2019: // Margin is insufficient
			mappedErr = ports.ErrInsufficientFunds
		case -2022: // ReduceOnly Order is rejected
			mappedErr = ports.ErrOrderPlacementFailed // Or a more specific error
		case -3005: // Insufficient balance
			mappedErr = ports.ErrInsufficientFunds
		case -3041: // Position is not sufficient
			mappedErr = ports.ErrInsufficientFunds
		case -4003: // Qty not within permissible range
			mappedErr = ports.ErrInvalidRequest
		case -4014: // Price not within permissible range
			mappedErr = ports.ErrInvalidRequest
		case -4015: // Leverage is not valid
			mappedErr = ports.ErrInvalidRequest
		case -4044: // Position not found
			mappedErr = ports.ErrPositionNotFound
		case -4047: // Exceeded the maximum allowable position at current leverage.
			mappedErr = ports.ErrInsufficientFunds // Or a specific position limit error
		default:
			// General classification for unmapped API errors
			mappedErr = ports.ErrUnknown
		}
		finalErr := fmt.Errorf("%s failed: %w: %w", operation, mappedErr, err)
		c.logger.Error(ctx, err, fmt.Sprintf("%s failed with API error", operation), fields)
		return finalErr
	}

	// Handle non-API errors (network, context cancellation, etc.)
	var finalErr error
	if errors.Is(err, context.DeadlineExceeded) {
		finalErr = fmt.Errorf("%s failed: %w: %w", operation, ports.ErrTimeout, err)
	} else if errors.Is(err, context.Canceled) {
		finalErr = fmt.Errorf("%s operation canceled: %w: %w", operation, ports.ErrContextCanceled, err)
	} else if strings.Contains(err.Error(), "use of closed network connection") ||
		strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "connection reset by peer") {
		finalErr = fmt.Errorf("%s failed: %w: %w", operation, ports.ErrConnectionFailed, err)
	} else {
		// Default for other errors (e.g., parsing errors within the adapter)
		finalErr = fmt.Errorf("%s failed: %w: %w", operation, ports.ErrUnknown, err)
	}

	c.logger.Error(ctx, err, fmt.Sprintf("%s failed", operation), fields)
	return finalErr
}

// SetServerTime synchronizes the client's time with the server's time.
func (c *Client) SetServerTime(ctx context.Context) error {
	op := "SetServerTime"
	_, err := c.futuresClient.NewSetServerTimeService().Do(ctx)
	if err != nil {
		return c.handleError(ctx, err, op)
	}
	c.logger.Debug(ctx, op+" successful")
	return nil
}

// GetMarkPrice retrieves the current mark price for a given symbol.
func (c *Client) GetMarkPrice(ctx context.Context, symbol string) (float64, error) {
	op := "GetMarkPrice"
	tickers, err := c.futuresClient.NewPremiumIndexService().Symbol(symbol).Do(ctx)
	if err != nil {
		return 0, c.handleError(ctx, err, op)
	}
	if len(tickers) == 0 {
		err := fmt.Errorf("no price data returned for symbol %s", symbol)
		return 0, c.handleError(ctx, err, op) // Wrap with handleError for logging
	}

	price, err := strconv.ParseFloat(tickers[0].MarkPrice, 64)
	if err != nil {
		// This is an internal parsing error, not an API error
		parseErr := fmt.Errorf("could not parse price '%s': %w", tickers[0].MarkPrice, err)
		return 0, c.handleError(ctx, parseErr, op) // Wrap with handleError for logging
	}
	return price, nil
}

// GetTickerPrice retrieves the last ticker price for a given symbol.
func (c *Client) GetTickerPrice(ctx context.Context, symbol string) (float64, error) {
	op := "GetTickerPrice"
	tickers, err := c.futuresClient.NewListPriceChangeStatsService().Symbol(symbol).Do(ctx)
	if err != nil {
		return 0, c.handleError(ctx, err, op)
	}
	if len(tickers) == 0 {
		err := fmt.Errorf("no ticker data returned for symbol %s", symbol)
		return 0, c.handleError(ctx, err, op)
	}

	price, err := strconv.ParseFloat(tickers[0].LastPrice, 64)
	if err != nil {
		parseErr := fmt.Errorf("could not parse price '%s': %w", tickers[0].LastPrice, err)
		return 0, c.handleError(ctx, parseErr, op)
	}
	return price, nil
}

// GetAccountBalance retrieves the available balance for a specific asset (e.g., "USDT").
func (c *Client) GetAccountBalance(ctx context.Context, asset string) (float64, error) {
	op := "GetAccountBalance"
	account, err := c.futuresClient.NewGetAccountService().Do(ctx)
	if err != nil {
		return 0, c.handleError(ctx, err, op)
	}

	for _, bal := range account.Assets {
		if bal.Asset == asset {
			// Using WalletBalance, consider AvailableBalance if needed for trading decisions
			balance, err := strconv.ParseFloat(bal.WalletBalance, 64)
			if err != nil {
				parseErr := fmt.Errorf("could not parse balance '%s' for asset %s: %w", bal.WalletBalance, asset, err)
				return 0, c.handleError(ctx, parseErr, op)
			}
			return balance, nil
		}
	}

	// Asset not found in the account details
	err = fmt.Errorf("asset %s not found in account balance", asset)
	return 0, c.handleError(ctx, err, op) // Wrap with handleError for logging
}

// Ping checks the connectivity to the exchange API.
func (c *Client) Ping(ctx context.Context) error {
	op := "Ping"
	err := c.futuresClient.NewPingService().Do(ctx)
	if err != nil {
		// Ping failure likely indicates connection or availability issues
		return c.handleError(ctx, fmt.Errorf("ping failed: %w", err), op) // Wrap inner error
	}
	c.logger.Debug(ctx, op+" successful")
	return nil
}

// GetServerTime retrieves the current server time from the exchange.
func (c *Client) GetServerTime(ctx context.Context) (time.Time, error) {
	op := "GetServerTime"
	serverTimeMs, err := c.futuresClient.NewServerTimeService().Do(ctx)
	if err != nil {
		return time.Time{}, c.handleError(ctx, err, op)
	}
	// Convert milliseconds to time.Time
	return time.UnixMilli(serverTimeMs), nil
}

// SetLeverage sets the leverage for a specific symbol.
func (c *Client) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	op := "SetLeverage"
	_, err := c.futuresClient.NewChangeLeverageService().
		Symbol(symbol).
		Leverage(leverage).
		Do(ctx)
	if err != nil {
		return c.handleError(ctx, err, op)
	}
	c.logger.Info(ctx, op+" successful", map[string]interface{}{"symbol": symbol, "leverage": leverage})
	return nil
}

// PlaceMarketOrder places a market order.
func (c *Client) PlaceMarketOrder(ctx context.Context, symbol string, side domain.OrderSide, quantity string) (*ports.OrderResponse, error) {
	op := "PlaceMarketOrder"
	binanceSide := futures.SideType(side) // Direct conversion assuming values match

	order, err := c.futuresClient.NewCreateOrderService().
		Symbol(symbol).
		Side(binanceSide).
		Type(futures.OrderTypeMarket).
		Quantity(quantity).
		Do(ctx)
	if err != nil {
		return nil, c.handleError(ctx, err, op)
	}

	resp := translateOrderResponse(order)
	c.logger.Info(ctx, op+" successful", map[string]interface{}{"symbol": symbol, "side": side, "quantity": quantity, "orderID": resp.OrderID, "avgPrice": resp.AvgPrice})
	return resp, nil
}

// PlaceStopMarketOrder places a stop-market order.
func (c *Client) PlaceStopMarketOrder(ctx context.Context, symbol string, side domain.OrderSide, quantity string, stopPrice string) (*ports.OrderResponse, error) {
	op := "PlaceStopMarketOrder"
	binanceSide := futures.SideType(side)

	order, err := c.futuresClient.NewCreateOrderService().
		Symbol(symbol).
		Side(binanceSide).
		Type(futures.OrderTypeStopMarket).
		Quantity(quantity).
		StopPrice(stopPrice).
		ClosePosition(true). // Ensure it closes position, adjust if needed for SL/TP logic
		Do(ctx)
	if err != nil {
		return nil, c.handleError(ctx, err, op)
	}

	resp := translateOrderResponse(order)
	c.logger.Info(ctx, op+" successful", map[string]interface{}{"symbol": symbol, "side": side, "quantity": quantity, "stopPrice": stopPrice, "orderID": resp.OrderID})
	return resp, nil
}

// PlaceTakeProfitMarketOrder places a take-profit-market order.
func (c *Client) PlaceTakeProfitMarketOrder(ctx context.Context, symbol string, side domain.OrderSide, quantity string, stopPrice string) (*ports.OrderResponse, error) {
	op := "PlaceTakeProfitMarketOrder"
	binanceSide := futures.SideType(side)

	// Add detailed logging before order placement
	c.logger.Info(ctx, op+": Attempting to place take profit order", map[string]interface{}{
		"symbol":    symbol,
		"side":      side,
		"quantity":  quantity,
		"stopPrice": stopPrice,
		"type":      "TAKE_PROFIT_MARKET",
	})

	order, err := c.futuresClient.NewCreateOrderService().
		Symbol(symbol).
		Side(binanceSide).
		Type(futures.OrderTypeTakeProfitMarket).
		Quantity(quantity).
		StopPrice(stopPrice).
		ClosePosition(true).
		Do(ctx)
	if err != nil {
		// Enhanced error logging
		c.logger.Error(ctx, err, op+": Failed to place take profit order", map[string]interface{}{
			"symbol":    symbol,
			"side":      side,
			"quantity":  quantity,
			"stopPrice": stopPrice,
		})
		return nil, c.handleError(ctx, err, op)
	}

	resp := translateOrderResponse(order)
	c.logger.Info(ctx, op+" successful", map[string]interface{}{
		"symbol":    symbol,
		"side":      side,
		"quantity":  quantity,
		"stopPrice": stopPrice,
		"orderID":   resp.OrderID,
		"status":    resp.Status,
	})
	return resp, nil
}

// GetPositionRisk retrieves the risk information for a specific position symbol.
func (c *Client) GetPositionRisk(ctx context.Context, symbol string) (*ports.PositionRisk, error) {
	op := "GetPositionRisk"
	positions, err := c.futuresClient.NewGetPositionRiskService().Symbol(symbol).Do(ctx)
	if err != nil {
		return nil, c.handleError(ctx, err, op)
	}
	if len(positions) == 0 {
		c.logger.Debug(ctx, op+": No position found for symbol", map[string]interface{}{"symbol": symbol})
		return nil, nil // It's valid not to have a position
	}

	// Assuming only one position per symbol for futures
	binancePos := positions[0]
	qty, _ := strconv.ParseFloat(binancePos.PositionAmt, 64) // Ignore error, default to 0
	if qty == 0 {
		c.logger.Debug(ctx, op+": Position amount is zero for symbol", map[string]interface{}{"symbol": symbol})
		return nil, nil // Position amount is zero, effectively no position
	}

	resp := translatePositionRisk(binancePos)
	return resp, nil
}

// StreamKlines starts a WebSocket stream for K-line/candlestick data.
func (c *Client) StreamKlines(ctx context.Context, symbol, interval string, handler func(kline *domain.Kline), errHandler func(err error)) (doneCh chan struct{}, stopCh chan struct{}, err error) {
	op := "StreamKlines"
	wsCtx, cancelWs := context.WithCancel(ctx) // Create a cancellable context for the WS lifecycle

	// Wrapper for the domain handler to perform translation
	binanceHandler := func(event *futures.WsKlineEvent) {
		domainKline, err := translateWsKline(event)
		if err != nil {
			c.logger.Error(wsCtx, err, op+": Failed to translate WebSocket kline event")
			// Decide if we should call the errHandler or just log
			// Calling errHandler might trigger reconnection logic unnecessarily for a translation error
			return
		}
		handler(domainKline)
	}

	// Wrapper for the error handler to perform translation and logging
	binanceErrHandler := func(err error) {
		// Use background context for logging if wsCtx is done? Or maybe wsCtx is fine.
		translatedErr := c.handleError(wsCtx, err, op+" WebSocket")
		c.logger.Warn(wsCtx, op+": WebSocket error reported", map[string]interface{}{"error": translatedErr})
		errHandler(translatedErr) // Pass the translated error up
	}

	// Reconnection loop
	go func() {
		defer cancelWs() // Ensure context is cancelled when this goroutine exits

		attempt := 0
		for {
			select {
			case <-wsCtx.Done():
				c.logger.Info(wsCtx, op+": Context cancelled, stopping connection attempts.", map[string]interface{}{"symbol": symbol, "interval": interval})
				return // Exit loop if context is cancelled
			default:
				// Attempt connection
				c.logger.Info(wsCtx, op+": Attempting WebSocket connection...", map[string]interface{}{"symbol": symbol, "interval": interval, "attempt": attempt + 1})
				innerDoneCh, innerStopCh, connectErr := futures.WsKlineServe(symbol, interval, binanceHandler, binanceErrHandler)

				if connectErr != nil {
					c.handleError(wsCtx, connectErr, op+" connection attempt") // Log the connection error
					attempt++
					if attempt >= c.maxReconnectAttempts {
						c.logger.Error(wsCtx, connectErr, op+": Max reconnection attempts exceeded, giving up.", map[string]interface{}{"symbol": symbol, "interval": interval, "maxAttempts": c.maxReconnectAttempts})
						// Signal failure? How to signal this back? Maybe close doneCh?
						// For now, just exit the goroutine. The caller might rely on context cancellation.
						return
					}

					// Calculate delay with exponential backoff and jitter
					delay := c.reconnectDelay * time.Duration(1<<uint(attempt-1))
					jitter := time.Duration(float64(delay) * 0.1 * float64(time.Millisecond)) // 10% jitter
					actualDelay := delay + jitter
					c.logger.Info(wsCtx, op+": Connection failed, retrying...", map[string]interface{}{"symbol": symbol, "interval": interval, "attempt": attempt + 1, "delay": actualDelay.String()})

					select {
					case <-time.After(actualDelay):
						continue // Retry connection
					case <-wsCtx.Done():
						c.logger.Info(wsCtx, op+": Context cancelled during backoff.", map[string]interface{}{"symbol": symbol, "interval": interval})
						return // Exit if context cancelled during wait
					}
				}

				// Connection successful
				c.logger.Info(wsCtx, op+": WebSocket connection established.", map[string]interface{}{"symbol": symbol, "interval": interval})
				attempt = 0 // Reset attempt count on successful connection

				// Wait for the inner connection to close or the context to be cancelled
				select {
				case <-innerDoneCh:
					c.logger.Warn(wsCtx, op+": WebSocket connection closed unexpectedly. Reconnecting...", map[string]interface{}{"symbol": symbol, "interval": interval})
					// Loop will continue and attempt reconnection
				case <-wsCtx.Done():
					c.logger.Info(wsCtx, op+": Context cancelled, stopping WebSocket.", map[string]interface{}{"symbol": symbol, "interval": interval})
					// Send stop signal to inner WebSocket
					select {
					case innerStopCh <- struct{}{}:
						c.logger.Debug(wsCtx, op+": Stop signal sent to inner WebSocket.", map[string]interface{}{"symbol": symbol, "interval": interval})
					default:
						c.logger.Warn(wsCtx, op+": Failed to send stop signal to inner WebSocket (already closed?).", map[string]interface{}{"symbol": symbol, "interval": interval})
					}
					// Wait for innerDoneCh to confirm closure? Might not be necessary.
					return // Exit goroutine
				}
			}
		}
	}()

	// Return channels linked to the lifecycle of the reconnection goroutine
	// doneCh signals when the reconnection loop exits (either success or max attempts)
	// stopCh allows the caller to cancel the reconnection loop via wsCtx
	doneCh = make(chan struct{})
	stopCh = make(chan struct{}) // This stopCh controls the outer loop via context cancellation

	// Goroutine to link the external stopCh to the internal context cancellation
	go func() {
		select {
		case <-stopCh:
			c.logger.Info(ctx, op+": Received external stop signal, cancelling WebSocket context.", map[string]interface{}{"symbol": symbol, "interval": interval})
			cancelWs()
		case <-wsCtx.Done():
			// If wsCtx is cancelled internally or by the parent context, just exit
			c.logger.Debug(ctx, op+": WebSocket context done, stop listener exiting.", map[string]interface{}{"symbol": symbol, "interval": interval})
		}
	}()

	// Goroutine to close the external doneCh when the internal context is done
	go func() {
		<-wsCtx.Done()
		c.logger.Info(ctx, op+": WebSocket context done, closing external done channel.", map[string]interface{}{"symbol": symbol, "interval": interval})
		close(doneCh)
	}()

	return doneCh, stopCh, nil
}

// GetKlines retrieves historical klines/candlestick data for the given symbol.
func (c *Client) GetKlines(ctx context.Context, symbol string, interval string, limit int) ([]*domain.Kline, error) {
	op := "GetKlines"
	binanceKlines, err := c.futuresClient.NewKlinesService().Symbol(symbol).Interval(interval).Limit(limit).Do(ctx)
	if err != nil {
		return nil, c.handleError(ctx, err, op)
	}

	domainKlines := make([]*domain.Kline, 0, len(binanceKlines))
	for _, bk := range binanceKlines {
		dk, err := translateBinanceKline(bk, symbol, interval)
		if err != nil {
			// Log the translation error but potentially continue with other klines?
			// Or return the error immediately? Returning seems safer.
			return nil, c.handleError(ctx, fmt.Errorf("failed to translate historical kline: %w", err), op)
		}
		domainKlines = append(domainKlines, dk)
	}

	return domainKlines, nil
}

// GetKlinesRange fetches all klines for a symbol/interval between start and end time.
func (c *Client) GetKlinesRange(ctx context.Context, symbol, interval string, start, end time.Time) ([]*domain.Kline, error) {
	op := "GetKlinesRange"
	var allKlines []*domain.Kline
	const maxLimit = 1500
	from := start

	for {
		klines, err := c.futuresClient.NewKlinesService().
			Symbol(symbol).
			Interval(interval).
			StartTime(from.UnixMilli()).
			EndTime(end.UnixMilli()).
			Limit(maxLimit).
			Do(ctx)
		if err != nil {
			return nil, c.handleError(ctx, err, op)
		}
		if len(klines) == 0 {
			break
		}
		for _, bk := range klines {
			dk, err := translateBinanceKline(bk, symbol, interval)
			if err != nil {
				return nil, c.handleError(ctx, fmt.Errorf("failed to translate historical kline range: %w", err), op)
			}
			allKlines = append(allKlines, dk)
		}
		last := klines[len(klines)-1]
		from = time.UnixMilli(last.CloseTime)
		if from.After(end) || len(klines) < maxLimit {
			break
		}
	}

	return allKlines, nil
}

// CancelOrder cancels an open order on Binance.
func (c *Client) CancelOrder(ctx context.Context, symbol string, orderID int64) (*ports.OrderResponse, error) {
	op := "CancelOrder"
	c.logger.Debug(ctx, "Attempting to cancel order", map[string]interface{}{"symbol": symbol, "orderID": orderID})

	res, err := c.futuresClient.NewCancelOrderService().
		Symbol(symbol).
		OrderID(orderID).
		Do(ctx)
	if err != nil {
		// Handle specific error for "Order does not exist" if needed,
		// but handleError should map -2013 to ErrOrderNotFound.
		// Handle specific error for "Order does not exist" if needed,
		// but handleError should map -2013 to ErrOrderNotFound.
		// Handle specific error for "Order does not exist" if needed,
		// but handleError should map -2013 to ErrOrderNotFound.
		return nil, c.handleError(ctx, err, op)
	}

	// Manually create a CreateOrderResponse from CancelOrderResponse fields
	// as direct casting is not allowed.
	createOrderResp := &futures.CreateOrderResponse{
		OrderID:       res.OrderID,
		Symbol:        res.Symbol,
		ClientOrderID: res.ClientOrderID,
		Price:         res.Price,
		OrigQuantity:  res.OrigQuantity,
		Status:        res.Status, // Should be CANCELED
		TimeInForce:   res.TimeInForce,
		Type:          res.Type,
		Side:          res.Side,
		// Fields like AvgPrice, ExecutedQuantity, UpdateTime are not in CancelOrderResponse
		// They will default to zero values in createOrderResp, which is fine.
	}

	resp := translateOrderResponse(createOrderResp)
	c.logger.Info(ctx, op+" successful", map[string]interface{}{"symbol": symbol, "orderID": orderID, "status": resp.Status})
	return resp, nil
}

// --- Translation Helpers ---

func translateOrderResponse(order *futures.CreateOrderResponse) *ports.OrderResponse {
	if order == nil {
		return nil
	}
	price, _ := strconv.ParseFloat(order.Price, 64)
	avgPrice, _ := strconv.ParseFloat(order.AvgPrice, 64)
	origQty, _ := strconv.ParseFloat(order.OrigQuantity, 64)
	execQty, _ := strconv.ParseFloat(order.ExecutedQuantity, 64)

	return &ports.OrderResponse{
		OrderID:       order.OrderID,
		Symbol:        order.Symbol,
		ClientOrderID: order.ClientOrderID,
		Price:         price,
		AvgPrice:      avgPrice,
		OrigQuantity:  origQty,
		ExecutedQty:   execQty,
		Status:        string(order.Status),
		TimeInForce:   string(order.TimeInForce),
		Type:          string(order.Type),
		Side:          string(order.Side),
		Timestamp:     time.UnixMilli(order.UpdateTime), // Assuming UpdateTime is relevant timestamp
	}
}

func translatePositionRisk(pos *futures.PositionRisk) *ports.PositionRisk {
	if pos == nil {
		return nil
	}
	posAmt, _ := strconv.ParseFloat(pos.PositionAmt, 64)
	entryPrice, _ := strconv.ParseFloat(pos.EntryPrice, 64)
	markPrice, _ := strconv.ParseFloat(pos.MarkPrice, 64)
	unProfit, _ := strconv.ParseFloat(pos.UnRealizedProfit, 64)
	liqPrice, _ := strconv.ParseFloat(pos.LiquidationPrice, 64)
	leverage, _ := strconv.Atoi(pos.Leverage) // Leverage is string in go-binance
	isoMargin, _ := strconv.ParseFloat(pos.IsolatedMargin, 64)
	maxNotional, _ := strconv.ParseFloat(pos.MaxNotionalValue, 64)
	isAutoAdd, _ := strconv.ParseBool(pos.IsAutoAddMargin)

	return &ports.PositionRisk{
		Symbol:           pos.Symbol,
		PositionAmt:      posAmt,
		EntryPrice:       entryPrice,
		MarkPrice:        markPrice,
		UnRealizedProfit: unProfit,
		LiquidationPrice: liqPrice,
		Leverage:         leverage,
		IsolatedMargin:   isoMargin,
		IsAutoAddMargin:  isAutoAdd,
		MaxNotionalValue: maxNotional,
		// UpdateTime: time.UnixMilli(pos.UpdateTime), // Removed as field doesn't exist in source
	}
}

func translateWsKline(event *futures.WsKlineEvent) (*domain.Kline, error) {
	if event == nil {
		return nil, errors.New("received nil kline event")
	}
	k := event.Kline
	open, err := strconv.ParseFloat(k.Open, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing open price '%s': %w", k.Open, err)
	}
	high, err := strconv.ParseFloat(k.High, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing high price '%s': %w", k.High, err)
	}
	low, err := strconv.ParseFloat(k.Low, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing low price '%s': %w", k.Low, err)
	}
	cls, err := strconv.ParseFloat(k.Close, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing close price '%s': %w", k.Close, err)
	}
	vol, err := strconv.ParseFloat(k.Volume, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing volume '%s': %w", k.Volume, err)
	}

	return &domain.Kline{
		OpenTime:  time.UnixMilli(k.StartTime),
		CloseTime: time.UnixMilli(k.EndTime),
		Symbol:    k.Symbol,
		Interval:  k.Interval,
		Open:      open,
		High:      high,
		Low:       low,
		Close:     cls,
		Volume:    vol,
		IsFinal:   k.IsFinal,
	}, nil
}

func translateBinanceKline(bk *futures.Kline, symbol, interval string) (*domain.Kline, error) {
	if bk == nil {
		return nil, errors.New("received nil historical kline")
	}
	open, err := strconv.ParseFloat(bk.Open, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing open price '%s': %w", bk.Open, err)
	}
	high, err := strconv.ParseFloat(bk.High, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing high price '%s': %w", bk.High, err)
	}
	low, err := strconv.ParseFloat(bk.Low, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing low price '%s': %w", bk.Low, err)
	}
	cls, err := strconv.ParseFloat(bk.Close, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing close price '%s': %w", bk.Close, err)
	}
	vol, err := strconv.ParseFloat(bk.Volume, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing volume '%s': %w", bk.Volume, err)
	}

	return &domain.Kline{
		OpenTime:  time.UnixMilli(bk.OpenTime),
		CloseTime: time.UnixMilli(bk.CloseTime),
		Symbol:    symbol,   // Use passed symbol as it's not in futures.Kline
		Interval:  interval, // Use passed interval
		Open:      open,
		High:      high,
		Low:       low,
		Close:     cls,
		Volume:    vol,
		IsFinal:   true, // Historical klines are always final
	}, nil
}
