package app

import (
	"os"
	"strconv"
)

// Config contains a minimal set of runtime configuration options required by
// internal modules during initial development. We intentionally keep this small
// for the dev branch — add more fields later as wiring requires them.
type Config struct {
	// ServerAddr is the HTTP listen address for the API server (CLI uses
	// the orchestrator in-process and does not require the network).
	ServerAddr string

	// StorageRoot is the base path where snapshots and blob files are kept.
	StorageRoot string

	// DBPath is the path to the SQLite index file.
	DBPath string

	// SchedulerGlobalConcurrency is the default number of concurrently running jobs.
	SchedulerGlobalConcurrency int

	// FetcherConcurrency is the number of parallel fetch worker slots.
	FetcherConcurrency int

	// WebClientBackend selects which implementation of WebClient to use.
	// Supported values:
	//   "nethttp"   - use net/http-based client (default)
	//   "chromedp"  - use chromedp-based client (experimental)
	WebClientBackend string
}

// DefaultConfig returns a Config populated with sensible development defaults.
func DefaultConfig() *Config {
	return &Config{
		ServerAddr:                 "127.0.0.1:8080",
		StorageRoot:                "./data",
		DBPath:                     "./data/moku.db",
		SchedulerGlobalConcurrency: 4,
		FetcherConcurrency:         8,
		WebClientBackend:           "nethttp",
	}
}

// LoadConfigFromEnv loads the minimal Config from environment variables,
// falling back to defaults when variables are not set or malformed.
//
// Supported environment variables:
//   MOKU_SERVER_ADDR
//   MOKU_STORAGE_ROOT
//   MOKU_DB_PATH
//   MOKU_SCHED_CONCURRENCY
//   MOKU_FETCHER_CONCURRENCY
//   MOKU_WEBCLIENT_BACKEND  (nethttp|chromedp)
func LoadConfigFromEnv() *Config {
	cfg := DefaultConfig()

	if v := os.Getenv("MOKU_SERVER_ADDR"); v != "" {
		cfg.ServerAddr = v
	}
	if v := os.Getenv("MOKU_STORAGE_ROOT"); v != "" {
		cfg.StorageRoot = v
	}
	if v := os.Getenv("MOKU_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("MOKU_WEBCLIENT_BACKEND"); v != "" {
		cfg.WebClientBackend = v
	}
	if v := os.Getenv("MOKU_SCHED_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.SchedulerGlobalConcurrency = n
		}
	}
	if v := os.Getenv("MOKU_FETCHER_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.FetcherConcurrency = n
		}
	}

	return cfg
}
