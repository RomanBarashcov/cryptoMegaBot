package strategies

import (
	"context"
	"cryptoMegaBot/internal/domain"
	"cryptoMegaBot/internal/ports"
	"cryptoMegaBot/internal/strategy/indicators"
	"fmt"
	"math"
	"time"
)

// MACrossoverConfig holds configuration for the Improved MA Crossover strategy
type MACrossoverConfig struct {
	// Core parameters - reduced for less complexity
	FastMAPeriod  int     // Fast MA period (e.g., 8)
	SlowMAPeriod  int     // Slow MA period (e.g., 21)
	SignalPeriod  int     // Signal line period for trend confirmation (e.g., 9)
	ATRPeriod     int     // ATR period for volatility measurement (e.g., 14)
	ATRMultiplier float64 // Multiplier for ATR-based stops (e.g., 2.5)

	// Multi-timeframe parameters
	UseMultiTimeframe bool   // Whether to use multi-timeframe analysis
	PrimaryTimeframe  string // Primary timeframe for trading decisions (e.g., "15m")
	TrendTimeframe    string // Higher timeframe for trend confirmation (e.g., "1h")

	// Scalping parameters for more frequent trading
	UseScalpTimeframe bool   // Whether to use scalping timeframe for more entries
	ScalpTimeframe    string // Shorter timeframe for scalping opportunities (e.g., "5m")
	ScalpFastPeriod   int    // Fast MA period for scalping (e.g., 5)
	ScalpSlowPeriod   int    // Slow MA period for scalping (e.g., 13)

	// Day trading parameters
	MaxDailyLosses         int           // Maximum number of losing trades per day before stopping
	MaxConsecutiveLosses   int           // Maximum number of consecutive losses before reducing size
	MaxHoldingTime         time.Duration // Maximum time to hold a position (e.g., 4h for day trading)
	PartialProfitPct       float64       // Percentage at which to take partial profits (e.g., 0.01 for 1%)
	TrailingActivePct      float64       // Percentage at which to activate trailing stop (e.g., 0.003 for 0.3%)
	BreakEvenActivation    float64       // Percentage at which to move stop loss to breakeven (e.g., 0.002 for 0.2%)
	TrailingStopTightening bool          // Whether to progressively tighten trailing stop as profit increases

	// Risk management parameters
	InitialRiskPerTrade       float64 // Initial risk per trade as percentage of account (e.g., 0.005 for 0.5%)
	DynamicLeverageAdjustment bool    // Whether to dynamically adjust leverage based on market conditions

	// Market hours parameters
	TradingHoursOnly bool    // Whether to only trade during specific hours
	TradingStartHour int     // Hour to start trading (e.g., 8 for 8:00 AM)
	TradingEndHour   int     // Hour to end trading (e.g., 20 for 8:00 PM)
	MaxLeverageUsed  float64 // Maximum leverage to use (e.g., 4.0 for 4x)
}

// MACrossover implements an improved Moving Average Crossover strategy
// with better entry criteria, risk management, and exit strategies
type MACrossover struct {
	*BaseStrategy
	config     MACrossoverConfig
	fastMA     *indicators.MovingAverage
	slowMA     *indicators.MovingAverage
	signalLine *indicators.MovingAverage
	atr        *indicators.ATR
	rsi        *indicators.RSI

	// Multi-timeframe indicators
	trendFastMA *indicators.MovingAverage
	trendSlowMA *indicators.MovingAverage

	// Scalping timeframe indicators
	scalpFastMA *indicators.MovingAverage
	scalpSlowMA *indicators.MovingAverage

	// Trading state
	dailyLossCount    int
	consecutiveLosses int
	lastLossResetDay  time.Time
	partialTakeProfit bool
	lastTradeResult   float64

	// Volatility tracking
	recentVolatility []float64

	// Performance tracking
	winCount              int
	lossCount             int
	totalPnL              float64
	lastTradeTime         time.Time
	consolidationDetected bool
}

