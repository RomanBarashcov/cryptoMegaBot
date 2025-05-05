package ports

import "context"

// Logger defines a standard interface for logging messages and errors.
// This allows injecting different logging implementations (e.g., standard log, zerolog, zap).
type Logger interface {
	// Debug logs a message at Debug level.
	Debug(ctx context.Context, msg string, fields ...map[string]interface{})
	// Info logs a message at Info level.
	Info(ctx context.Context, msg string, fields ...map[string]interface{})
	// Warn logs a message at Warning level.
	Warn(ctx context.Context, msg string, fields ...map[string]interface{})
	// Error logs an error message at Error level.
	Error(ctx context.Context, err error, msg string, fields ...map[string]interface{})
}
