package app

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cryptoMegaBot/config"
	"cryptoMegaBot/internal/domain"
	"cryptoMegaBot/internal/ports"
	"cryptoMegaBot/internal/strategy"
)

// Mock implementations
type mockLogger struct {
	debugMsgs []string
	infoMsgs  []string
	warnMsgs  []string
	errorMsgs []string
}

func (m *mockLogger) Debug(ctx context.Context, msg string, fields ...map[string]interface{}) {
	m.debugMsgs = append(m.debugMsgs, msg)
}

func (m *mockLogger) Info(ctx context.Context, msg string, fields ...map[string]interface{}) {
	m.infoMsgs = append(m.infoMsgs, msg)
}

func (m *mockLogger) Warn(ctx context.Context, msg string, fields ...map[string]interface{}) {
	m.warnMsgs = append(m.warnMsgs, msg)
}

func (m *mockLogger) Error(ctx context.Context, err error, msg string, fields ...map[string]interface{}) {
	m.errorMsgs = append(m.errorMsgs, msg)
}

func (m *mockLogger) Fatal(ctx context.Context, err error, msg string, fields ...map[string]interface{}) {
	// No-op for tests
}

type mockStrategy struct {
	shouldEnter bool
	shouldClose bool
	closeReason domain.CloseReason
}

func (m *mockStrategy) RequiredDataPoints() int {
	return 10
}

func (m *mockStrategy) ShouldEnterTrade(ctx context.Context, klines []*domain.Kline, currentPrice float64) bool {
	return m.shouldEnter
}

func (m *mockStrategy) ShouldClosePosition(ctx context.Context, position *domain.Position, klines []*domain.Kline, currentPrice float64) (bool, domain.CloseReason) {
	return m.shouldClose, m.closeReason
}

type mockExchange struct {
	serverTimeErr   error
	leverageErr     error
	markPrice       float64
	markPriceErr    error
	orderResponses  map[string]*ports.OrderResponse
	orderErrors     map[string]error
	klines          []*domain.Kline
	klinesErr       error
	positionRisk    *ports.PositionRisk
	positionRiskErr error
	serverTime      time.Time
}

func (m *mockExchange) GetServerTime(ctx context.Context) (time.Time, error) {
	return m.serverTime, m.serverTimeErr
}

func (m *mockExchange) SetServerTime(ctx context.Context) error {
	return m.serverTimeErr
}

func (m *mockExchange) GetMarkPrice(ctx context.Context, symbol string) (float64, error) {
	return m.markPrice, m.markPriceErr
}

func (m *mockExchange) GetTickerPrice(ctx context.Context, symbol string) (float64, error) {
	return m.markPrice, m.markPriceErr // Using same price as mark price for simplicity
}

func (m *mockExchange) GetAccountBalance(ctx context.Context, asset string) (float64, error) {
	return 1000.0, nil // Default test balance
}

func (m *mockExchange) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	return m.leverageErr
}

func (m *mockExchange) PlaceMarketOrder(ctx context.Context, symbol string, side domain.OrderSide, quantity string) (*ports.OrderResponse, error) {
	key := "market_" + string(side)
	return m.orderResponses[key], m.orderErrors[key]
}

func (m *mockExchange) PlaceStopMarketOrder(ctx context.Context, symbol string, side domain.OrderSide, quantity string, stopPrice string) (*ports.OrderResponse, error) {
	key := "stop_" + string(side)
	return m.orderResponses[key], m.orderErrors[key]
}

func (m *mockExchange) PlaceTakeProfitMarketOrder(ctx context.Context, symbol string, side domain.OrderSide, quantity string, stopPrice string) (*ports.OrderResponse, error) {
	key := "tp_" + string(side)
	return m.orderResponses[key], m.orderErrors[key]
}

func (m *mockExchange) GetPositionRisk(ctx context.Context, symbol string) (*ports.PositionRisk, error) {
	return m.positionRisk, m.positionRiskErr
}

func (m *mockExchange) CancelOrder(ctx context.Context, symbol string, orderID int64) (*ports.OrderResponse, error) {
	key := "cancel_" + strconv.FormatInt(orderID, 10)
	return m.orderResponses[key], m.orderErrors[key]
}

func (m *mockExchange) GetKlines(ctx context.Context, symbol string, interval string, limit int) ([]*domain.Kline, error) {
	return m.klines, m.klinesErr
}