// NewImprovedMACrossover creates a new Improved MA Crossover strategy instance
func NewImprovedMACrossover(config MACrossoverConfig, logger ports.Logger) (*MACrossover, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is required for strategy")
	}

	// Validate configuration
	if config.FastMAPeriod <= 0 || config.SlowMAPeriod <= 0 || config.SignalPeriod <= 0 || config.ATRPeriod <= 0 {
		return nil, fmt.Errorf("strategy periods must be positive")
	}
	if config.FastMAPeriod >= config.SlowMAPeriod {
		return nil, fmt.Errorf("fast MA period must be less than slow MA period")
	}
	if config.ATRMultiplier <= 0 {
		return nil, fmt.Errorf("ATR multiplier must be positive")
	}

	// Set defaults for new parameters if not specified
	if config.MaxHoldingTime == 0 {
		config.MaxHoldingTime = 4 * time.Hour // Default to 4 hours for day trading
	}
	if config.PartialProfitPct == 0 {
		config.PartialProfitPct = 0.005 // Default to 0.5% for partial profit taking (reduced from 1%)
	}
	if config.TrailingActivePct == 0 {
		config.TrailingActivePct = 0.002 // Default to 0.2% for trailing stop activation (reduced from 0.3%)
	}
	if config.MaxDailyLosses == 0 {
		config.MaxDailyLosses = 2 // Default to 2 losses per day
	}
	if config.MaxConsecutiveLosses == 0 {
		config.MaxConsecutiveLosses = 2 // Default to 2 consecutive losses
	}
	if config.MaxLeverageUsed == 0 {
		config.MaxLeverageUsed = 4.0 // Default to 4x max leverage
	}
	if config.InitialRiskPerTrade == 0 {
		config.InitialRiskPerTrade = 0.005 // Default to 0.5% risk per trade
	}
	if config.BreakEvenActivation == 0 {
		config.BreakEvenActivation = 0.002 // Default to 0.2% for breakeven activation
	}
	if config.ScalpFastPeriod == 0 {
		config.ScalpFastPeriod = 5 // Default to 5 periods for scalping fast MA
	}
	if config.ScalpSlowPeriod == 0 {
		config.ScalpSlowPeriod = 13 // Default to 13 periods for scalping slow MA
	}

	// Create indicators with simplified configuration
	fastMA := indicators.NewMovingAverage(indicators.MovingAverageConfig{
		IndicatorConfig: indicators.IndicatorConfig{Period: config.FastMAPeriod},
		Type:            indicators.ExponentialMovingAverage, // EMA for faster response
	})

	slowMA := indicators.NewMovingAverage(indicators.MovingAverageConfig{
		IndicatorConfig: indicators.IndicatorConfig{Period: config.SlowMAPeriod},
		Type:            indicators.ExponentialMovingAverage,
	})

	signalLine := indicators.NewMovingAverage(indicators.MovingAverageConfig{
		IndicatorConfig: indicators.IndicatorConfig{Period: config.SignalPeriod},
		Type:            indicators.ExponentialMovingAverage,
	})

	atr := indicators.NewATR(indicators.ATRConfig{
		IndicatorConfig: indicators.IndicatorConfig{Period: config.ATRPeriod},
	})

	// RSI for additional confirmation
	rsi := indicators.NewRSI(indicators.RSIConfig{
		IndicatorConfig: indicators.IndicatorConfig{Period: 14},
		Overbought:      70,
		Oversold:        30,
	})

	// Create trend timeframe indicators if multi-timeframe is enabled
	var trendFastMA, trendSlowMA *indicators.MovingAverage
	if config.UseMultiTimeframe {
		trendFastMA = indicators.NewMovingAverage(indicators.MovingAverageConfig{
			IndicatorConfig: indicators.IndicatorConfig{Period: config.FastMAPeriod},
			Type:            indicators.ExponentialMovingAverage,
		})

		trendSlowMA = indicators.NewMovingAverage(indicators.MovingAverageConfig{
			IndicatorConfig: indicators.IndicatorConfig{Period: config.SlowMAPeriod},
			Type:            indicators.ExponentialMovingAverage,
		})
	}

	// Create scalping timeframe indicators if enabled
	var scalpFastMA, scalpSlowMA *indicators.MovingAverage
	if config.UseScalpTimeframe {
		scalpFastMA = indicators.NewMovingAverage(indicators.MovingAverageConfig{
			IndicatorConfig: indicators.IndicatorConfig{Period: config.ScalpFastPeriod},
			Type:            indicators.ExponentialMovingAverage,
		})

		scalpSlowMA = indicators.NewMovingAverage(indicators.MovingAverageConfig{
			IndicatorConfig: indicators.IndicatorConfig{Period: config.ScalpSlowPeriod},
			Type:            indicators.ExponentialMovingAverage,
		})
	}

	return &MACrossover{
		BaseStrategy:          NewBaseStrategy(logger),
		config:                config,
		fastMA:                fastMA,
		slowMA:                slowMA,
		signalLine:            signalLine,
		atr:                   atr,
		rsi:                   rsi,
		trendFastMA:           trendFastMA,
		trendSlowMA:           trendSlowMA,
		scalpFastMA:           scalpFastMA,
		scalpSlowMA:           scalpSlowMA,
		dailyLossCount:        0,
		consecutiveLosses:     0,
		lastLossResetDay:      time.Now().Truncate(24 * time.Hour),
		partialTakeProfit:     false,
		lastTradeResult:       0,
		recentVolatility:      make([]float64, 0, 20), // Track last 20 ATR values
		winCount:              0,
		lossCount:             0,
		totalPnL:              0,
		lastTradeTime:         time.Now(),
		consolidationDetected: false,
	}, nil
}

// Name returns the name of the strategy
func (m *MACrossover) Name() string {
	return "Improved Moving Average Crossover"
}

// RequiredDataPoints returns the minimum number of klines needed for the strategy
func (m *MACrossover) RequiredDataPoints() int {
	// Use the maximum period plus some buffer for calculations
	maxPeriod := m.config.SlowMAPeriod
	if m.config.ATRPeriod > maxPeriod {
		maxPeriod = m.config.ATRPeriod
	}
	return maxPeriod + 30 // Add buffer for trend detection
}

// detectMarketRegime determines if the market is in a tradeable regime
// Returns: isUptrend, isTradeable, trendStrength
func (m *MACrossover) detectMarketRegime(ctx context.Context, klines []*domain.Kline) (bool, bool, float64) {
	// Calculate slow MA for multiple periods to determine trend direction
	slowMA, err := m.slowMA.Calculate(ctx, klines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate slow MA")
		return false, false, 0
	}

	// Calculate slow MA for previous periods to determine trend direction
	prevKlines := klines[:len(klines)-5] // 5 periods ago
	prevSlowMA, err := m.slowMA.Calculate(ctx, prevKlines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate previous slow MA")
		return false, false, 0
	}

	// Calculate even earlier slow MA to confirm trend persistence
	earlierKlines := klines[:len(klines)-10] // 10 periods ago
	earlierSlowMA, err := m.slowMA.Calculate(ctx, earlierKlines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate earlier slow MA")
		return false, false, 0
	}

	// Calculate ATR to measure volatility
	atr, err := m.atr.Calculate(ctx, klines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate ATR")
		return false, false, 0
	}

	// Current price
	currentPrice := klines[len(klines)-1].Close

	// Determine trend direction and strength
	isUptrend := slowMA > prevSlowMA && prevSlowMA > earlierSlowMA
	trendStrength := (slowMA/earlierSlowMA - 1) * 100 // Percentage change

	// Volatility as percentage of price
	volatilityPercent := atr / currentPrice * 100

	// Track recent volatility for trend analysis
	if len(m.recentVolatility) >= 20 {
		m.recentVolatility = m.recentVolatility[1:] // Remove oldest
	}
	m.recentVolatility = append(m.recentVolatility, volatilityPercent)

	// Calculate average volatility
	avgVolatility := 0.0
	for _, v := range m.recentVolatility {
		avgVolatility += v
	}
	if len(m.recentVolatility) > 0 {
		avgVolatility /= float64(len(m.recentVolatility))
	}

	// Check if current volatility is expanding (good for trend following)
	isVolatilityExpanding := len(m.recentVolatility) >= 5 &&
		volatilityPercent > avgVolatility*1.1 // 10% above average

	// Check trading hours if enabled
	isWithinTradingHours := true
	if m.config.TradingHoursOnly {
		currentHour := klines[len(klines)-1].OpenTime.Hour()
		isWithinTradingHours = currentHour >= m.config.TradingStartHour &&
			currentHour < m.config.TradingEndHour
	}

	// Check daily loss limit
	currentDay := klines[len(klines)-1].OpenTime.Truncate(24 * time.Hour)
	if !currentDay.Equal(m.lastLossResetDay) {
		// Reset daily loss counter at the start of a new day
		m.dailyLossCount = 0
		m.lastLossResetDay = currentDay
	}

	isUnderLossLimit := m.dailyLossCount < m.config.MaxDailyLosses

	// Market is tradeable if:
	// 1. There's a clear trend direction
	// 2. Trend strength is significant (> 0.15%) - reduced from 0.2% for more trades
	// 3. Volatility is reasonable (not too low, not too high) - widened range
	// 4. Within trading hours (if enabled)
	// 5. Under daily loss limit
	isTradeable := isUptrend &&
		trendStrength > 0.15 &&
		volatilityPercent > 0.15 && // Reduced from 0.2% for more trades
		volatilityPercent < 5.0 && // Increased from 4.0% for more trades
		isWithinTradingHours &&
		isUnderLossLimit

	// Log detailed market regime information
	m.logger.Debug(ctx, "Market regime analysis", map[string]interface{}{
		"isUptrend":             isUptrend,
		"trendStrength":         trendStrength,
		"volatilityPercent":     volatilityPercent,
		"avgVolatility":         avgVolatility,
		"isVolatilityExpanding": isVolatilityExpanding,
		"isWithinTradingHours":  isWithinTradingHours,
		"dailyLossCount":        m.dailyLossCount,
		"isUnderLossLimit":      isUnderLossLimit,
		"isTradeable":           isTradeable,
	})

	return isUptrend, isTradeable, trendStrength
}

