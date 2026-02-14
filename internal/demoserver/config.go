package demoserver

// Config holds configuration for the demo server.
type Config struct {
	// Port is the port on which the demo server listens.
	Port int

	// InitialVersion is the starting version for all pages (default: 1).
	InitialVersion int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Port:           9999,
		InitialVersion: 1,
	}
}
