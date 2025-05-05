package domain

import "time"

// Trade represents a completed trade event.
type Trade struct {
	ID          int64       // Unique identifier for the trade (usually from DB)
	PositionID  int64       // Identifier of the position this trade closed (optional)
	Symbol      string      // Trading symbol (e.g., "ETHUSDT")
	EntryPrice  float64     // Price at which the position was entered
	ExitPrice   float64     // Price at which the position was exited
	Quantity    float64     // Size of the position traded
	Leverage    int         // Leverage used for the position
	PNL         float64     // Profit and Loss for this trade
	EntryTime   time.Time   // Timestamp when the position was entered
	ExitTime    time.Time   // Timestamp when the position was exited
	CloseReason CloseReason // Reason why the position was closed (SL, TP, etc.)
}