// analyzeHigherTimeframe analyzes the trend on a higher timeframe
// Returns: isUptrend, trendStrength
func (m *MACrossover) analyzeHigherTimeframe(ctx context.Context, trendKlines []*domain.Kline) (bool, float64) {
	if !m.config.UseMultiTimeframe || len(trendKlines) < m.config.SlowMAPeriod+10 {
		return true, 0 // Default to true if not using multi-timeframe
	}

	// Calculate trend indicators
	trendFastMA, err := m.trendFastMA.Calculate(ctx, trendKlines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate trend fast MA")
		return true, 0
	}

	trendSlowMA, err := m.trendSlowMA.Calculate(ctx, trendKlines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate trend slow MA")
		return true, 0
	}

	// Calculate previous trend indicators for comparison
	prevTrendKlines := trendKlines[:len(trendKlines)-5]
	prevTrendFastMA, err := m.trendFastMA.Calculate(ctx, prevTrendKlines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate previous trend fast MA")
		return true, 0
	}

	// We don't use prevTrendSlowMA directly, but we calculate it for logging purposes
	_, err = m.trendSlowMA.Calculate(ctx, prevTrendKlines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate previous trend slow MA")
		return true, 0
	}

	// Determine trend direction and strength
	isUptrend := trendFastMA > trendSlowMA && trendFastMA > prevTrendFastMA
	trendStrength := (trendFastMA/prevTrendFastMA - 1) * 100 // Percentage change

	m.logger.Debug(ctx, "Higher timeframe analysis", map[string]interface{}{
		"timeframe":     m.config.TrendTimeframe,
		"isUptrend":     isUptrend,
		"trendStrength": trendStrength,
		"trendFastMA":   trendFastMA,
		"trendSlowMA":   trendSlowMA,
	})

	return isUptrend, trendStrength
}

// detectPullback detects pullbacks in an uptrend for entry opportunities
func (m *MACrossover) detectPullback(ctx context.Context, klines []*domain.Kline, currentPrice float64) bool {
	if len(klines) < 10 {
		return false
	}

	// Check if we have a recent pullback (price dipped and is now recovering)
	recentLow := klines[len(klines)-2].Low
	previousLow := klines[len(klines)-3].Low

	// Check if we had a dip (lower low)
	hadDip := recentLow < previousLow

	// Check if price is now recovering from the dip
	isRecovering := currentPrice > klines[len(klines)-2].Close

	// Check if the pullback wasn't too deep (not more than 1.5% from recent high)
	recentHigh := 0.0
	for i := 1; i <= 5; i++ {
		if klines[len(klines)-i].High > recentHigh {
			recentHigh = klines[len(klines)-i].High
		}
	}

	pullbackDepth := (recentHigh - recentLow) / recentHigh * 100
	isShallowPullback := pullbackDepth < 1.5 && pullbackDepth > 0.2

	return hadDip && isRecovering && isShallowPullback
}

// detectScalpingOpportunity detects short-term scalping opportunities
func (m *MACrossover) detectScalpingOpportunity(ctx context.Context, klines []*domain.Kline, currentPrice float64) bool {
	if !m.config.UseScalpTimeframe || len(klines) < 20 {
		return false
	}

	// Calculate scalping indicators
	scalpFastMA, err := m.scalpFastMA.Calculate(ctx, klines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate scalp fast MA")
		return false
	}

	scalpSlowMA, err := m.scalpSlowMA.Calculate(ctx, klines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate scalp slow MA")
		return false
	}

	// Calculate RSI for oversold/overbought conditions
	rsi, err := m.rsi.Calculate(ctx, klines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate RSI for scalping")
		return false
	}

	// Check for volume spike (potential reversal signal)
	isVolumeSpiking := klines[len(klines)-1].Volume > klines[len(klines)-2].Volume*1.3

	// Check for quick reversal in oversold conditions
	isOversold := rsi < 35
	isRsiRising := rsi > calculateRSI(klines, len(klines)-2, 14)

	// Check for bullish MA crossover on scalping timeframe
	hasCrossedAbove := scalpFastMA > scalpSlowMA &&
		calculateMA(klines, len(klines)-2, m.config.ScalpFastPeriod) <= calculateMA(klines, len(klines)-2, m.config.ScalpSlowPeriod)

	// Check for price action patterns
	// Look for bullish engulfing or hammer pattern
	isBullishCandle := klines[len(klines)-1].Close > klines[len(klines)-1].Open
	isPreviousBearish := klines[len(klines)-2].Close < klines[len(klines)-2].Open
	isBullishEngulfing := isBullishCandle && isPreviousBearish &&
		klines[len(klines)-1].Open < klines[len(klines)-2].Close &&
		klines[len(klines)-1].Close > klines[len(klines)-2].Open

	// Check for hammer pattern (long lower wick, small body)
	bodySize := math.Abs(klines[len(klines)-1].Close - klines[len(klines)-1].Open)
	lowerWick := math.Min(klines[len(klines)-1].Open, klines[len(klines)-1].Close) - klines[len(klines)-1].Low
	isHammer := lowerWick > bodySize*2 && bodySize < (klines[len(klines)-1].High-klines[len(klines)-1].Low)*0.3

	// Combine signals for scalping opportunity
	hasScalpingOpportunity := (isOversold && isRsiRising) || hasCrossedAbove || isBullishEngulfing || isHammer

	// Only consider scalping if volume is increasing
	if hasScalpingOpportunity && isVolumeSpiking {
		m.logger.Info(ctx, "Scalping opportunity detected", map[string]interface{}{
			"currentPrice":       currentPrice,
			"scalpFastMA":        scalpFastMA,
			"scalpSlowMA":        scalpSlowMA,
			"rsi":                rsi,
			"isOversold":         isOversold,
			"isRsiRising":        isRsiRising,
			"hasCrossedAbove":    hasCrossedAbove,
			"isBullishEngulfing": isBullishEngulfing,
			"isHammer":           isHammer,
			"isVolumeSpiking":    isVolumeSpiking,
		})
		return true
	}

	return false
}

