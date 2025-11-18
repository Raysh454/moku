package interfaces

// Logger is a deliberately small, framework-agnostic logging interface.
// Keep implementations outside internal packages so you can swap in any logger.
type Logger interface {
	// Debug logs a debug-level message.
	Debug(msg string, fields ...Field)

	// Info logs an informational message.
	Info(msg string, fields ...Field)

	// Warn logs a warning.
	Warn(msg string, fields ...Field)

	// Error logs an error.
	Error(msg string, fields ...Field)

	// With returns a child logger with persistent fields.
	With(fields ...Field) Logger
}

// Field is a simple key/value pair for structured logging fields.
type Field struct {
	Key   string
	Value interface{}
}
