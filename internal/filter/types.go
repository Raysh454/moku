// Package filter provides URL and response filtering functionality.
// It supports filtering by file extension, URL pattern, and HTTP status code.
package filter

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// RuleType defines the type of filter rule.
type RuleType string

const (
	RuleTypeExtension  RuleType = "extension"   // Filter by file extension (.jpg, .png)
	RuleTypePattern    RuleType = "pattern"     // Filter by URL pattern (*/assets/*)
	RuleTypeStatusCode RuleType = "status_code" // Filter by HTTP status code (404, 500)
)

// Priority constants for rule evaluation order.
// Higher priority rules are evaluated first.
const (
	PriorityPattern    = 100 // Patterns evaluated first (most specific)
	PriorityExtension  = 50  // Extensions evaluated second
	PriorityStatusCode = 25  // Status codes evaluated last
)

// FilterRule represents a single filter rule stored in the database.
type FilterRule struct {
	ID        string   `json:"id"`
	WebsiteID string   `json:"website_id"`
	RuleType  RuleType `json:"rule_type"`
	RuleValue string   `json:"rule_value"` // ".jpg", "*/media/*", "404"
	Priority  int      `json:"priority"`
	Enabled   bool     `json:"enabled"`
	CreatedAt int64    `json:"created_at"`
	UpdatedAt int64    `json:"updated_at"`
}

// Validate checks if the filter rule is valid.
func (r *FilterRule) Validate() error {
	if !r.RuleType.IsValid() {
		return fmt.Errorf("invalid rule type: %s", r.RuleType)
	}
	if r.RuleValue == "" {
		return errors.New("rule value cannot be empty")
	}

	switch r.RuleType {
	case RuleTypeExtension:
		if err := ValidateExtension(r.RuleValue); err != nil {
			return fmt.Errorf("invalid extension: %w", err)
		}
	case RuleTypePattern:
		if err := ValidatePattern(r.RuleValue); err != nil {
			return fmt.Errorf("invalid pattern: %w", err)
		}
	case RuleTypeStatusCode:
		if err := ValidateStatusCode(r.RuleValue); err != nil {
			return fmt.Errorf("invalid status_code: %w", err)
		}
	}

	return nil
}

// DefaultPriority returns the default priority for this rule type.
func (r *FilterRule) DefaultPriority() int {
	switch r.RuleType {
	case RuleTypePattern:
		return PriorityPattern
	case RuleTypeExtension:
		return PriorityExtension
	case RuleTypeStatusCode:
		return PriorityStatusCode
	default:
		return 0
	}
}

// IsValid checks if the RuleType is a known valid type.
func (rt RuleType) IsValid() bool {
	switch rt {
	case RuleTypeExtension, RuleTypePattern, RuleTypeStatusCode:
		return true
	default:
		return false
	}
}

// FilterConfig holds the filter configuration.
// This is a simplified skip-only configuration.
type FilterConfig struct {
	// SkipExtensions are file extensions to skip (e.g., ".jpg", ".png")
	SkipExtensions []string `json:"skip_extensions,omitempty"`

	// SkipPatterns are URL patterns to skip (glob syntax)
	SkipPatterns []string `json:"skip_patterns,omitempty"`

	// SkipStatusCodes are HTTP status codes to filter (e.g., 404)
	SkipStatusCodes []int `json:"skip_status_codes,omitempty"`

	// Rules are the individual FilterRule objects (from database)
	Rules []FilterRule `json:"rules,omitempty"`
}

// IsEmpty returns true if the config has no filtering rules.
func (c *FilterConfig) IsEmpty() bool {
	if c == nil {
		return true
	}
	return len(c.SkipExtensions) == 0 &&
		len(c.SkipPatterns) == 0 &&
		len(c.SkipStatusCodes) == 0 &&
		len(c.Rules) == 0
}

// FilterResult contains the result of a filter check.
type FilterResult struct {
	Filtered bool   `json:"filtered"`
	Reason   string `json:"reason,omitempty"`
	RuleID   string `json:"rule_id,omitempty"`
}

// NewFilteredResult creates a FilterResult indicating the URL should be filtered.
func NewFilteredResult(reason string, ruleID string) FilterResult {
	return FilterResult{
		Filtered: true,
		Reason:   reason,
		RuleID:   ruleID,
	}
}

// FilterEngine is the interface for filter evaluation.
type FilterEngine interface {
	ShouldFilter(url string) FilterResult
	ShouldFilterStatus(statusCode int) FilterResult
}

// ValidateExtension checks if an extension value is valid.
func ValidateExtension(ext string) error {
	if ext == "" {
		return errors.New("extension cannot be empty")
	}
	if !strings.HasPrefix(ext, ".") {
		return errors.New("extension should start with '.'")
	}
	if len(ext) < 2 {
		return errors.New("extension too short")
	}
	// Check for invalid characters (only alphanumeric after the dot)
	for _, c := range ext[1:] {
		if !isValidExtensionChar(c) {
			return fmt.Errorf("invalid character '%c' in extension", c)
		}
	}
	return nil
}

func isValidExtensionChar(c rune) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9')
}

// ValidatePattern checks if a glob pattern is valid.
func ValidatePattern(pattern string) error {
	if pattern == "" {
		return errors.New("pattern cannot be empty")
	}
	if !strings.Contains(pattern, "*") && !strings.Contains(pattern, "?") {
		return errors.New("pattern must contain at least one wildcard (* or ?)")
	}
	return nil
}

// ValidateStatusCode checks if a status code value is valid.
func ValidateStatusCode(code string) error {
	if code == "" {
		return errors.New("status_code cannot be empty")
	}
	codeInt, err := strconv.Atoi(code)
	if err != nil {
		return fmt.Errorf("status_code must be a number: %w", err)
	}
	if codeInt < 100 || codeInt > 599 {
		return fmt.Errorf("status_code must be between 100 and 599, got %d", codeInt)
	}
	return nil
}

// WebsiteFilterConfig is the JSON structure stored in websites.config column.
type WebsiteFilterConfig struct {
	SkipExtensions  []string `json:"skip_extensions,omitempty"`
	SkipPatterns    []string `json:"skip_patterns,omitempty"`
	SkipStatusCodes []int    `json:"skip_status_codes,omitempty"`
}

// ToFilterConfig converts WebsiteFilterConfig to FilterConfig.
func (w *WebsiteFilterConfig) ToFilterConfig() *FilterConfig {
	if w == nil {
		return &FilterConfig{}
	}
	return &FilterConfig{
		SkipExtensions:  w.SkipExtensions,
		SkipPatterns:    w.SkipPatterns,
		SkipStatusCodes: w.SkipStatusCodes,
	}
}

// FilteredEndpoint represents an endpoint that was filtered with metadata.
type FilteredEndpoint struct {
	URL          string    `json:"url"`
	CanonicalURL string    `json:"canonical_url"`
	FilterReason string    `json:"filter_reason"`
	FilteredAt   time.Time `json:"filtered_at"`
}
