package domain

import "time"

// Kline represents a single candlestick data point.
type Kline struct {
	OpenTime  time.Time // Start time of the interval
	CloseTime time.Time // End time of the interval
	Symbol    string    // Trading symbol
	Interval  string    // Kline interval (e.g., "1m", "1h")
	Open      float64   // Opening price
	High      float64   // Highest price
	Low       float64   // Lowest price
	Close     float64   // Closing price
	Volume    float64   // Trading volume
	IsFinal   bool      // Whether this kline is the final one for the interval
}
