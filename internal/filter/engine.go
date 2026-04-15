package filter

import (
	"fmt"
	"sort"
	"strconv"
)

// Engine evaluates URLs and status codes against filter configuration.
type Engine struct {
	config *Config
}

// NewEngine creates a new filter engine with the given configuration.
func NewEngine(config *Config) *Engine {
	if config == nil {
		config = &Config{}
	}
	return &Engine{config: config}
}

// ShouldFilter checks if a URL should be filtered based on the configuration.
// Returns a Result with filtered=true if the URL should be skipped.
//
// Evaluation order (higher priority first):
// 1. Pattern rules (most specific)
// 2. Extension rules
// 3. Status code rules (handled by ShouldFilterStatus)
func (e *Engine) ShouldFilter(urlStr string) Result {
	if e.config == nil || e.config.IsEmpty() {
		return Result{Filtered: false}
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
			return Result{
				Filtered: true,
				Reason:   fmt.Sprintf("%s:%s", rule.RuleType, rule.RuleValue),
			}
		}
	}

	// Then check config-level patterns (higher priority than extensions)
	if matched, pattern := matchPattern(urlStr, e.config.SkipPatterns); matched {
		return Result{
			Filtered: true,
			Reason:   fmt.Sprintf("pattern:%s", pattern),
		}
	}

	// Finally check config-level extensions
	if matched, ext := matchExtension(urlStr, e.config.SkipExtensions); matched {
		return Result{
			Filtered: true,
			Reason:   fmt.Sprintf("extension:%s", ext),
		}
	}

	return Result{Filtered: false}
}

// ShouldFilterStatus checks if a response status code should be filtered.
func (e *Engine) ShouldFilterStatus(statusCode int) Result {
	if e.config == nil || e.config.IsEmpty() {
		return Result{Filtered: false}
	}

	// Check database rules for status codes first
	for _, rule := range e.config.Rules {
		if !rule.Enabled || rule.RuleType != RuleTypeStatusCode {
			continue
		}

		codeVal, err := strconv.Atoi(rule.RuleValue)
		if err != nil {
			continue
		}

		if statusCode == codeVal {
			return Result{
				Filtered: true,
				Reason:   fmt.Sprintf("status_code:%d", statusCode),
			}
		}
	}

	// Check config-level status codes
	if matchStatusCode(statusCode, e.config.SkipStatusCodes) {
		return Result{
			Filtered: true,
			Reason:   fmt.Sprintf("status_code:%d", statusCode),
		}
	}

	return Result{Filtered: false}
}

// sortedRules returns the rules sorted by priority (highest first).
func (e *Engine) sortedRules() []Rule {
	if e.config == nil || len(e.config.Rules) == 0 {
		return nil
	}

	rules := make([]Rule, len(e.config.Rules))
	copy(rules, e.config.Rules)

	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority > rules[j].Priority
	})

	return rules
}
