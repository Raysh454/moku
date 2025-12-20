package server

type Config struct {
	// ServerAddr is the HTTP listen address for the API server (CLI uses
	// the orchestrator in-process and does not require the network).
	ServerAddr string
}