func (m *mockExchange) StreamKlines(ctx context.Context, symbol string, interval string, klineHandler func(*domain.Kline), errorHandler func(error)) (chan struct{}, chan struct{}, error) {
	doneCh := make(chan struct{})
	stopCh := make(chan struct{})
	return doneCh, stopCh, nil
}

func (m *mockExchange) Ping(ctx context.Context) error {
	return nil
}

type mockPositionRepo struct {
	positions      map[string]*domain.Position
	createErr      error
	updateErr      error
	findOpenErr    error
	findByIDErr    error
	findAllErr     error
	totalProfit    float64
	totalProfitErr error
}

func (m *mockPositionRepo) Create(ctx context.Context, pos *domain.Position) (int64, error) {
	if m.createErr != nil {
		return 0, m.createErr
	}
	pos.ID = 1 // Assign test ID
	m.positions[pos.Symbol] = pos
	return pos.ID, nil
}

func (m *mockPositionRepo) Update(ctx context.Context, pos *domain.Position) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.positions[pos.Symbol] = pos
	return nil
}

func (m *mockPositionRepo) FindOpenBySymbol(ctx context.Context, symbol string) (*domain.Position, error) {
	if m.findOpenErr != nil {
		return nil, m.findOpenErr
	}
	pos := m.positions[symbol]
	if pos != nil && pos.Status == domain.StatusOpen {
		return pos, nil
	}
	return nil, nil
}

func (m *mockPositionRepo) FindByID(ctx context.Context, id int64) (*domain.Position, error) {
	if m.findByIDErr != nil {
		return nil, m.findByIDErr
	}
	for _, pos := range m.positions {
		if pos.ID == id {
			return pos, nil
		}
	}
	return nil, nil
}

func (m *mockPositionRepo) FindAll(ctx context.Context) ([]*domain.Position, error) {
	if m.findAllErr != nil {
		return nil, m.findAllErr
	}
	positions := make([]*domain.Position, 0, len(m.positions))
	for _, pos := range m.positions {
		positions = append(positions, pos)
	}
	return positions, nil
}

func (m *mockPositionRepo) GetTotalProfit(ctx context.Context) (float64, error) {
	return m.totalProfit, m.totalProfitErr
}

type mockTradeRepo struct {
	todayCount    int
	todayCountErr error
	trades        []*domain.Position
	findClosedErr error
}

func (m *mockTradeRepo) FindClosedBySymbol(ctx context.Context, symbol string, limit int) ([]*domain.Position, error) {
	if m.findClosedErr != nil {
		return nil, m.findClosedErr
	}
	return m.trades, nil
}

func (m *mockTradeRepo) CountTodayBySymbol(ctx context.Context, symbol string) (int, error) {
	return m.todayCount, m.todayCountErr
}

func TestNewTradingService(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Config
		logger  ports.Logger
		wantErr bool
	}{
		{
			name: "valid configuration",
			cfg: &config.Config{
				Symbol:    "ETHUSDT",
				Quantity:  0.1,
				StopLoss:  0.02,
				MaxProfit: 0.05,
				MaxOrders: 5,
				Leverage:  10,
			},
			logger:  &mockLogger{},
			wantErr: false,
		},
		{
			name:    "nil config",
			cfg:     nil,
			logger:  &mockLogger{},
			wantErr: true,
		},
		{
			name: "invalid quantity",
			cfg: &config.Config{
				Symbol:    "ETHUSDT",
				Quantity:  0,
				StopLoss:  0.02,
				MaxProfit: 0.05,
				MaxOrders: 5,
			},
			logger:  &mockLogger{},
			wantErr: true,
		},
		{
			name: "invalid stop loss",
			cfg: &config.Config{
				Symbol:    "ETHUSDT",
				Quantity:  0.1,
				StopLoss:  0,
				MaxProfit: 0.05,
				MaxOrders: 5,
			},
			logger:  &mockLogger{},
			wantErr: true,
		},
		{
			name: "invalid max profit",
			cfg: &config.Config{
				Symbol:    "ETHUSDT",
				Quantity:  0.1,
				StopLoss:  0.02,
				MaxProfit: 0,
				MaxOrders: 5,
			},
			logger:  &mockLogger{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exchange := &mockExchange{
				serverTime: time.Now(),
			}
			posRepo := &mockPositionRepo{positions: make(map[string]*domain.Position)}
			tradeRepo := &mockTradeRepo{}

			// Create a real strategy instance for testing
			strat, err := strategy.New(strategy.Config{
				ShortTermMAPeriod: 20,
				LongTermMAPeriod:  50,
				EMAPeriod:         20,
				RSIPeriod:         14,
				RSIOverbought:     70.0,
				RSIOversold:       30.0,
			}, tt.logger)
			if !tt.wantErr {
				require.NoError(t, err)
			}

			service, err := NewTradingService(tt.cfg, tt.logger, exchange, posRepo, tradeRepo, strat)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, service)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, service)
			}
		})
	}
}

