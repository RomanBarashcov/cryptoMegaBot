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
	CloseReasonStopLoss       CloseReason = "SL"
	CloseReasonTakeProfit     CloseReason = "TP"
	CloseReasonMarket         CloseReason = "Market" // Manual or strategy-based market close
	CloseReasonLiquidation    CloseReason = "Liquidation"
	CloseReasonUnknown        CloseReason = "Unknown"
	CloseReasonManual         CloseReason = "MANUAL"
	CloseReasonTrendReversal  CloseReason = "TREND_REVERSAL"
	CloseReasonTimeLimit      CloseReason = "TIME_LIMIT"      // Position closed due to time-based exit rule
	CloseReasonVolatilityDrop CloseReason = "VOLATILITY_DROP" // Position closed due to volatility drop
	CloseReasonConsolidation  CloseReason = "CONSOLIDATION"   // Position closed due to price consolidation
	CloseReasonMarketClose    CloseReason = "MARKET_CLOSE"    // Position closed due to approaching market close
)
