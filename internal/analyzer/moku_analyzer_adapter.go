package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/webclient"
)

// MokuAnalyzerAdapter integrates with the moku-analyzer FastAPI service.
// This adapter implements the Analyzer interface by making HTTP calls to
// the external moku-analyzer service for vulnerability scanning.
type MokuAnalyzerAdapter struct {
	baseURL    string
	httpClient *http.Client
	logger     logging.Logger
}

// NewMokuAnalyzerAdapter creates a new analyzer that connects to the moku-analyzer service.
func NewMokuAnalyzerAdapter(cfg *app.Config, logger logging.Logger) (Analyzer, error) {
	baseURL := cfg.AnalyzerURL
	if baseURL == "" {
		baseURL = "http://localhost:8080" // Default to local service
	}

	componentLogger := logger.With(logging.Field{Key: "component", Value: "moku_analyzer_adapter"})

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	adapter := &MokuAnalyzerAdapter{
		baseURL:    baseURL,
		httpClient: httpClient,
		logger:     componentLogger,
	}

	// Test connection
	if err := adapter.testConnection(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to connect to moku-analyzer service: %w", err)
	}

	componentLogger.Info("connected to moku-analyzer service", logging.Field{Key: "url", Value: baseURL})
	return adapter, nil
}

// testConnection verifies the moku-analyzer service is available
func (a *MokuAnalyzerAdapter) testConnection(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", a.baseURL+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

// SubmitScan submits a URL for analysis to the moku-analyzer service
func (a *MokuAnalyzerAdapter) SubmitScan(ctx context.Context, req *ScanRequest) (string, error) {
	if req == nil {
		return "", fmt.Errorf("SubmitScan: nil request")
	}

	a.logger.Info("submitting scan to moku-analyzer",
		logging.Field{Key: "url", Value: req.URL})

	// Prepare request payload
	payload := map[string]interface{}{
		"method": "url",
		"url":    req.URL,
		// Use default adapter or check options
		"adapter": req.Options["adapter"],
	}

	if payload["adapter"] == "" {
		payload["adapter"] = "builtin" // Default adapter
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/scan", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to submit scan: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("scan submission failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result struct {
		JobID string `json:"job_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	a.logger.Info("scan submitted successfully",
		logging.Field{Key: "job_id", Value: result.JobID})

	return result.JobID, nil
}

// GetScan retrieves scan results from the moku-analyzer service
func (a *MokuAnalyzerAdapter) GetScan(ctx context.Context, jobID string) (*ScanResult, error) {
	if jobID == "" {
		return nil, fmt.Errorf("GetScan: empty job ID")
	}

	a.logger.Info("retrieving scan results",
		logging.Field{Key: "job_id", Value: jobID})

	// Make HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/scan/%s", a.baseURL, jobID), nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get scan results: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get results with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResult struct {
		ID            string `json:"id"`
		Status        string `json:"status"`
		Vulnerabilities []struct {
			Type        string `json:"type"`
			Severity    string `json:"severity"`
			Description string `json:"description"`
			Location    string `json:"location"`
			Evidence    string `json:"evidence"`
			Meta        map[string]interface{} `json:"meta"`
		} `json:"vulnerabilities"`
		Error string `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResult); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert to Moku's ScanResult format
	result := &ScanResult{
		JobID:       apiResult.ID,
		Status:      a.mapStatus(apiResult.Status),
		URL:         "", // We don't have this in the response, could be added to API
		SubmittedAt: time.Now(), // Approximate
		Meta:        make(map[string]any),
	}

	if apiResult.Error != "" {
		result.Error = apiResult.Error
		result.Status = "failed"
	} else if result.Status == "done" {
		result.CompletedAt = &time.Time{}
		*result.CompletedAt = time.Now()

		// Convert vulnerabilities to Moku's format
		if len(apiResult.Vulnerabilities) > 0 {
			// Create a score result based on vulnerabilities
			score := &assessor.ScoreResult{
				Score:       a.calculateScore(apiResult.Vulnerabilities),
				Category:    a.determineCategory(apiResult.Vulnerabilities),
				Description: fmt.Sprintf("Found %d vulnerabilities", len(apiResult.Vulnerabilities)),
				Details:     make(map[string]interface{}),
			}

			// Add vulnerability details
			vulns := make([]map[string]interface{}, len(apiResult.Vulnerabilities))
			for i, vuln := range apiResult.Vulnerabilities {
				vulns[i] = map[string]interface{}{
					"type":        vuln.Type,
					"severity":    vuln.Severity,
					"description": vuln.Description,
					"location":    vuln.Location,
					"evidence":    vuln.Evidence,
					"meta":        vuln.Meta,
				}
			}
			score.Details["vulnerabilities"] = vulns

			result.Score = score
		}
	}

	return result, nil
}

// ScanAndWait submits a scan and polls for completion
func (a *MokuAnalyzerAdapter) ScanAndWait(ctx context.Context, req *ScanRequest, timeoutSec int, pollIntervalSec int) (*ScanResult, error) {
	jobID, err := a.SubmitScan(ctx, req)
	if err != nil {
		return nil, err
	}

	timeout := time.Duration(timeoutSec) * time.Second
	pollInterval := time.Duration(pollIntervalSec) * time.Second
	deadline := time.Now().Add(timeout)

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("scan timeout after %d seconds", timeoutSec)
		}

		result, err := a.GetScan(ctx, jobID)
		if err != nil {
			return nil, err
		}

		if result.Status == "done" || result.Status == "failed" {
			return result, nil
		}

		a.logger.Debug("scan still running, waiting",
			logging.Field{Key: "job_id", Value: jobID},
			logging.Field{Key: "status", Value: result.Status})

		time.Sleep(pollInterval)
	}
}