func TestTradingService_handleKlineEvent(t *testing.T) {
	// Create base configuration
	cfg := &config.Config{
		Symbol:    "ETHUSDT",
		Quantity:  0.1,
		StopLoss:  0.02,
		MaxProfit: 0.05,
		MaxOrders: 5,
		Leverage:  10,
	}

	tests := []struct {
		name         string
		kline        *domain.Kline
		mockSetup    func(*mockExchange, *mockPositionRepo, *mockTradeRepo, *strategy.Strategy)
		checkState   func(*testing.T, *TradingService)
		wantPosition bool
	}{
		{
			name: "non-final kline - no action",
			kline: &domain.Kline{
				Symbol:    "ETHUSDT",
				Interval:  "1m",
				OpenTime:  time.Now(),
				CloseTime: time.Now().Add(time.Minute),
				Open:      2000.0,
				High:      2010.0,
				Low:       1990.0,
				Close:     2005.0,
				Volume:    100.0,
				IsFinal:   false,
			},
			mockSetup: func(e *mockExchange, p *mockPositionRepo, t *mockTradeRepo, s *strategy.Strategy) {
				// No setup needed for this test
			},
			checkState: func(t *testing.T, s *TradingService) {
				assert.Nil(t, s.currentPosition)
			},
			wantPosition: false,
		},
		{
			name: "final kline - no position - strategy says enter",
			kline: &domain.Kline{
				Symbol:    "ETHUSDT",
				Interval:  "1m",
				OpenTime:  time.Now(),
				CloseTime: time.Now().Add(time.Minute),
				Open:      2000.0,
				High:      2010.0,
				Low:       1990.0,
				Close:     2005.0,
				Volume:    100.0,
				IsFinal:   true,
			},
			mockSetup: func(e *mockExchange, p *mockPositionRepo, t *mockTradeRepo, s *strategy.Strategy) {
				// Setup mock exchange responses
				e.markPrice = 2005.0
				e.orderResponses = map[string]*ports.OrderResponse{
					"market_BUY": {
						OrderID:      1,
						Symbol:       "ETHUSDT",
						OrigQuantity: 0.1,
						ExecutedQty:  0.1,
						AvgPrice:     2005.0,
						Status:       "FILLED",
						Type:         "MARKET",
						Side:         string(domain.Buy),
						Timestamp:    time.Now(),
					},
					"stop_SELL": {
						OrderID:      2,
						Symbol:       "ETHUSDT",
						OrigQuantity: 0.1,
						ExecutedQty:  0.0,
						Price:        1964.9, // 2005 * (1 - 0.02)
						Status:       "NEW",
						Type:         "STOP_MARKET",
						Side:         string(domain.Sell),
						Timestamp:    time.Now(),
					},
					"tp_SELL": {
						OrderID:      3,
						Symbol:       "ETHUSDT",
						OrigQuantity: 0.1,
						ExecutedQty:  0.0,
						Price:        2105.25, // 2005 * (1 + 0.05)
						Status:       "NEW",
						Type:         "TAKE_PROFIT_MARKET",
						Side:         string(domain.Sell),
						Timestamp:    time.Now(),
					},
				}
				e.orderErrors = make(map[string]error)

				// Setup mock trade repo
				t.todayCount = 2 // Below max orders
			},
			checkState: func(t *testing.T, s *TradingService) {
				assert.NotNil(t, s.currentPosition)
				assert.Equal(t, domain.StatusOpen, s.currentPosition.Status)
				assert.Equal(t, 2005.0, s.currentPosition.EntryPrice)
			},
			wantPosition: true,
		},
		{
			name: "final kline - has position - strategy says close",
			kline: &domain.Kline{
				Symbol:    "ETHUSDT",
				Interval:  "1m",
				OpenTime:  time.Now(),
				CloseTime: time.Now().Add(time.Minute),
				Open:      2000.0,
				High:      2010.0,
				Low:       1990.0,
				Close:     2105.25, // Hit take profit
				Volume:    100.0,
				IsFinal:   true,
			},
			mockSetup: func(e *mockExchange, p *mockPositionRepo, t *mockTradeRepo, s *strategy.Strategy) {
				// Setup existing position
				pos := &domain.Position{
					ID:         1,
					Symbol:     "ETHUSDT",
					EntryPrice: 2005.0,
					StopLoss:   1964.9,  // 2005 * (1 - 0.02)
					TakeProfit: 2105.25, // 2005 * (1 + 0.05)
					Status:     domain.StatusOpen,
				}
				p.positions["ETHUSDT"] = pos

				// Setup mock exchange responses for closing
				e.markPrice = 2105.25
				e.orderResponses = map[string]*ports.OrderResponse{
					"market_SELL": {
						OrderID:      4,
						Symbol:       "ETHUSDT",
						OrigQuantity: 0.1,
						ExecutedQty:  0.1,
						AvgPrice:     2105.25,
						Status:       "FILLED",
						Type:         "MARKET",
						Side:         string(domain.Sell),
						Timestamp:    time.Now(),
					},
				}
			},
			checkState: func(t *testing.T, s *TradingService) {
				assert.Nil(t, s.currentPosition)
			},
			wantPosition: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			exchange := &mockExchange{}
			posRepo := &mockPositionRepo{positions: make(map[string]*domain.Position)}
			tradeRepo := &mockTradeRepo{}

			// Use mock strategy instead of real one
			strat := &mockStrategy{
				shouldEnter: tt.name == "final kline - no position - strategy says enter",
				shouldClose: tt.name == "final kline - has position - strategy says close",
				closeReason: domain.CloseReasonTakeProfit,
			}

			service, err := NewTradingService(cfg, logger, exchange, posRepo, tradeRepo, strat)
			require.NoError(t, err)

			if tt.mockSetup != nil {
				tt.mockSetup(exchange, posRepo, tradeRepo, nil) // Pass nil for strategy since we're using mock
			}

			service.handleKlineEvent(tt.kline)

			if tt.checkState != nil {
				tt.checkState(t, service)
			}
		})
	}
}

