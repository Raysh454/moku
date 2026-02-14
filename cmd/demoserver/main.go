// Command demoserver starts the Moku demo server for demonstrating tracking capabilities.
// Usage: go run ./cmd/demoserver [port]
// Default port: 9999
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/raysh454/moku/internal/demoserver"
)

func main() {
	cfg := demoserver.DefaultConfig()

	// Optional: custom port from command line
	if len(os.Args) > 1 {
		port, err := strconv.Atoi(os.Args[1])
		if err != nil || port < 1 || port > 65535 {
			log.Fatalf("Invalid port: %s", os.Args[1])
		}
		cfg.Port = port
	}

	fmt.Println("===========================================")
	fmt.Println("   Moku Demo Server - Tracking Demo")
	fmt.Println("===========================================")
	fmt.Println()
	fmt.Println("This server demonstrates the tracking capabilities")
	fmt.Println("of the Moku security scanner by providing pages")
	fmt.Println("with multiple versions that can be switched on-the-fly.")
	fmt.Println()
	fmt.Println("Features tracked:")
	fmt.Println("  - Forms (login, admin, upload, contact)")
	fmt.Println("  - Form inputs (password, file, hidden, CSRF tokens)")
	fmt.Println("  - Security headers (CSP, X-Frame-Options, HSTS)")
	fmt.Println("  - Cookies (HttpOnly, Secure, SameSite)")
	fmt.Println("  - Scripts (inline and external)")
	fmt.Println()

	server := demoserver.NewDemoServer(cfg)
	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
