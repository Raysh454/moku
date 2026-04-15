package filter

import (
	"strconv"
	"strings"
)

// DefaultConfig returns a Config with security-focused defaults.
// These defaults skip obvious binary/media files while keeping security-relevant content.
func DefaultConfig() *Config {
	return &Config{
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
func MergeConfigs(configs ...*Config) *Config {
	result := &Config{
		SkipExtensions:  []string{},
		SkipPatterns:    []string{},
		SkipStatusCodes: []int{},
		Rules:           []Rule{},
	}

	seenExtensions := make(map[string]bool)
	seenPatterns := make(map[string]bool)
	seenStatusCodes := make(map[int]bool)
	seenRules := make(map[string]bool)

	for _, cfg := range configs {
		if cfg == nil {
			continue
		}

		// Extensions need normalization before dedup (case / missing-dot).
		for _, ext := range cfg.SkipExtensions {
			normalized := normalizeExtension(ext)
			if !seenExtensions[normalized] {
				seenExtensions[normalized] = true
				result.SkipExtensions = append(result.SkipExtensions, normalized)
			}
		}

		result.SkipPatterns = dedupAppend(result.SkipPatterns, cfg.SkipPatterns, seenPatterns)
		result.SkipStatusCodes = dedupAppend(result.SkipStatusCodes, cfg.SkipStatusCodes, seenStatusCodes)

		// Rules dedup by ID, not by value.
		for _, rule := range cfg.Rules {
			if !seenRules[rule.ID] {
				seenRules[rule.ID] = true
				result.Rules = append(result.Rules, rule)
			}
		}
	}

	return result
}

// dedupAppend appends values from src to dst, skipping any value already in seen.
// Values added are recorded in seen, so the same map can be threaded across
// multiple calls to accumulate state.
func dedupAppend[T comparable](dst, src []T, seen map[T]bool) []T {
	for _, v := range src {
		if seen[v] {
			continue
		}
		seen[v] = true
		dst = append(dst, v)
	}
	return dst
}

// normalizeExtension ensures extension is lowercase and starts with a dot.
func normalizeExtension(ext string) string {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return ext
}

// RulesToConfig converts a slice of Rules to Config.
func RulesToConfig(rules []Rule) *Config {
	cfg := &Config{
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
			if n, err := strconv.Atoi(rule.RuleValue); err == nil {
				cfg.SkipStatusCodes = append(cfg.SkipStatusCodes, n)
			}
		}
	}

	return cfg
}
