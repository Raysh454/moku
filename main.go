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

func main() {
	// Defaults
	host := "0.0.0.0"
	port := 8080

	// Optional args: [host] [port]
	args := os.Args[1:]
	if len(args) >= 1 && args[0] != "" {
		host = args[0]
	}
	if len(args) >= 2 && args[1] != "" {
		if p, err := strconv.Atoi(args[1]); err == nil && p > 0 && p < 65536 {
			port = p
		} else {
			log.Fatalf("invalid port: %q", args[1])
		}
	}

	listenAddr := fmt.Sprintf("%s:%d", host, port)

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

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}

		srv.Close()

		close(idleConnsClosed)
	}()

	log.Printf("Listening on %s", listenAddr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("ListenAndServe: %v", err)
	}

	<-idleConnsClosed
}
