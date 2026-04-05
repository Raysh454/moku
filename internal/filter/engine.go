package filter

import (
	"fmt"
	"sort"
)

// Engine evaluates URLs and status codes against filter configuration.
type Engine struct {
	config *FilterConfig
}

// NewEngine creates a new filter engine with the given configuration.
func NewEngine(config *FilterConfig) *Engine {
	if config == nil {
		config = &FilterConfig{}
	}
	return &Engine{config: config}
}

// ShouldFilter checks if a URL should be filtered based on the configuration.
// Returns a FilterResult with filtered=true if the URL should be skipped.
//
// Evaluation order (higher priority first):
// 1. Pattern rules (most specific)
// 2. Extension rules
// 3. Status code rules (handled by ShouldFilterStatus)
func (e *Engine) ShouldFilter(urlStr string) FilterResult {
	if e.config == nil || e.config.IsEmpty() {
		return FilterResult{Filtered: false}
	}

	// Sort rules by priority (highest first) for evaluation
	rules := e.sortedRules()

	// First check database rules (they have explicit priority)
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		var matched bool
		switch rule.RuleType {
		case RuleTypePattern:
			matched, _ = matchPattern(urlStr, []string{rule.RuleValue})
		case RuleTypeExtension:
			matched, _ = matchExtension(urlStr, []string{rule.RuleValue})
		case RuleTypeStatusCode:
			// Status codes are checked via ShouldFilterStatus, not here
			continue
		}

		if matched {
			return NewFilteredResult(
				fmt.Sprintf("%s:%s", rule.RuleType, rule.RuleValue),
				rule.ID,
			)
		}
	}

	// Then check config-level patterns (higher priority than extensions)
	if matched, pattern := matchPattern(urlStr, e.config.SkipPatterns); matched {
		return NewFilteredResult(
			fmt.Sprintf("pattern:%s", pattern),
			"",
		)
	}

	// Finally check config-level extensions
	if matched, ext := matchExtension(urlStr, e.config.SkipExtensions); matched {
		return NewFilteredResult(
			fmt.Sprintf("extension:%s", ext),
			"",
		)
	}

	return FilterResult{Filtered: false}
}

// ShouldFilterStatus checks if a response status code should be filtered.
func (e *Engine) ShouldFilterStatus(statusCode int) FilterResult {
	if e.config == nil || e.config.IsEmpty() {
		return FilterResult{Filtered: false}
	}

	// Check database rules for status codes first
	for _, rule := range e.config.Rules {
		if !rule.Enabled || rule.RuleType != RuleTypeStatusCode {
			continue
		}

		codeVal, err := parseInt(rule.RuleValue)
		if err != nil {
			continue
		}

		if statusCode == codeVal {
			return NewFilteredResult(
				fmt.Sprintf("status_code:%d", statusCode),
				rule.ID,
			)
		}
	}

	// Check config-level status codes
	if matchStatusCode(statusCode, e.config.SkipStatusCodes) {
		return NewFilteredResult(
			fmt.Sprintf("status_code:%d", statusCode),
			"",
		)
	}

	return FilterResult{Filtered: false}
}

// sortedRules returns the rules sorted by priority (highest first).
func (e *Engine) sortedRules() []FilterRule {
	if e.config == nil || len(e.config.Rules) == 0 {
		return nil
	}

	rules := make([]FilterRule, len(e.config.Rules))
	copy(rules, e.config.Rules)

	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority > rules[j].Priority
	})

	return rules
}

// FilterURLs filters a list of URLs and returns two lists:
// - unfiltered: URLs that should be fetched
// - filtered: URLs that should be skipped, with their filter reasons
func (e *Engine) FilterURLs(urls []string) (unfiltered []string, filtered []FilteredEndpoint) {
	for _, u := range urls {
		result := e.ShouldFilter(u)
		if result.Filtered {
			filtered = append(filtered, FilteredEndpoint{
				URL:          u,
				FilterReason: result.Reason,
			})
		} else {
			unfiltered = append(unfiltered, u)
		}
	}
	return
}
