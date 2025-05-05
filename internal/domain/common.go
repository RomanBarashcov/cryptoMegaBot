package domain

// OrderSide represents the side of an order (BUY or SELL).
type OrderSide string

const (
	Buy  OrderSide = "BUY"
	Sell OrderSide = "SELL"
)

// PositionStatus represents the status of a trading position.
type PositionStatus string

const (
	StatusOpen   PositionStatus = "open"
	StatusClosed PositionStatus = "closed"
)

// CloseReason indicates why a position was closed.
type CloseReason string

const (
	CloseReasonStopLoss    CloseReason = "SL"
	CloseReasonTakeProfit  CloseReason = "TP"
	CloseReasonMarket      CloseReason = "Market" // Manual or strategy-based market close
	CloseReasonLiquidation CloseReason = "Liquidation"
	CloseReasonUnknown     CloseReason = "Unknown"
)
