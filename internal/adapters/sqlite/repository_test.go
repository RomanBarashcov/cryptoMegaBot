package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cryptoMegaBot/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLogger implements ports.Logger for testing
type mockLogger struct{}

func (m *mockLogger) Debug(ctx context.Context, msg string, fields ...map[string]interface{}) {}
func (m *mockLogger) Info(ctx context.Context, msg string, fields ...map[string]interface{})  {}
func (m *mockLogger) Warn(ctx context.Context, msg string, fields ...map[string]interface{})  {}
func (m *mockLogger) Error(ctx context.Context, err error, msg string, fields ...map[string]interface{}) {
}
func (m *mockLogger) Fatal(ctx context.Context, err error, msg string, fields ...map[string]interface{}) {
}

// setupTestDB creates a temporary database for testing
func setupTestDB(t *testing.T) (*Repository, func()) {
	t.Helper()

	// Create temporary directory for test database
	tmpDir, err := os.MkdirTemp("", "trading-bot-test-*")
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "test.db")
	repo, err := NewRepository(Config{
		DBPath: dbPath,
		Logger: &mockLogger{},
	})
	require.NoError(t, err)

	// Return cleanup function
	cleanup := func() {
		repo.Close()
		os.RemoveAll(tmpDir)
	}

	return repo, cleanup
}

func TestRepository_CreateAndFindPosition(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*Repository) error
		pos     *domain.Position
		wantErr bool
	}{
		{
			name: "valid position",
			pos: &domain.Position{
				Symbol:     "ETHUSDT",
				EntryPrice: 2000.0,
				Quantity:   1.0,
				Leverage:   4,
				StopLoss:   1900.0,
				TakeProfit: 2200.0,
				EntryTime:  time.Now(),
				Status:     domain.StatusOpen,
			},
			wantErr: false,
		},
		{
			name: "duplicate open position",
			setup: func(r *Repository) error {
				// Create initial position
				pos := &domain.Position{
					Symbol:     "ETHUSDT",
					EntryPrice: 2000.0,
					Quantity:   1.0,
					Leverage:   4,
					StopLoss:   1900.0,
					TakeProfit: 2200.0,
					EntryTime:  time.Now(),
					Status:     domain.StatusOpen,
				}
				_, err := r.Create(context.Background(), pos)
				return err
			},
			pos: &domain.Position{
				Symbol:     "ETHUSDT",
				EntryPrice: 2100.0,
				Quantity:   1.0,
				Leverage:   4,
				StopLoss:   2000.0,
				TakeProfit: 2300.0,
				EntryTime:  time.Now(),
				Status:     domain.StatusOpen,
			},
			wantErr: true, // Should fail due to trigger
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, cleanup := setupTestDB(t)
			defer cleanup()

			ctx := context.Background()

			// Run setup if provided
			if tt.setup != nil {
				err := tt.setup(repo)
				require.NoError(t, err)
			}

			// Create position
			id, err := repo.Create(ctx, tt.pos)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Greater(t, id, int64(0))

			// Find position
			found, err := repo.FindByID(ctx, id)
			require.NoError(t, err)
			require.NotNil(t, found)

			// Verify fields
			assert.Equal(t, tt.pos.Symbol, found.Symbol)
			assert.Equal(t, tt.pos.EntryPrice, found.EntryPrice)
			assert.Equal(t, tt.pos.Quantity, found.Quantity)
			assert.Equal(t, tt.pos.Leverage, found.Leverage)
			assert.Equal(t, tt.pos.StopLoss, found.StopLoss)
			assert.Equal(t, tt.pos.TakeProfit, found.TakeProfit)
			assert.Equal(t, tt.pos.Status, found.Status)
		})
	}
}

func TestRepository_UpdatePosition(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*Repository) error
		pos     *domain.Position
		update  func(*domain.Position)
		wantErr bool
	}{
		{
			name: "close position",
			setup: func(r *Repository) error {
				pos := &domain.Position{
					Symbol:     "ETHUSDT",
					EntryPrice: 2000.0,
					Quantity:   1.0,
					Leverage:   4,
					StopLoss:   1900.0,
					TakeProfit: 2200.0,
					EntryTime:  time.Now(),
					Status:     domain.StatusOpen,
				}
				id, err := r.Create(context.Background(), pos)
				if err != nil {
					return err
				}
				pos.ID = id
				return nil
			},
			pos: &domain.Position{
				Symbol:     "ETHUSDT",
				EntryPrice: 2000.0,
				Quantity:   1.0,
				Leverage:   4,
				StopLoss:   1900.0,
				TakeProfit: 2200.0,
				EntryTime:  time.Now(),
				Status:     domain.StatusOpen,
			},
			update: func(p *domain.Position) {
				p.Status = domain.StatusClosed
				p.ExitPrice = 2100.0
				p.ExitTime = time.Now()
				p.PNL = 100.0
				p.CloseReason = domain.CloseReasonTakeProfit
			},
			wantErr: false,
		},
		{
			name: "update non-existent position",
			pos: &domain.Position{
				ID:         999,
				Symbol:     "ETHUSDT",
				EntryPrice: 2000.0,
				Quantity:   1.0,
				Leverage:   4,
				StopLoss:   1900.0,
				TakeProfit: 2200.0,
				EntryTime:  time.Now(),
				Status:     domain.StatusClosed,
			},
			update: func(p *domain.Position) {
				p.ExitPrice = 2100.0
				p.ExitTime = time.Now()
				p.PNL = 100.0
				p.CloseReason = domain.CloseReasonTakeProfit
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, cleanup := setupTestDB(t)
			defer cleanup()

			ctx := context.Background()

			// Run setup if provided
			if tt.setup != nil {
				err := tt.setup(repo)
				require.NoError(t, err)

				// For close position test, get the position ID from the database
				if tt.name == "close position" {
					openPos, err := repo.FindOpenBySymbol(ctx, tt.pos.Symbol)
					require.NoError(t, err)
					require.NotNil(t, openPos)
					tt.pos.ID = openPos.ID
				}
			}

			// Apply update
			tt.update(tt.pos)

			// Update position
			err := repo.Update(ctx, tt.pos)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify update
			found, err := repo.FindByID(ctx, tt.pos.ID)
			require.NoError(t, err)
			require.NotNil(t, found)

			assert.Equal(t, tt.pos.Status, found.Status)
			assert.Equal(t, tt.pos.ExitPrice, found.ExitPrice)
			assert.Equal(t, tt.pos.PNL, found.PNL)
			assert.Equal(t, tt.pos.CloseReason, found.CloseReason)
		})
	}
}

