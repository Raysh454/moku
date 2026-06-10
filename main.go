package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/server"
)

// Default listen host and port. The server binds to loopback by default so a
// fresh checkout never exposes the API on every interface. Opt into bind-all
// with MOKU_HOST=0.0.0.0 (or a positional host arg).
const (
	defaultListenHost = "127.0.0.1"
	defaultListenPort = 8080
)

// resolveListenAddr derives the HTTP listen address from positional args and
// environment, with precedence: positional args > MOKU_HOST/MOKU_PORT env >
// loopback defaults. args is os.Args[1:] ([host] [port]); getenv is injected so
// the precedence is testable without mutating the process environment.
func resolveListenAddr(args []string, getenv func(string) string) (string, error) {
	host := defaultListenHost
	if envHost := getenv("MOKU_HOST"); envHost != "" {
		host = envHost
	}
	if len(args) >= 1 && args[0] != "" {
		host = args[0]
	}

	port := defaultListenPort
	if envPort := getenv("MOKU_PORT"); envPort != "" {
		p, err := parsePort(envPort)
		if err != nil {
			return "", fmt.Errorf("invalid MOKU_PORT: %w", err)
		}
		port = p
	}
	if len(args) >= 2 && args[1] != "" {
		p, err := parsePort(args[1])
		if err != nil {
			return "", fmt.Errorf("invalid port argument: %w", err)
		}
		port = p
	}

	return fmt.Sprintf("%s:%d", host, port), nil
}

// parsePort validates that raw is a TCP port in the range 1..65535.
func parsePort(raw string) (int, error) {
	p, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%q is not a number", raw)
	}
	if p <= 0 || p >= 65536 {
		return 0, fmt.Errorf("%d is out of range (1..65535)", p)
	}
	return p, nil
}

func main() {
	listenAddr, err := resolveListenAddr(os.Args[1:], os.Getenv)
	if err != nil {
		log.Fatalf("resolving listen address: %v", err)
	}

	// App config
	cfg := app.DefaultConfig()

	// Logger
	logger := logging.NewStdoutLogger("main")

	srv, err := server.NewServer(server.Config{
		ListenAddr: listenAddr,
		AppConfig:  cfg,
		Logger:     logger,
	})
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}
	defer srv.Close()

	httpServer := srv.HTTPServer()

	// Graceful shutdown
	idleConnsClosed := make(chan struct{})

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// 1. Begin shutdown: reject new jobs, cancel running ones, and close the
		//    event broker so open SSE streams return. The HTTP server has no
		//    write timeout, so without closing the broker first, Shutdown would
		//    block on every open stream until the context deadline.
		log.Println("Beginning graceful shutdown (canceling jobs, closing event streams)...")
		srv.BeginShutdown()

		// 2. Drain in-flight HTTP requests while the registry and per-site DBs are
		//    still open, so requests never hit a closed database.
		log.Println("Shutting down HTTP server...")
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}

		// 3. Close: wait for job goroutines to unwind, then close site components
		//    and the registry DB.
		log.Println("Closing orchestrator and components...")
		srv.Close()

		close(idleConnsClosed)
	}()

	log.Printf("Listening on %s", listenAddr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("ListenAndServe: %v", err)
	}

	<-idleConnsClosed
}