// detectVolatilityDrop detects when intraday volatility is dropping
func (m *MACrossover) detectVolatilityDrop(ctx context.Context, klines []*domain.Kline) bool {
	if len(klines) < 20 {
		return false
	}

	// Calculate current ATR
	atr, err := m.atr.Calculate(ctx, klines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate ATR for volatility drop detection")
		return false
	}

	// Calculate ATR from 3 periods ago
	prevKlines := klines[:len(klines)-3]
	prevAtr, err := m.atr.Calculate(ctx, prevKlines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate previous ATR")
		return false
	}

	// Calculate ATR from 6 periods ago
	earlierKlines := klines[:len(klines)-6]
	earlierAtr, err := m.atr.Calculate(ctx, earlierKlines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate earlier ATR")
		return false
	}

	// Check if volatility is consistently dropping
	isVolatilityDropping := atr < prevAtr && prevAtr < earlierAtr

	// Calculate percentage drop
	volatilityDropPercent := 0.0
	if earlierAtr > 0 {
		volatilityDropPercent = (earlierAtr - atr) / earlierAtr * 100
	}

	// Significant drop is more than 15%
	isSignificantDrop := volatilityDropPercent > 15

	if isVolatilityDropping && isSignificantDrop {
		m.logger.Info(ctx, "Volatility drop detected", map[string]interface{}{
			"currentAtr":            atr,
			"prevAtr":               prevAtr,
			"earlierAtr":            earlierAtr,
			"volatilityDropPercent": volatilityDropPercent,
		})
		return true
	}

	return false
}

// detectConsolidation detects when price is consolidating (moving sideways)
func (m *MACrossover) detectConsolidation(ctx context.Context, klines []*domain.Kline, periods int) bool {
	if len(klines) < periods+5 {
		return false
	}

	// Calculate the high and low of the last 'periods' candles
	high := klines[len(klines)-1].High
	low := klines[len(klines)-1].Low

	for i := 2; i <= periods; i++ {
		if klines[len(klines)-i].High > high {
			high = klines[len(klines)-i].High
		}
		if klines[len(klines)-i].Low < low {
			low = klines[len(klines)-i].Low
		}
	}

	// Calculate the range as a percentage of the average price
	avgPrice := (high + low) / 2
	rangePercent := (high - low) / avgPrice * 100

	// Calculate the average true range for comparison
	atr, err := m.atr.Calculate(ctx, klines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate ATR for consolidation detection")
		return false
	}

	// Calculate ATR as percentage of price
	currentPrice := klines[len(klines)-1].Close
	atrPercent := atr / currentPrice * 100

	// Consolidation is when the range is less than 1.5x the ATR
	isConsolidating := rangePercent < atrPercent*1.5

	// Also check if price is moving sideways (no clear trend)
	fastMA, err := m.fastMA.Calculate(ctx, klines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate fast MA for consolidation detection")
		return false
	}

	// Calculate fast MA from 5 periods ago
	prevKlines := klines[:len(klines)-5]
	prevFastMA, err := m.fastMA.Calculate(ctx, prevKlines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate previous fast MA")
		return false
	}

	// Check if MA is flat (less than 0.1% change)
	isMaFlat := math.Abs(fastMA-prevFastMA)/prevFastMA*100 < 0.1

	if isConsolidating && isMaFlat {
		m.logger.Info(ctx, "Price consolidation detected", map[string]interface{}{
			"rangePercent": rangePercent,
			"atrPercent":   atrPercent,
			"maChange":     math.Abs(fastMA-prevFastMA) / prevFastMA * 100,
			"periods":      periods,
		})
		m.consolidationDetected = true
		return true
	}

	return false
}

// isApproachingMarketClose checks if we're approaching the end of trading hours
func (m *MACrossover) isApproachingMarketClose(currentTime time.Time) bool {
	if !m.config.TradingHoursOnly {
		return false
	}

	// For day trading, consider "market close" as the configured end hour
	endHour := m.config.TradingEndHour
	currentHour := currentTime.Hour()
	currentMinute := currentTime.Minute()

	// If within 30 minutes of close
	return currentHour == endHour-1 && currentMinute >= 30
}

// calculateDynamicHoldingTime calculates a dynamic holding time based on market conditions
func (m *MACrossover) calculateDynamicHoldingTime(ctx context.Context, klines []*domain.Kline, position *domain.Position, profitPercent float64) time.Duration {
	// Base holding time from config
	baseTime := m.config.MaxHoldingTime

	// Adjust based on market session
	currentHour := klines[len(klines)-1].OpenTime.Hour()
	if currentHour >= 14 && currentHour <= 20 { // Afternoon/evening session
		baseTime = baseTime * 3 / 4 // 75% of normal holding time
	}

	// Adjust based on profit/loss
	if profitPercent < 0 {
		// If in loss, reduce holding time based on loss severity
		lossAdjustment := math.Min(math.Abs(profitPercent)/2, 50) / 100
		return time.Duration(float64(baseTime) * (1 - lossAdjustment))
	} else if profitPercent > 0 && profitPercent < 0.3 {
		// Small profit but not enough - give it a bit more time
		return time.Duration(float64(baseTime) * 1.2) // 20% more time
	}

	return baseTime
}