func TestTradingService_canTrade(t *testing.T) {
	cfg := &config.Config{
		Symbol:    "ETHUSDT",
		Quantity:  0.1,
		StopLoss:  0.02,
		MaxProfit: 0.05,
		MaxOrders: 5,
		Leverage:  10,
	}

	tests := []struct {
		name       string
		mockSetup  func(*TradingService)
		wantCan    bool
		wantReason string
	}{
		{
			name: "can trade - all conditions met",
			mockSetup: func(s *TradingService) {
				s.currentPosition = nil
				s.tradesToday = 0
			},
			wantCan:    true,
			wantReason: "",
		},
		{
			name: "cannot trade - position already open",
			mockSetup: func(s *TradingService) {
				s.currentPosition = &domain.Position{
					ID:     1,
					Symbol: "ETHUSDT",
					Status: domain.StatusOpen,
				}
			},
			wantCan:    false,
			wantReason: "position 1 already open",
		},
		{
			name: "cannot trade - daily limit reached",
			mockSetup: func(s *TradingService) {
				s.currentPosition = nil
				s.tradesToday = 5
			},
			wantCan:    false,
			wantReason: "daily trade limit reached (5/5)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create service with mocks
			logger := &mockLogger{}
			exchange := &mockExchange{}
			posRepo := &mockPositionRepo{positions: make(map[string]*domain.Position)}
			tradeRepo := &mockTradeRepo{}
			strategy := &strategy.Strategy{}

			service, err := NewTradingService(cfg, logger, exchange, posRepo, tradeRepo, strategy)
			require.NoError(t, err)

			// Setup test state
			tt.mockSetup(service)

			// Test canTrade
			can, reason := service.canTrade(context.Background())
			assert.Equal(t, tt.wantCan, can)
			assert.Equal(t, tt.wantReason, reason)
		})
	}
}

