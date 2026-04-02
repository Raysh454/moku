// Example usage of the MokuAnalyzerAdapter
//
// This example shows how to integrate the moku-analyzer service
// with the Moku platform for vulnerability scanning.

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/raysh454/moku/internal/analyzer"
	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/logging"
)

func main() {
	// Create app config with analyzer URL
	cfg := app.DefaultConfig()
	cfg.AnalyzerURL = "http://localhost:8080" // Your moku-analyzer service URL

	// Create logger
	logger := logging.NewStdoutLogger("example")

	// Create the analyzer adapter
	analyzerAdapter, err := analyzer.NewMokuAnalyzerAdapter(cfg, logger)
	if err != nil {
		log.Fatalf("Failed to create analyzer: %v", err)
	}
	defer analyzerAdapter.Close()

	// Example scan request
	req := &analyzer.ScanRequest{
		URL: "https://example.com",
		Options: map[string]string{
			"adapter": "nuclei", // Use nuclei scanner
		},
	}

	// Submit scan and wait for results
	fmt.Println("Submitting scan...")
	result, err := analyzerAdapter.ScanAndWait(context.Background(), req, 300, 5)
	if err != nil {
		log.Fatalf("Scan failed: %v", err)
	}

	// Print results
	fmt.Printf("Scan completed!\n")
	fmt.Printf("Status: %s\n", result.Status)
	fmt.Printf("URL: %s\n", result.URL)
	if result.Score != nil {
		fmt.Printf("Score: %.1f (%s)\n", result.Score.Score, result.Score.Category)
		fmt.Printf("Description: %s\n", result.Score.Description)
	}
	if result.Error != "" {
		fmt.Printf("Error: %s\n", result.Error)
	}
}