// Helper function to calculate RSI at a specific point in history
func calculateRSI(klines []*domain.Kline, endIndex int, period int) float64 {
	if endIndex < period || endIndex >= len(klines) {
		return 50 // Default to neutral if not enough data
	}

	// Calculate price changes
	changes := make([]float64, period)
	for i := 0; i < period; i++ {
		changes[i] = klines[endIndex-i].Close - klines[endIndex-i-1].Close
	}

	// Calculate average gains and losses
	var sumGain, sumLoss float64
	for _, change := range changes {
		if change > 0 {
			sumGain += change
		} else {
			sumLoss -= change // Make positive for calculation
		}
	}

	// Calculate RS and RSI
	avgGain := sumGain / float64(period)
	avgLoss := sumLoss / float64(period)

	if avgLoss == 0 {
		return 100 // Prevent division by zero
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))

	return rsi
}

// ShouldEnterTrade implements the strategy's entry logic with improved conditions for day trading
func (m *MACrossover) ShouldEnterTrade(ctx context.Context, klines []*domain.Kline, currentPrice float64) bool {
	requiredPoints := m.RequiredDataPoints()
	if len(klines) < requiredPoints {
		m.logger.Debug(ctx, "Not enough kline data for strategy evaluation",
			map[string]interface{}{"available": len(klines), "required": requiredPoints})
		return false
	}

	// 1. Check market regime first - only trade in favorable conditions
	isUptrend, isTradeable, trendStrength := m.detectMarketRegime(ctx, klines)
	if !isTradeable {
		// Check for scalping opportunity even if main regime isn't tradeable
		if m.config.UseScalpTimeframe && m.detectScalpingOpportunity(ctx, klines, currentPrice) {
			m.logger.Info(ctx, "Entering trade based on scalping opportunity despite unfavorable market regime", nil)
			return true
		}

		m.logger.Debug(ctx, "Market regime not favorable for trading",
			map[string]interface{}{
				"isUptrend":     isUptrend,
				"trendStrength": trendStrength,
			})
		return false
	}

	// 2. Check higher timeframe trend if multi-timeframe analysis is enabled
	var higherTimeframeUptrend bool
	var higherTimeframeTrendStrength float64

	if m.config.UseMultiTimeframe {
		// This would be implemented in a real system by passing the higher timeframe klines
		// For now, we'll use the same klines but assume they're from the higher timeframe
		higherTimeframeUptrend, higherTimeframeTrendStrength = m.analyzeHigherTimeframe(ctx, klines)

		// Only proceed if higher timeframe is in uptrend
		if !higherTimeframeUptrend {
			m.logger.Debug(ctx, "Higher timeframe not in uptrend",
				map[string]interface{}{
					"timeframe":     m.config.TrendTimeframe,
					"trendStrength": higherTimeframeTrendStrength,
				})
			return false
		}
	}

	// 3. Calculate core indicators
	fastMA, err := m.fastMA.Calculate(ctx, klines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate fast MA")
		return false
	}

	slowMA, err := m.slowMA.Calculate(ctx, klines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate slow MA")
		return false
	}

	signalMA, err := m.signalLine.Calculate(ctx, klines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate signal line")
		return false
	}

	rsi, err := m.rsi.Calculate(ctx, klines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate RSI")
		return false
	}

	atr, err := m.atr.Calculate(ctx, klines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate ATR")
		return false
	}

	// 4. Calculate additional confirmation indicators

	// Volume trend (increasing volume is bullish)
	recentVolume := 0.0
	pastVolume := 0.0
	for i := 0; i < 5; i++ {
		recentVolume += klines[len(klines)-1-i].Volume
	}
	for i := 5; i < 10; i++ {
		pastVolume += klines[len(klines)-1-i].Volume
	}
	volumeRatio := recentVolume / pastVolume

	// Price momentum (rate of change)
	momentum := (currentPrice - klines[len(klines)-10].Close) / klines[len(klines)-10].Close * 100

	// Higher highs and higher lows pattern
	isHigherHigh := klines[len(klines)-1].High > klines[len(klines)-2].High &&
		klines[len(klines)-2].High > klines[len(klines)-3].High
	isHigherLow := klines[len(klines)-1].Low > klines[len(klines)-2].Low &&
		klines[len(klines)-2].Low > klines[len(klines)-3].Low

	// 5. Entry conditions - relaxed for more trades

	// Primary trend condition: Fast MA crossed above Slow MA recently
	// or price is above both MAs in an established uptrend
	hasCrossedAbove := fastMA > slowMA &&
		calculateMA(klines, len(klines)-3, m.config.FastMAPeriod) <= calculateMA(klines, len(klines)-3, m.config.SlowMAPeriod)

	isPriceAboveMAs := currentPrice > fastMA && currentPrice > slowMA

	// Check for pullback entry in established uptrend
	isPullbackEntry := fastMA > slowMA && // Already in uptrend
		fastMA > calculateMA(klines, len(klines)-5, m.config.FastMAPeriod) && // Trend is rising
		m.detectPullback(ctx, klines, currentPrice) // Detected a pullback

	// Signal line confirmation
	isAboveSignal := currentPrice > signalMA

	// RSI condition: Not overbought, in healthy range (widened range)
	isHealthyRSI := rsi > 35 && rsi < 68 // Widened from 40-65 for more trades

	// Momentum condition: Positive momentum (reduced threshold)
	isStrongMomentum := momentum > 0.3 // Reduced from 0.5 for more trades

	// Volume condition: Increasing volume
	isIncreasingVolume := volumeRatio > 1.1 // Reduced from 1.2 for more trades

	// Pattern recognition
	hasConfirmationPattern := isHigherHigh || isHigherLow

	// Volatility check: ATR should be reasonable relative to price
	isReasonableVolatility := atr < (currentPrice * 0.015) // Increased from 0.012 for more trades

	// Count how many confirmation conditions are met
	confirmationCount := 0
	if isAboveSignal {
		confirmationCount++
	}
	if isHealthyRSI {
		confirmationCount++
	}
	if isStrongMomentum {
		confirmationCount++
	}
	if isIncreasingVolume {
		confirmationCount++
	}
	if hasConfirmationPattern {
		confirmationCount++
	}
	if isReasonableVolatility {
		confirmationCount++
	}

	// Add multi-timeframe confirmation if enabled
	if m.config.UseMultiTimeframe && higherTimeframeUptrend && higherTimeframeTrendStrength > 0.3 {
		confirmationCount++
	}

	// Need primary conditions plus at least 2 confirmation conditions (reduced from 3)
	// Also allow pullback entries in established uptrends
	if ((hasCrossedAbove && isPriceAboveMAs) || isPullbackEntry) && confirmationCount >= 2 {
		m.logger.Info(ctx, "Trade entry conditions met", map[string]interface{}{
			"currentPrice":      currentPrice,
			"fastMA":            fastMA,
			"slowMA":            slowMA,
			"signalMA":          signalMA,
			"rsi":               rsi,
			"momentum":          momentum,
			"volumeRatio":       volumeRatio,
			"atr":               atr,
			"trendStrength":     trendStrength,
			"confirmationCount": confirmationCount,
			"isPullbackEntry":   isPullbackEntry,
			"hasCrossedAbove":   hasCrossedAbove,
		})
		return true
	}

	// Check for scalping opportunity as a last resort
	if m.config.UseScalpTimeframe && m.detectScalpingOpportunity(ctx, klines, currentPrice) {
		m.logger.Info(ctx, "Trade entry conditions met via scalping opportunity", nil)
		return true
	}

	m.logger.Debug(ctx, "Trade entry conditions not met", map[string]interface{}{
		"currentPrice":       currentPrice,
		"fastMA":             fastMA,
		"slowMA":             slowMA,
		"signalMA":           signalMA,
		"rsi":                rsi,
		"momentum":           momentum,
		"volumeRatio":        volumeRatio,
		"atr":                atr,
		"hasCrossedAbove":    hasCrossedAbove,
		"isPriceAboveMAs":    isPriceAboveMAs,
		"isPullbackEntry":    isPullbackEntry,
		"isAboveSignal":      isAboveSignal,
		"isHealthyRSI":       isHealthyRSI,
		"isStrongMomentum":   isStrongMomentum,
		"isIncreasingVolume": isIncreasingVolume,
		"confirmationCount":  confirmationCount,
	})
	return false
}

