package ports

import "errors"

// Standard application-level errors.
// Adapters should wrap underlying infrastructure errors with these standard errors.
var (
	// General Errors
	ErrUnknown            = errors.New("unknown error occurred")
	ErrInvalidRequest     = errors.New("invalid request parameters or format")
	ErrNotFound           = errors.New("resource not found")
	ErrTimeout            = errors.New("operation timed out")
	ErrContextCanceled    = errors.New("operation canceled via context")
	ErrPermissionDenied   = errors.New("permission denied")
	ErrConfigurationError = errors.New("invalid or missing configuration")

	// Exchange Specific Errors
	ErrExchangeUnavailable  = errors.New("exchange API is unavailable")
	ErrConnectionFailed     = errors.New("failed to connect to the exchange")
	ErrRateLimited          = errors.New("API rate limit exceeded")
	ErrAuthenticationFailed = errors.New("exchange authentication failed (check API keys)")
	ErrInvalidAPIKeys       = errors.New("invalid API keys or permissions")
	ErrInsufficientFunds    = errors.New("insufficient funds for operation")
	ErrOrderNotFound        = errors.New("order not found on the exchange")
	ErrPositionNotFound     = errors.New("position not found on the exchange")
	ErrOrderPlacementFailed = errors.New("failed to place order")
	ErrOrderCancelFailed    = errors.New("failed to cancel order")

	// Database Specific Errors
	ErrDuplicateEntry = errors.New("database record already exists")
	ErrDBConnection   = errors.New("database connection error")
	ErrQueryFailed    = errors.New("database query failed")
	ErrUpdateFailed   = errors.New("database update failed")
	ErrDeleteFailed   = errors.New("database delete failed")
)