// Health checks the moku-analyzer service health
func (a *MokuAnalyzerAdapter) Health(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", a.baseURL+"/health", nil)
	if err != nil {
		return "", err
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	var health struct {
		Status   string   `json:"status"`
		Adapters []string `json:"adapters"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return "", fmt.Errorf("failed to parse health response: %w", err)
	}

	return fmt.Sprintf("moku-analyzer healthy - status: %s, adapters: %v", health.Status, health.Adapters), nil
}

// Close releases resources
func (a *MokuAnalyzerAdapter) Close() error {
	// HTTP client doesn't need explicit closing
	a.logger.Info("moku-analyzer adapter closed")
	return nil
}

// Helper functions

// mapStatus converts moku-analyzer status to Moku status
func (a *MokuAnalyzerAdapter) mapStatus(apiStatus string) string {
	switch apiStatus {
	case "done":
		return "completed"
	case "failed":
		return "failed"
	case "running":
		return "running"
	case "pending":
		return "pending"
	default:
		return "unknown"
	}
}

// calculateScore computes a score based on vulnerabilities found
func (a *MokuAnalyzerAdapter) calculateScore(vulns []struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Location    string `json:"location"`
	Evidence    string `json:"evidence"`
	Meta        map[string]interface{} `json:"meta"`
}) float64 {
	if len(vulns) == 0 {
		return 100.0 // Perfect score if no vulnerabilities
	}

	// Simple scoring: deduct points based on severity
	score := 100.0
	for _, vuln := range vulns {
		switch vuln.Severity {
		case "critical":
			score -= 25.0
		case "high":
			score -= 15.0
		case "medium":
			score -= 8.0
		case "low":
			score -= 3.0
		case "info":
			score -= 1.0
		}
	}

	if score < 0 {
		score = 0
	}

	return score
}

// determineCategory determines the overall category based on vulnerabilities
func (a *MokuAnalyzerAdapter) determineCategory(vulns []struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Location    string `json:"location"`
	Evidence    string `json:"evidence"`
	Meta        map[string]interface{} `json:"meta"`
}) string {
	if len(vulns) == 0 {
		return "secure"
	}

	hasCritical := false
	hasHigh := false

	for _, vuln := range vulns {
		if vuln.Severity == "critical" {
			hasCritical = true
		} else if vuln.Severity == "high" {
			hasHigh = true
		}
	}

	if hasCritical {
		return "critical"
	} else if hasHigh {
		return "high"
	} else {
		return "medium"
	}
}