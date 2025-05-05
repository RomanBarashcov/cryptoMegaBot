package ports

import (
	"context"
	"time"

	"cryptoMegaBot/internal/domain"
)

// OrderResponse represents the essential details returned after placing an order.
type OrderResponse struct {
	OrderID       int64     // Exchange's order ID
	Symbol        string    // Symbol for the order
	ClientOrderID string    // User-defined order ID
	Price         float64   // Price of the order (might be 0 for market orders initially)
	AvgPrice      float64   // Average filled price
	OrigQuantity  float64   // Original quantity requested
	ExecutedQty   float64   // Quantity filled
	Status        string    // Order status (e.g., NEW, FILLED, CANCELED)
	TimeInForce   string    // Time in force (e.g., GTC, IOC, FOK)
	Type          string    // Order type (e.g., MARKET, LIMIT, STOP_MARKET)
	Side          string    // Order side (BUY, SELL)
	Timestamp     time.Time // Time the order response was generated
}

// PositionRisk represents the risk details for an open position.
type PositionRisk struct {
	Symbol           string  // Symbol of the position
	PositionAmt      float64 // Current position amount (positive for long, negative for short)
	EntryPrice       float64 // Average entry price of the position
	MarkPrice        float64 // Current mark price
	UnRealizedProfit float64 // Unrealized profit/loss
	LiquidationPrice float64 // Estimated liquidation price
	Leverage         int     // Current leverage for the position
	IsolatedMargin   float64 // Isolated margin (if applicable)
	IsAutoAddMargin  bool    // Whether auto margin add is enabled
	MaxNotionalValue float64 // Maximum notional value allowed
	// UpdateTime       time.Time // No direct UpdateTime field in futures.PositionRisk
}

// ExchangeClient defines the interface for interacting with a cryptocurrency exchange.
// This abstraction allows decoupling the core bot logic from specific exchange implementations.
type ExchangeClient interface {
	// SetServerTime synchronizes the client's time with the server's time.
	SetServerTime(ctx context.Context) error

	// GetMarkPrice retrieves the current mark price for a given symbol.
	GetMarkPrice(ctx context.Context, symbol string) (float64, error)

	// GetTickerPrice retrieves the last ticker price for a given symbol.
	GetTickerPrice(ctx context.Context, symbol string) (float64, error)

	// GetAccountBalance retrieves the available balance for a specific asset (e.g., "USDT").
	GetAccountBalance(ctx context.Context, asset string) (float64, error)

	// SetLeverage sets the leverage for a specific symbol.
	SetLeverage(ctx context.Context, symbol string, leverage int) error

	// PlaceMarketOrder places a market order.
	// Returns the essential order details upon successful execution.
	PlaceMarketOrder(ctx context.Context, symbol string, side domain.OrderSide, quantity string) (*OrderResponse, error)

	// PlaceStopMarketOrder places a stop-market order.
	// Returns the essential order details upon successful placement.
	PlaceStopMarketOrder(ctx context.Context, symbol string, side domain.OrderSide, quantity string, stopPrice string) (*OrderResponse, error)

	// PlaceTakeProfitMarketOrder places a take-profit-market order.
	// Returns the essential order details upon successful placement.
	PlaceTakeProfitMarketOrder(ctx context.Context, symbol string, side domain.OrderSide, quantity string, stopPrice string) (*OrderResponse, error)

	// GetPositionRisk retrieves the risk information for a specific position symbol.
	// Returns nil if no position exists for the symbol.
	GetPositionRisk(ctx context.Context, symbol string) (*PositionRisk, error)

	// StreamKlines starts a WebSocket stream for K-line/candlestick data.
	// It takes handlers for processing domain.Kline events and errors.
	// Returns channels to control the stream (doneCh, stopCh) or an error if connection fails.
	StreamKlines(ctx context.Context, symbol, interval string, handler func(kline *domain.Kline), errHandler func(err error)) (doneCh chan struct{}, stopCh chan struct{}, err error)

	// Ping checks the connectivity to the exchange API.
	Ping(ctx context.Context) error

	// GetServerTime retrieves the current server time from the exchange.
	GetServerTime(ctx context.Context) (time.Time, error)

	// GetKlines retrieves historical klines/candlestick data for the given symbol.
	GetKlines(ctx context.Context, symbol string, interval string, limit int) ([]*domain.Kline, error)

	// CancelOrder cancels an existing open order by its ID.
	CancelOrder(ctx context.Context, symbol string, orderID int64) (*OrderResponse, error) // Returns details of the cancelled order
}
