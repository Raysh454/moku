package interfaces

import "fmt"

// TestLogger is a simple logger implementation for testing purposes.
// It writes to stdout and can be used in tests where a Logger interface is required.
type TestLogger struct {
	verbose bool
}

// NewTestLogger creates a new test logger.
func NewTestLogger(verbose bool) *TestLogger {
	return &TestLogger{verbose: verbose}
}

func (tl *TestLogger) Debug(msg string, fields ...Field) {
	if tl.verbose {
		fmt.Printf("[DEBUG] %s %v\n", msg, fields)
	}
}

func (tl *TestLogger) Info(msg string, fields ...Field) {
	if tl.verbose {
		fmt.Printf("[INFO] %s %v\n", msg, fields)
	}
}

func (tl *TestLogger) Warn(msg string, fields ...Field) {
	fmt.Printf("[WARN] %s %v\n", msg, fields)
}

func (tl *TestLogger) Error(msg string, fields ...Field) {
	fmt.Printf("[ERROR] %s %v\n", msg, fields)
}

func (tl *TestLogger) With(fields ...Field) Logger {
	return tl
}