func TestRepository_FindOpenBySymbol(t *testing.T) {
	tests := []struct {
		name    string
		symbol  string
		setup   func(*Repository) error
		want    *domain.Position
		wantErr bool
	}{
		{
			name:   "find existing open position",
			symbol: "ETHUSDT",
			setup: func(r *Repository) error {
				pos := &domain.Position{
					Symbol:     "ETHUSDT",
					EntryPrice: 2000.0,
					Quantity:   1.0,
					Leverage:   4,
					StopLoss:   1900.0,
					TakeProfit: 2200.0,
					EntryTime:  time.Now(),
					Status:     domain.StatusOpen,
				}
				_, err := r.Create(context.Background(), pos)
				return err
			},
			want: &domain.Position{
				Symbol:     "ETHUSDT",
				EntryPrice: 2000.0,
				Quantity:   1.0,
				Leverage:   4,
				StopLoss:   1900.0,
				TakeProfit: 2200.0,
				Status:     domain.StatusOpen,
			},
			wantErr: false,
		},
		{
			name:    "no open position",
			symbol:  "BTCUSDT",
			setup:   func(r *Repository) error { return nil },
			want:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, cleanup := setupTestDB(t)
			defer cleanup()

			ctx := context.Background()

			// Setup test data
			err := tt.setup(repo)
			require.NoError(t, err)

			// Find position
			got, err := repo.FindOpenBySymbol(ctx, tt.symbol)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			if tt.want == nil {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got)
			assert.Equal(t, tt.want.Symbol, got.Symbol)
			assert.Equal(t, tt.want.EntryPrice, got.EntryPrice)
			assert.Equal(t, tt.want.Quantity, got.Quantity)
			assert.Equal(t, tt.want.Leverage, got.Leverage)
			assert.Equal(t, tt.want.StopLoss, got.StopLoss)
			assert.Equal(t, tt.want.TakeProfit, got.TakeProfit)
			assert.Equal(t, tt.want.Status, got.Status)
		})
	}
}

func TestRepository_GetTotalProfit(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*Repository) error
		want    float64
		wantErr bool
	}{
		{
			name: "multiple closed positions",
			setup: func(r *Repository) error {
				positions := []*domain.Position{
					{
						Symbol:     "ETHUSDT",
						EntryPrice: 2000.0,
						ExitPrice:  2100.0,
						Quantity:   1.0,
						Leverage:   4,
						StopLoss:   1900.0,
						TakeProfit: 2200.0,
						EntryTime:  time.Now(),
						ExitTime:   time.Now(),
						Status:     domain.StatusClosed,
						PNL:        100.0,
					},
					{
						Symbol:     "BTCUSDT",
						EntryPrice: 40000.0,
						ExitPrice:  41000.0,
						Quantity:   0.1,
						Leverage:   2,
						StopLoss:   39000.0,
						TakeProfit: 42000.0,
						EntryTime:  time.Now(),
						ExitTime:   time.Now(),
						Status:     domain.StatusClosed,
						PNL:        100.0,
					},
				}

				for _, pos := range positions {
					id, err := r.Create(context.Background(), pos)
					if err != nil {
						return err
					}
					pos.ID = id
					if err := r.Update(context.Background(), pos); err != nil {
						return err
					}
				}
				return nil
			},
			want:    200.0, // Sum of both positions' PNL
			wantErr: false,
		},
		{
			name:    "no closed positions",
			setup:   func(r *Repository) error { return nil },
			want:    0.0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, cleanup := setupTestDB(t)
			defer cleanup()

			ctx := context.Background()

			// Setup test data
			err := tt.setup(repo)
			require.NoError(t, err)

			// Get total profit
			got, err := repo.GetTotalProfit(ctx)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