// ShouldClosePosition implements the strategy's exit logic with improved risk management
func (m *MACrossover) ShouldClosePosition(ctx context.Context, position *domain.Position, klines []*domain.Kline, currentPrice float64) (bool, domain.CloseReason) {
	if !position.IsOpen() {
		return false, ""
	}

	// Calculate indicators for exit decisions
	fastMA, err := m.fastMA.Calculate(ctx, klines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate fast MA")
		return false, ""
	}

	slowMA, err := m.slowMA.Calculate(ctx, klines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate slow MA")
		return false, ""
	}

	signalMA, err := m.signalLine.Calculate(ctx, klines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate signal line")
		return false, ""
	}

	rsi, err := m.rsi.Calculate(ctx, klines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate RSI")
		return false, ""
	}

	atr, err := m.atr.Calculate(ctx, klines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate ATR")
		return false, ""
	}

	// Calculate current profit percentage
	profitPercent := (currentPrice - position.EntryPrice) / position.EntryPrice * 100

	// 0. Check for approaching market close (for day trading)
	if m.isApproachingMarketClose(klines[len(klines)-1].OpenTime) && profitPercent > 0 {
		m.logger.Info(ctx, "Closing position due to approaching market close", map[string]interface{}{
			"currentTime":   klines[len(klines)-1].OpenTime,
			"profitPercent": profitPercent,
		})
		return true, domain.CloseReasonMarketClose
	}

	// 1. Dynamic time-based exit based on configuration
	currentKlineTime := klines[len(klines)-1].OpenTime
	holdingTime := currentKlineTime.Sub(position.EntryTime)

	// Use more sophisticated dynamic holding time calculation
	adjustedMaxHoldingTime := m.calculateDynamicHoldingTime(ctx, klines, position, profitPercent)

	if holdingTime > adjustedMaxHoldingTime {
		m.logger.Info(ctx, "Closing position due to max holding time reached", map[string]interface{}{
			"entryTime":            position.EntryTime,
			"currentKlineTime":     currentKlineTime,
			"holdingTime":          holdingTime.String(),
			"maxHoldingPeriod":     m.config.MaxHoldingTime.String(),
			"adjustedHoldingTime":  adjustedMaxHoldingTime.String(),
			"profitPercent":        profitPercent,
			"timeAdjustmentReason": getTimeAdjustmentReason(profitPercent),
		})

		// Update daily loss counter if we're closing at a loss
		if profitPercent < 0 {
			m.dailyLossCount++
			m.consecutiveLosses++
			m.lastTradeResult = profitPercent
			m.logger.Info(ctx, "Updating loss counters", map[string]interface{}{
				"dailyLossCount":    m.dailyLossCount,
				"consecutiveLosses": m.consecutiveLosses,
				"maxDailyLosses":    m.config.MaxDailyLosses,
			})
		} else {
			// Reset consecutive losses if we have a winning trade
			m.consecutiveLosses = 0
		}

		return true, domain.CloseReasonTimeLimit
	}

	// 1.1 Check for price consolidation (sideways movement)
	if m.detectConsolidation(ctx, klines, 12) && profitPercent > 0 {
		m.logger.Info(ctx, "Closing position due to price consolidation", map[string]interface{}{
			"profitPercent": profitPercent,
			"holdingTime":   holdingTime.String(),
		})
		return true, domain.CloseReasonConsolidation
	}

	// 1.2 Check for volatility drop (market losing momentum)
	if m.detectVolatilityDrop(ctx, klines) && profitPercent > 0 {
		m.logger.Info(ctx, "Closing position due to volatility drop", map[string]interface{}{
			"profitPercent": profitPercent,
			"holdingTime":   holdingTime.String(),
		})
		return true, domain.CloseReasonVolatilityDrop
	}

	// 2. Enhanced trailing stop logic - activate earlier at 0.2% profit (was 0.3%)
	if profitPercent >= m.config.TrailingActivePct*100 && position.TrailingStopPrice == 0 {
		// Initialize trailing stop with ATR-based distance
		atrDistance := atr * 1.5                                                  // Use 1.5x ATR for trailing stop distance
		position.TrailingStopDistance = math.Min(atrDistance, currentPrice*0.004) // Cap at 0.4%
		position.TrailingStopPrice = currentPrice - position.TrailingStopDistance
		m.logger.Info(ctx, "Trailing stop initialized", map[string]interface{}{
			"currentPrice":        currentPrice,
			"trailingStopPrice":   position.TrailingStopPrice,
			"trailingStopPercent": position.TrailingStopDistance / currentPrice * 100,
			"profitPercent":       profitPercent,
			"atrValue":            atr,
		})
	} else if position.TrailingStopPrice > 0 && currentPrice > position.TrailingStopPrice+position.TrailingStopDistance {
		// Update trailing stop if price moves higher
		newTrailingStop := currentPrice - position.TrailingStopDistance

		// Progressive trailing stop tightening as profit increases
		if m.config.TrailingStopTightening {
			if profitPercent >= 1.5 {
				// Tighten by 30% for profits >= 1.5%
				position.TrailingStopDistance *= 0.7
				m.logger.Info(ctx, "Tightening trailing stop by 30%", map[string]interface{}{
					"newDistance":   position.TrailingStopDistance,
					"profitPercent": profitPercent,
				})
			} else if profitPercent >= 1.0 {
				// Tighten by 20% for profits >= 1.0%
				position.TrailingStopDistance *= 0.8
				m.logger.Info(ctx, "Tightening trailing stop by 20%", map[string]interface{}{
					"newDistance":   position.TrailingStopDistance,
					"profitPercent": profitPercent,
				})
			} else if profitPercent >= 0.5 {
				// Tighten by 10% for profits >= 0.5%
				position.TrailingStopDistance *= 0.9
				m.logger.Info(ctx, "Tightening trailing stop by 10%", map[string]interface{}{
					"newDistance":   position.TrailingStopDistance,
					"profitPercent": profitPercent,
				})
			}
		}

		// Recalculate new trailing stop with potentially tightened distance
		newTrailingStop = currentPrice - position.TrailingStopDistance

		if newTrailingStop > position.TrailingStopPrice {
			position.TrailingStopPrice = newTrailingStop
			m.logger.Info(ctx, "Trailing stop updated", map[string]interface{}{
				"currentPrice":      currentPrice,
				"trailingStopPrice": position.TrailingStopPrice,
				"profitPercent":     profitPercent,
			})
		}
	}

	// 2.1 Partial profit taking at 0.5% profit (was 1%)
	if profitPercent >= m.config.PartialProfitPct*100 && !m.partialTakeProfit {
		m.partialTakeProfit = true
		m.logger.Info(ctx, "Partial profit taking signal", map[string]interface{}{
			"currentPrice":  currentPrice,
			"entryPrice":    position.EntryPrice,
			"profitPercent": profitPercent,
			"partialPct":    m.config.PartialProfitPct * 100,
		})
		// In a real implementation, we would reduce position size here
		// For backtesting, we'll just log it and move stop loss to breakeven
		if position.StopLoss < position.EntryPrice {
			position.StopLoss = position.EntryPrice * 1.001 // Breakeven + 0.1%
			m.logger.Info(ctx, "Moving stop loss to breakeven after partial profit", map[string]interface{}{
				"newStopLoss": position.StopLoss,
			})
		}
	}

	// 2.2 Earlier breakeven activation
	if profitPercent >= m.config.BreakEvenActivation*100 && position.StopLoss < position.EntryPrice {
		position.StopLoss = position.EntryPrice * 1.0001 // Breakeven + 0.01%
		m.logger.Info(ctx, "Moving stop loss to breakeven at small profit", map[string]interface{}{
			"profitPercent": profitPercent,
			"newStopLoss":   position.StopLoss,
		})
	}

	// Check for trailing stop hit
	if position.TrailingStopPrice > 0 && currentPrice <= position.TrailingStopPrice {
		m.logger.Info(ctx, "Trailing stop triggered", map[string]interface{}{
			"currentPrice":      currentPrice,
			"trailingStopPrice": position.TrailingStopPrice,
			"profitPercent":     profitPercent,
		})
		return true, domain.CloseReasonStopLoss
	}

	// 3. Improved dynamic stop loss with wider initial stop
	// Use ATR-based stop loss with higher multiplier for more room
	atrMultiplier := m.config.ATRMultiplier // Higher multiplier (e.g., 2.5 instead of 1.5)
	atrStopLoss := position.EntryPrice - (atr * atrMultiplier)

	// Use the higher of ATR-based stop loss or fixed stop loss
	dynamicStopLoss := math.Max(atrStopLoss, position.StopLoss)

	// Tiered profit-based stop loss levels - more aggressive
	if profitPercent >= 1.5 {
		// Move stop loss to break even + 1.0% when profit >= 1.5%
		dynamicStopLoss = math.Max(dynamicStopLoss, position.EntryPrice*1.01)
	} else if profitPercent >= 1.0 {
		// Move stop loss to break even + 0.7% when profit >= 1.0%
		dynamicStopLoss = math.Max(dynamicStopLoss, position.EntryPrice*1.007)
	} else if profitPercent >= 0.5 {
		// Move stop loss to break even + 0.3% when profit >= 0.5%
		dynamicStopLoss = math.Max(dynamicStopLoss, position.EntryPrice*1.003)
	}

	// Check for stop loss hit with dynamic level
	if currentPrice <= dynamicStopLoss {
		m.logger.Info(ctx, "Dynamic stop loss triggered", map[string]interface{}{
			"currentPrice":    currentPrice,
			"dynamicStopLoss": dynamicStopLoss,
			"atrValue":        atr,
			"atrStopLoss":     atrStopLoss,
			"profitPercent":   profitPercent,
		})
		return true, domain.CloseReasonStopLoss
	}

	// 4. Take profit check
	if currentPrice >= position.TakeProfit {
		m.logger.Info(ctx, "Take profit triggered", map[string]interface{}{
			"currentPrice":  currentPrice,
			"takeProfit":    position.TakeProfit,
			"profitPercent": profitPercent,
		})
		return true, domain.CloseReasonTakeProfit
	}

	// 5. Enhanced trend reversal detection - more sensitive
	// Check for MA crossover (fast MA crosses below slow MA)
	hasCrossedBelow := fastMA < slowMA &&
		calculateMA(klines, len(klines)-3, m.config.FastMAPeriod) >= calculateMA(klines, len(klines)-3, m.config.SlowMAPeriod)

	// Price below signal line
	isBelowSignal := currentPrice < signalMA

	// RSI conditions
	isOverbought := rsi > 70

	// Momentum reversal
	momentum := (currentPrice - klines[len(klines)-5].Close) / klines[len(klines)-5].Close * 100
	prevMomentum := (klines[len(klines)-2].Close - klines[len(klines)-7].Close) / klines[len(klines)-7].Close * 100
	isLosingMomentum := momentum < prevMomentum && momentum < 0

	// Volume spike (potential reversal signal)
	isVolumeSpiking := klines[len(klines)-1].Volume > klines[len(klines)-2].Volume*1.5

	// Count reversal signals
	reversalSignalCount := 0
	if hasCrossedBelow {
		reversalSignalCount++
	}
	if isBelowSignal {
		reversalSignalCount++
	}
	if isOverbought {
		reversalSignalCount++
	}
	if isLosingMomentum {
		reversalSignalCount++
	}
	if isVolumeSpiking {
		reversalSignalCount++
	}

	// More sensitive trend reversal detection - require fewer signals and lower profit threshold
	if profitPercent > 0.2 && reversalSignalCount >= 2 {
		m.logger.Info(ctx, "Closing position due to trend reversal", map[string]interface{}{
			"profitPercent":       profitPercent,
			"hasCrossedBelow":     hasCrossedBelow,
			"isBelowSignal":       isBelowSignal,
			"isOverbought":        isOverbought,
			"isLosingMomentum":    isLosingMomentum,
			"isVolumeSpiking":     isVolumeSpiking,
			"reversalSignalCount": reversalSignalCount,
		})
		return true, domain.CloseReasonTrendReversal
	}

	return false, ""
}

