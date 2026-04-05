package filter

import (
	"strings"
)

// DefaultFilterConfig returns a FilterConfig with security-focused defaults.
// These defaults skip obvious binary/media files while keeping security-relevant content.
func DefaultFilterConfig() *FilterConfig {
	return &FilterConfig{
		// Skip obvious binary/media files that are unlikely to contain security issues
		SkipExtensions: []string{
			// Images (static pixels - no XSS risk)
			".jpg", ".jpeg", ".png", ".gif", ".bmp", ".ico", ".webp", ".tiff",
			// Video
			".mp4", ".avi", ".mov", ".wmv", ".flv", ".webm", ".mkv", ".m4v",
			// Audio
			".mp3", ".wav", ".ogg", ".m4a", ".flac", ".aac", ".wma",
			// Archives
			".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".iso",
			// Executables
			".exe", ".dll", ".so", ".dylib", ".bin",
			// Fonts
			".ttf", ".woff", ".woff2", ".eot", ".otf",
			// Documents (usually static, low security value)
			".pdf", ".doc", ".docx", ".ppt", ".pptx", ".xls", ".xlsx",
		},
		// No patterns skipped by default (opt-in)
		SkipPatterns: []string{},
		// No status codes skipped by default (404 filtering is opt-in, 401/403 are security signals)
		SkipStatusCodes: []int{},
	}
}

// MergeConfigs merges multiple filter configs with later configs taking precedence.
// The merge strategy is: combine all skip lists and deduplicate.
// Order: global -> website -> api overrides
func MergeConfigs(configs ...*FilterConfig) *FilterConfig {
	result := &FilterConfig{
		SkipExtensions:  []string{},
		SkipPatterns:    []string{},
		SkipStatusCodes: []int{},
		Rules:           []FilterRule{},
	}

	// Track seen items for deduplication
	seenExtensions := make(map[string]bool)
	seenPatterns := make(map[string]bool)
	seenStatusCodes := make(map[int]bool)
	seenRules := make(map[string]bool)

	for _, cfg := range configs {
		if cfg == nil {
			continue
		}

		// Merge extensions
		for _, ext := range cfg.SkipExtensions {
			normalized := normalizeExtension(ext)
			if !seenExtensions[normalized] {
				seenExtensions[normalized] = true
				result.SkipExtensions = append(result.SkipExtensions, normalized)
			}
		}

		// Merge patterns
		for _, pattern := range cfg.SkipPatterns {
			if !seenPatterns[pattern] {
				seenPatterns[pattern] = true
				result.SkipPatterns = append(result.SkipPatterns, pattern)
			}
		}

		// Merge status codes
		for _, code := range cfg.SkipStatusCodes {
			if !seenStatusCodes[code] {
				seenStatusCodes[code] = true
				result.SkipStatusCodes = append(result.SkipStatusCodes, code)
			}
		}

		// Merge rules
		for _, rule := range cfg.Rules {
			if !seenRules[rule.ID] {
				seenRules[rule.ID] = true
				result.Rules = append(result.Rules, rule)
			}
		}
	}

	return result
}

// normalizeExtension ensures extension is lowercase and starts with a dot.
func normalizeExtension(ext string) string {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return ext
}

// RulesToConfig converts a slice of FilterRules to FilterConfig.
func RulesToConfig(rules []FilterRule) *FilterConfig {
	cfg := &FilterConfig{
		SkipExtensions:  []string{},
		SkipPatterns:    []string{},
		SkipStatusCodes: []int{},
		Rules:           rules,
	}

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		switch rule.RuleType {
		case RuleTypeExtension:
			cfg.SkipExtensions = append(cfg.SkipExtensions, rule.RuleValue)
		case RuleTypePattern:
			cfg.SkipPatterns = append(cfg.SkipPatterns, rule.RuleValue)
		case RuleTypeStatusCode:
			// Parse status code
			if n, err := parseInt(rule.RuleValue); err == nil {
				cfg.SkipStatusCodes = append(cfg.SkipStatusCodes, n)
			}
		}
	}

	return cfg
}

func parseInt(s string) (int, error) {
	var result int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, &strconvError{s}
		}
		result = result*10 + int(c-'0')
	}
	return result, nil
}

type strconvError struct {
	s string
}

func (e *strconvError) Error() string {
	return "invalid number: " + e.s
}
