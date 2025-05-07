package domain

import "time"

// Position represents a trading position held by the bot.
type Position struct {
	ID         int64          // Unique identifier for the position (usually from DB)
	Symbol     string         // Trading symbol (e.g., "ETHUSDT")
	EntryPrice float64        // Price at which the position was entered
	ExitPrice  float64        // Price at which the position was exited (0 if open)
	Quantity   float64        // Size of the position
	Leverage   int            // Leverage used for the position
	StopLoss   float64        // Price level for stop-loss order
	TakeProfit float64        // Price level for take-profit order
	EntryTime  time.Time      // Timestamp when the position was entered
	ExitTime   time.Time      // Timestamp when the position was exited (zero value if open)
	Status     PositionStatus // Current status (open, closed)
	PNL        float64        // Profit and Loss for the position (calculated on close)

	// Associated order IDs for SL/TP management (nullable in DB)
	StopLossOrderID   *string     `db:"stop_loss_order_id"`
	TakeProfitOrderID *string     `db:"take_profit_order_id"`
	CloseReason       CloseReason `db:"close_reason"` // Reason for closing (SL, TP, Manual, etc.)

	// Trailing stop parameters
	TrailingStopDistance float64 `db:"trailing_stop_distance"` // Distance for trailing stop in price units
	TrailingStopPrice    float64 `db:"trailing_stop_price"`    // Current trailing stop price level
}

// IsOpen checks if the position status is open.
func (p *Position) IsOpen() bool {
	return p.Status == StatusOpen
}