func TestTradingService_Start(t *testing.T) {
	tests := []struct {
		name           string
		serverTimeErr  error
		leverageErr    error
		findOpenErr    error
		countTodayErr  error
		klinesErr      error
		klines         []*domain.Kline
		openPosition   *domain.Position
		todayCount     int
		expectedError  bool
		expectedErrMsg string
	}{
		{
			name:          "successful start with no open position",
			klines:        generateTestKlines(100), // Generate 100 test klines
			todayCount:    2,
			expectedError: false,
		},
		{
			name:           "server time sync failure",
			serverTimeErr:  assert.AnError,
			expectedError:  true,
			expectedErrMsg: "failed to set server time",
		},
		{
			name:           "find open position failure",
			findOpenErr:    assert.AnError,
			expectedError:  true,
			expectedErrMsg: "failed to query open position",
		},
		{
			name:           "count today trades failure",
			countTodayErr:  assert.AnError,
			expectedError:  true,
			expectedErrMsg: "failed to count today's trades",
		},
		{
			name:           "insufficient klines data",
			klines:         generateTestKlines(10), // Not enough klines
			expectedError:  true,
			expectedErrMsg: "not enough initial klines loaded",
		},
		{
			name:   "successful start with existing position",
			klines: generateTestKlines(100),
			openPosition: &domain.Position{
				ID:         1,
				Symbol:     "ETHUSDT",
				EntryPrice: 2000.0,
				Status:     domain.StatusOpen,
			},
			todayCount:    1,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			logger := &mockLogger{}
			exchange := &mockExchange{
				serverTimeErr: tt.serverTimeErr,
				leverageErr:   tt.leverageErr,
				klines:        tt.klines,
				klinesErr:     tt.klinesErr,
				serverTime:    time.Now(),
			}
			posRepo := &mockPositionRepo{
				positions:   make(map[string]*domain.Position),
				findOpenErr: tt.findOpenErr,
			}
			if tt.openPosition != nil {
				posRepo.positions[tt.openPosition.Symbol] = tt.openPosition
			}
			tradeRepo := &mockTradeRepo{
				todayCount:    tt.todayCount,
				todayCountErr: tt.countTodayErr,
			}

			// Create a real strategy instance for testing
			strat, err := strategy.New(strategy.Config{
				ShortTermMAPeriod: 20,
				LongTermMAPeriod:  50,
				EMAPeriod:         20,
				RSIPeriod:         14,
				RSIOverbought:     70.0,
				RSIOversold:       30.0,
			}, logger)
			require.NoError(t, err)

			// Create service
			cfg := &config.Config{
				Symbol:    "ETHUSDT",
				Quantity:  0.1,
				StopLoss:  0.02,
				MaxProfit: 0.05,
				MaxOrders: 5,
				Leverage:  10,
			}

			svc, err := NewTradingService(cfg, logger, exchange, posRepo, tradeRepo, strat)
			require.NoError(t, err)

			// Create context that we can cancel
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Start service in a goroutine
			errCh := make(chan error)
			go func() {
				errCh <- svc.Start(ctx)
			}()

			// Wait briefly to allow initialization
			time.Sleep(100 * time.Millisecond)

			// Cancel context to stop the service
			cancel()

			// Get the error result
			err = <-errCh

			if tt.expectedError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrMsg)
			} else {
				assert.NoError(t, err)
			}

			// Verify logs based on scenario
			if tt.serverTimeErr != nil {
				assert.Contains(t, logger.errorMsgs, "Failed to synchronize server time")
			} else if tt.openPosition != nil {
				assert.Contains(t, logger.infoMsgs, "Found existing open position")
			} else {
				assert.Contains(t, logger.infoMsgs, "No existing open position found")
			}
		})
	}
}

// Helper function to generate test klines
func generateTestKlines(count int) []*domain.Kline {
	klines := make([]*domain.Kline, count)
	baseTime := time.Now().Add(-time.Duration(count) * time.Minute)
	basePrice := 2000.0

	for i := 0; i < count; i++ {
		klines[i] = &domain.Kline{
			Symbol:    "ETHUSDT",
			Interval:  "1m",
			OpenTime:  baseTime.Add(time.Duration(i) * time.Minute),
			CloseTime: baseTime.Add(time.Duration(i+1) * time.Minute),
			Open:      basePrice + float64(i),
			High:      basePrice + float64(i) + 10,
			Low:       basePrice + float64(i) - 10,
			Close:     basePrice + float64(i) + 5,
			Volume:    100.0,
			IsFinal:   true,
		}
	}
	return klines
}