// GetPositionSize calculates the optimal position size based on volatility
func (m *MACrossover) GetPositionSize(ctx context.Context, klines []*domain.Kline, availableFunds float64) float64 {
	// Calculate ATR for volatility assessment
	atr, err := m.atr.Calculate(ctx, klines)
	if err != nil {
		m.logger.Error(ctx, err, "Failed to calculate ATR for position sizing")
		return 1.0 // Default size
	}

	currentPrice := klines[len(klines)-1].Close

	// Volatility as percentage of price
	volatilityPercent := atr / currentPrice * 100

	// Base position size - inverse to volatility
	// Higher volatility = smaller position size
	var sizeFactor float64
	if volatilityPercent < 0.5 {
		sizeFactor = 1.0 // Low volatility - full size
	} else if volatilityPercent < 1.0 {
		sizeFactor = 0.8 // Medium-low volatility - 80% size
	} else if volatilityPercent < 1.5 {
		sizeFactor = 0.6 // Medium volatility - 60% size
	} else if volatilityPercent < 2.0 {
		sizeFactor = 0.4 // Medium-high volatility - 40% size
	} else {
		sizeFactor = 0.25 // High volatility - 25% size
	}

	// Dynamic leverage adjustment based on market conditions
	var leverageFactor float64
	if !m.config.DynamicLeverageAdjustment {
		leverageFactor = m.config.MaxLeverageUsed
	} else {
		// Adjust leverage based on volatility and market regime
		if volatilityPercent < 0.5 {
			leverageFactor = m.config.MaxLeverageUsed // Full leverage in low volatility
		} else if volatilityPercent < 1.0 {
			leverageFactor = m.config.MaxLeverageUsed * 0.75 // 75% of max in medium-low volatility
		} else if volatilityPercent < 1.5 {
			leverageFactor = m.config.MaxLeverageUsed * 0.5 // 50% of max in medium volatility
		} else {
			leverageFactor = m.config.MaxLeverageUsed * 0.25 // 25% of max in high volatility
		}

		// Further reduce leverage after consecutive losses
		if m.consecutiveLosses > 0 {
			leverageFactor *= math.Pow(0.7, float64(m.consecutiveLosses)) // Reduce by 30% for each consecutive loss
		}
	}

	// Ensure leverage never exceeds maximum
	if leverageFactor > m.config.MaxLeverageUsed {
		leverageFactor = m.config.MaxLeverageUsed
	}

	// Calculate position size based on available funds, size factor, and risk per trade
	// Convert funds to ETH equivalent (assuming ETH price is currentPrice)
	ethEquivalent := availableFunds / currentPrice

	// Apply risk per trade limit
	riskBasedSize := (availableFunds * m.config.InitialRiskPerTrade) / (currentPrice * 0.01) // Assuming 1% stop loss

	// Limit position size to 1 ETH maximum (realistic constraint)
	maxEth := 1.0
	if ethEquivalent > maxEth {
		ethEquivalent = maxEth
	}

	// Take the minimum of volatility-based size, risk-based size, and max ETH
	basePositionSize := math.Min(ethEquivalent*sizeFactor, riskBasedSize)

	// Apply leverage factor
	positionSize := basePositionSize * leverageFactor / m.config.MaxLeverageUsed

	m.logger.Info(ctx, "Calculated position size", map[string]interface{}{
		"volatilityPercent": volatilityPercent,
		"sizeFactor":        sizeFactor,
		"leverageFactor":    leverageFactor,
		"consecutiveLosses": m.consecutiveLosses,
		"riskBasedSize":     riskBasedSize,
		"basePositionSize":  basePositionSize,
		"positionSize":      positionSize,
		"availableFunds":    availableFunds,
		"ethEquivalent":     ethEquivalent,
		"maxEth":            maxEth,
	})

	return positionSize
}

// GetATR returns the current ATR value
func (m *MACrossover) GetATR(ctx context.Context, klines []*domain.Kline) (float64, error) {
	return m.atr.Calculate(ctx, klines)
}

// Helper function to calculate MA at a specific point in history
func calculateMA(klines []*domain.Kline, endIndex int, period int) float64 {
	if endIndex < period || endIndex >= len(klines) {
		return 0
	}

	sum := 0.0
	for i := endIndex - period + 1; i <= endIndex; i++ {
		sum += klines[i].Close
	}
	return sum / float64(period)
}

// getTimeAdjustmentReason returns a string describing why the holding time was adjusted
func getTimeAdjustmentReason(profitPercent float64) string {
	if profitPercent < 0 {
		return "Loss position"
	} else if profitPercent < 0.5 {
		return "Small profit"
	}
	return "Standard"
}
