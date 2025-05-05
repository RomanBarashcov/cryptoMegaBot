package logger

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
)

// StdLogger implements the ports.Logger interface using the standard log package.
type StdLogger struct {
	logger *log.Logger
	level  LogLevel
}

// LogLevel defines the logging level.
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

// String returns the string representation of the LogLevel.
func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel converts a string level to LogLevel.
func ParseLevel(levelStr string) LogLevel {
	switch strings.ToUpper(levelStr) {
	case "DEBUG":
		return LevelDebug
	case "INFO":
		return LevelInfo
	case "WARN", "WARNING":
		return LevelWarn
	case "ERROR":
		return LevelError
	default:
		return LevelInfo // Default to Info
	}
}

// NewStdLogger creates a new standard logger.
// It logs to os.Stderr by default.
func NewStdLogger(level LogLevel) *StdLogger {
	return &StdLogger{
		logger: log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds), // Include microseconds for better timing
		level:  level,
	}
}

func (l *StdLogger) log(ctx context.Context, level LogLevel, msg string, err error, fields ...map[string]interface{}) {
	if level < l.level {
		return // Skip logging if the level is below the configured threshold
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] %s", level.String(), msg))

	if err != nil {
		sb.WriteString(fmt.Sprintf(" | error: %v", err))
	}

	// Simple key-value pair formatting for fields
	if len(fields) > 0 && fields[0] != nil {
		sb.WriteString(" |")
		for k, v := range fields[0] {
			sb.WriteString(fmt.Sprintf(" %s=%v", k, v))
		}
	}

	l.logger.Println(sb.String())
}

// Debug logs a message at Debug level.
func (l *StdLogger) Debug(ctx context.Context, msg string, fields ...map[string]interface{}) {
	l.log(ctx, LevelDebug, msg, nil, fields...)
}

// Info logs a message at Info level.
func (l *StdLogger) Info(ctx context.Context, msg string, fields ...map[string]interface{}) {
	l.log(ctx, LevelInfo, msg, nil, fields...)
}

// Warn logs a message at Warning level.
func (l *StdLogger) Warn(ctx context.Context, msg string, fields ...map[string]interface{}) {
	l.log(ctx, LevelWarn, msg, nil, fields...)
}

// Error logs an error message at Error level.
func (l *StdLogger) Error(ctx context.Context, err error, msg string, fields ...map[string]interface{}) {
	l.log(ctx, LevelError, msg, err, fields...)
}
