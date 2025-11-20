package analyzer

import (
	"context"
	"fmt"
	"net/http"

	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/interfaces"
	"github.com/raysh454/moku/internal/model"
	"github.com/raysh454/moku/internal/webclient"
)

// DefaultAnalyzer is a basic analyzer implementation that uses the webclient
// to fetch and analyze web content.
type DefaultAnalyzer struct {
	client interfaces.WebClient
	logger interfaces.Logger
}

// NewDefaultAnalyzer creates a new DefaultAnalyzer instance using the nethttp webclient.
// It accepts an optional *http.Client to allow callers to inject a preconfigured HTTP client.
func NewDefaultAnalyzer(cfg *app.Config, logger interfaces.Logger, httpClient *http.Client) (*DefaultAnalyzer, error) {
	// Create component-scoped logger
	componentLogger := logger.With(interfaces.Field{Key: "component", Value: "default_analyzer"})
	
	// Create webclient using the new constructor that accepts *http.Client
	client, err := webclient.NewNetHTTPClient(cfg, componentLogger, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create webclient: %w", err)
	}
	
	componentLogger.Info("created default analyzer")
	
	return &DefaultAnalyzer{
		client: client,
		logger: componentLogger,
	}, nil
}

// Analyze performs a basic analysis on the given URL by fetching its content.
func (da *DefaultAnalyzer) Analyze(ctx context.Context, url string) (*model.Response, error) {
	da.logger.Info("analyzing URL", interfaces.Field{Key: "url", Value: url})
	
	resp, err := da.client.Get(ctx, url)
	if err != nil {
		da.logger.Error("failed to fetch URL",
			interfaces.Field{Key: "url", Value: url},
			interfaces.Field{Key: "error", Value: err.Error()})
		return nil, fmt.Errorf("analyze: %w", err)
	}
	
	da.logger.Info("successfully analyzed URL",
		interfaces.Field{Key: "url", Value: url},
		interfaces.Field{Key: "status_code", Value: resp.StatusCode})
	
	return resp, nil
}

// Close closes the analyzer and releases any resources.
func (da *DefaultAnalyzer) Close() error {
	da.logger.Info("closing default analyzer")
	return da.client.Close()
}
