package server

import (
	"time"

	"github.com/raysh454/moku/internal/analyzer"
	"github.com/raysh454/moku/internal/api"
	"github.com/raysh454/moku/internal/filter"
)

// CreateProjectRequest represents the payload required to create a project.
type CreateProjectRequest struct {
	Slug        string `json:"slug" example:"ibm"`
	Name        string `json:"name" example:"IBM"`
	Description string `json:"description" example:"Project for IBM's public website"`
}

// CreateWebsiteRequest represents the payload for creating a website within a project.
type CreateWebsiteRequest struct {
	Slug   string `json:"slug" example:"demo"`
	Origin string `json:"origin" example:"http://localhost:9999"`
}

// AddWebsiteEndpointsRequest contains URLs to add to the endpoint index.
type AddWebsiteEndpointsRequest struct {
	URLs   []string `json:"urls" example:"[\"http://localhost:9999/index\"]"`
	Source string   `json:"source" example:"manual"`
}

// AddedEndpointsResponse reports how many endpoints were inserted.
type AddedEndpointsResponse struct {
	Added int `json:"added" example:"42"`
}

// StartFetchJobRequest optionally scopes a fetch job by endpoint status and limit.
type StartFetchJobRequest struct {
	Status string           `json:"status" default:"*" example:"*"`
	Limit  int              `json:"limit" example:"100"`
	Config *api.FetchConfig `json:"config,omitempty"`
	// Filter overrides for this fetch job
	SkipExtensions  []string `json:"skip_extensions,omitempty"`
	SkipPatterns    []string `json:"skip_patterns,omitempty"`
	SkipStatusCodes []int    `json:"skip_status_codes,omitempty"`
}

// StartEnumerateJobRequest configures enumeration with per-enumerator settings.
type StartEnumerateJobRequest struct {
	Config api.EnumerationConfig `json:"config"`
}

// StartScanJobRequest configures a vulnerability-scan job. The URL field is
// required; every other field is honored only when the selected backend's
// Capabilities advertise support for it.
type StartScanJobRequest struct {
	URL         string               `json:"url" example:"https://example.com/"`
	Profile     analyzer.ScanProfile `json:"profile,omitempty" example:"balanced"`
	Scope       *analyzer.ScanScope  `json:"scope,omitempty"`
	Auth        *analyzer.ScanAuth   `json:"auth,omitempty"`
	MaxDuration time.Duration        `json:"max_duration,omitempty" swaggertype:"primitive,integer" example:"300000000000"`
	RawOptions  map[string]string    `json:"raw_options,omitempty"`
}

// AnalyzerCapabilitiesResponse reports which backend is wired to a site and
// what optional ScanRequest fields it honors.
type AnalyzerCapabilitiesResponse struct {
	Backend      analyzer.Backend      `json:"backend"`
	Capabilities analyzer.Capabilities `json:"capabilities"`
}

// AnalyzerHealthResponse reports whether the configured analyzer backend is
// reachable right now.
type AnalyzerHealthResponse struct {
	Backend analyzer.Backend `json:"backend"`
	Status  string           `json:"status" example:"ok"`
}

// ErrorResponse is a uniform error payload returned by the API.
type ErrorResponse struct {
	Error string `json:"error" example:"not found"`
}

// --- Filter API Models ---

// CreateFilterRuleRequest represents the payload for creating a filter rule.
type CreateFilterRuleRequest struct {
	RuleType  string `json:"rule_type" example:"extension" enum:"extension,pattern,status_code"`
	RuleValue string `json:"rule_value" example:".jpg"`
	Priority  *int   `json:"priority,omitempty" example:"50"`
}

// UpdateFilterRuleRequest represents the payload for updating a filter rule.
type UpdateFilterRuleRequest struct {
	RuleType  string `json:"rule_type" example:"extension" enum:"extension,pattern,status_code"`
	RuleValue string `json:"rule_value" example:".jpg"`
	Enabled   *bool  `json:"enabled,omitempty" example:"true"`
}

// FilterRuleResponse represents a filter rule in API responses.
type FilterRuleResponse struct {
	ID        string `json:"id" example:"abc123"`
	WebsiteID string `json:"website_id" example:"xyz789"`
	RuleType  string `json:"rule_type" example:"extension"`
	RuleValue string `json:"rule_value" example:".jpg"`
	Enabled   bool   `json:"enabled" example:"true"`
	CreatedAt int64  `json:"created_at" example:"1609459200"`
	UpdatedAt int64  `json:"updated_at" example:"1609459200"`
}

// FilterRulesListResponse contains a list of filter rules.
type FilterRulesListResponse struct {
	Rules []FilterRuleResponse `json:"rules"`
}

// FilterConfigResponse represents the merged filter configuration.
type FilterConfigResponse struct {
	SkipExtensions  []string `json:"skip_extensions"`
	SkipPatterns    []string `json:"skip_patterns"`
	SkipStatusCodes []int    `json:"skip_status_codes"`
}

// UpdateFilterConfigRequest represents the payload for updating website filter config.
type UpdateFilterConfigRequest struct {
	SkipExtensions  []string `json:"skip_extensions,omitempty"`
	SkipPatterns    []string `json:"skip_patterns,omitempty"`
	SkipStatusCodes []int    `json:"skip_status_codes,omitempty"`
}

// UnfilterEndpointsRequest represents the payload for unfiltering endpoints.
type UnfilterEndpointsRequest struct {
	CanonicalURLs []string `json:"canonical_urls,omitempty"`
	All           bool     `json:"all,omitempty"`
}

// UnfilterEndpointsResponse reports how many endpoints were unfiltered.
type UnfilterEndpointsResponse struct {
	Unfiltered int `json:"unfiltered" example:"10"`
}

// EndpointStatsResponse contains endpoint statistics.
type EndpointStatsResponse struct {
	Total            int            `json:"total"`
	ByStatus         map[string]int `json:"by_status"`
	FilteredByReason map[string]int `json:"filtered_by_reason,omitempty"`
}

// ApplyFiltersResponse is the response for the apply filters endpoint.
type ApplyFiltersResponse struct {
	Filtered int    `json:"filtered"`
	Message  string `json:"message"`
}

// toFilterRuleResponse converts a filter.Rule to FilterRuleResponse.
func toFilterRuleResponse(r filter.Rule) FilterRuleResponse {
	return FilterRuleResponse{
		ID:        r.ID,
		WebsiteID: r.WebsiteID,
		RuleType:  string(r.RuleType),
		RuleValue: r.RuleValue,
		Enabled:   r.Enabled,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}
