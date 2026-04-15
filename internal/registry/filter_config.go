package registry

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/raysh454/moku/internal/filter"
	"github.com/raysh454/moku/internal/logging"
)

// LoadFilterConfig loads and merges filter configuration for a website.
// It combines the global config with website-specific rules from the database
// and the website's config JSON column.
func (r *Registry) LoadFilterConfig(ctx context.Context, websiteID string, globalConfig *filter.Config) (*filter.Config, error) {
	// Load website config from database
	websiteConfig, err := r.loadWebsiteFilterConfig(ctx, websiteID)
	if err != nil {
		r.logger.Debug("Failed to load website filter config, using empty config",
			logging.Field{Key: "error", Value: err.Error()})
		websiteConfig = &filter.Config{}
	}

	// Load filter rules from database
	rules, err := r.ListEnabledFilterRules(ctx, websiteID)
	if err != nil {
		r.logger.Debug("Failed to load filter rules, using empty rules",
			logging.Field{Key: "error", Value: err.Error()})
		rules = []filter.Rule{}
	}

	// Convert rules to config
	rulesConfig := filter.RulesToConfig(rules)

	// Merge: global -> website config -> rules
	merged := filter.MergeConfigs(globalConfig, websiteConfig, rulesConfig)

	return merged, nil
}

// loadWebsiteFilterConfig loads the filter configuration from the website's config JSON column.
func (r *Registry) loadWebsiteFilterConfig(ctx context.Context, websiteID string) (*filter.Config, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT config FROM websites WHERE id = ?`,
		websiteID,
	)

	var configJSON string
	if err := row.Scan(&configJSON); err != nil {
		return nil, fmt.Errorf("query website config: %w", err)
	}

	if configJSON == "" || configJSON == "{}" {
		return &filter.Config{}, nil
	}

	// Parse the JSON config
	var websiteFilterCfg filter.WebsiteConfig
	if err := json.Unmarshal([]byte(configJSON), &websiteFilterCfg); err != nil {
		// Not a valid filter config, return empty
		return &filter.Config{}, nil
	}

	return websiteFilterCfg.ToConfig(), nil
}

// UpdateWebsiteFilterConfig updates the filter configuration in the website's config JSON column.
func (r *Registry) UpdateWebsiteFilterConfig(ctx context.Context, websiteID string, cfg *filter.WebsiteConfig) error {
	configJSON, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal filter config: %w", err)
	}

	result, err := r.db.ExecContext(ctx,
		`UPDATE websites SET config = ? WHERE id = ?`,
		string(configJSON), websiteID,
	)
	if err != nil {
		return fmt.Errorf("update website config: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return ErrWebsiteNotFound
	}

	return nil
}

// GetWebsiteFilterConfig returns the filter configuration from the website's config JSON column.
func (r *Registry) GetWebsiteFilterConfig(ctx context.Context, websiteID string) (*filter.WebsiteConfig, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT config FROM websites WHERE id = ?`,
		websiteID,
	)

	var configJSON string
	if err := row.Scan(&configJSON); err != nil {
		return nil, fmt.Errorf("query website config: %w", err)
	}

	if configJSON == "" || configJSON == "{}" {
		return &filter.WebsiteConfig{}, nil
	}

	var cfg filter.WebsiteConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		// Not a valid filter config, return empty
		return &filter.WebsiteConfig{}, nil
	}

	return &cfg, nil
}
