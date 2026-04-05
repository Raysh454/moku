package registry

import (
	"context"
	"testing"

	"github.com/raysh454/moku/internal/filter"
)

// helper to create a test website with default rules cleared
func createTestWebsite(t *testing.T, reg *Registry, ctx context.Context) string {
	t.Helper()

	// Create a project first
	project, err := reg.CreateProject(ctx, "", "Test Project", "test")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// Create a website (this will seed default rules)
	website, err := reg.CreateWebsite(ctx, project.ID, "test-site", "https://example.com")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}

	// Delete all default rules so tests can work with a clean slate
	defaultRules, err := reg.ListFilterRules(ctx, website.ID)
	if err != nil {
		t.Fatalf("ListFilterRules: %v", err)
	}
	for _, rule := range defaultRules {
		if err := reg.DeleteFilterRule(ctx, rule.ID); err != nil {
			t.Fatalf("DeleteFilterRule: %v", err)
		}
	}

	return website.ID
}

// helper to create a test website WITH default rules (for testing seeding behavior)
func createTestWebsiteWithDefaults(t *testing.T, reg *Registry, ctx context.Context) string {
	t.Helper()

	// Create a project first
	project, err := reg.CreateProject(ctx, "", "Test Project", "test")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// Create a website (this will seed default rules)
	website, err := reg.CreateWebsite(ctx, project.ID, "test-site", "https://example.com")
	if err != nil {
		t.Fatalf("CreateWebsite: %v", err)
	}

	return website.ID
}

func TestFilterRules_CRUD(t *testing.T) {
	reg, cleanup := newTestRegistry(t)
	defer cleanup()

	ctx := context.Background()

	websiteID := createTestWebsite(t, reg, ctx)

	// Test AddFilterRule
	rule, err := reg.AddFilterRule(ctx, websiteID, filter.RuleTypeExtension, ".jpg")
	if err != nil {
		t.Fatalf("AddFilterRule: %v", err)
	}
	if rule.ID == "" {
		t.Error("AddFilterRule returned empty rule ID")
	}

	// Test GetFilterRule
	got, err := reg.GetFilterRule(ctx, rule.ID)
	if err != nil {
		t.Fatalf("GetFilterRule: %v", err)
	}
	if got.ID != rule.ID {
		t.Errorf("GetFilterRule ID = %q, want %q", got.ID, rule.ID)
	}
	if got.WebsiteID != websiteID {
		t.Errorf("GetFilterRule WebsiteID = %q, want %q", got.WebsiteID, websiteID)
	}
	if got.RuleType != filter.RuleTypeExtension {
		t.Errorf("GetFilterRule RuleType = %q, want %q", got.RuleType, filter.RuleTypeExtension)
	}
	if got.RuleValue != ".jpg" {
		t.Errorf("GetFilterRule RuleValue = %q, want %q", got.RuleValue, ".jpg")
	}
	if !got.Enabled {
		t.Error("GetFilterRule Enabled should be true")
	}

	// Test ListFilterRules
	rules, err := reg.ListFilterRules(ctx, websiteID)
	if err != nil {
		t.Fatalf("ListFilterRules: %v", err)
	}
	if len(rules) != 1 {
		t.Errorf("ListFilterRules count = %d, want 1", len(rules))
	}

	// Test UpdateFilterRule
	err = reg.UpdateFilterRule(ctx, rule.ID, filter.RuleTypeExtension, ".png", true)
	if err != nil {
		t.Fatalf("UpdateFilterRule: %v", err)
	}

	updated, err := reg.GetFilterRule(ctx, rule.ID)
	if err != nil {
		t.Fatalf("GetFilterRule after update: %v", err)
	}
	if updated.RuleValue != ".png" {
		t.Errorf("UpdateFilterRule RuleValue = %q, want %q", updated.RuleValue, ".png")
	}

	// Test UpdateFilterRulePriority
	err = reg.UpdateFilterRulePriority(ctx, rule.ID, 200)
	if err != nil {
		t.Fatalf("UpdateFilterRulePriority: %v", err)
	}

	withPriority, err := reg.GetFilterRule(ctx, rule.ID)
	if err != nil {
		t.Fatalf("GetFilterRule after priority update: %v", err)
	}
	if withPriority.Priority != 200 {
		t.Errorf("UpdateFilterRulePriority Priority = %d, want 200", withPriority.Priority)
	}

	// Test EnableFilterRule / DisableFilterRule
	err = reg.DisableFilterRule(ctx, rule.ID)
	if err != nil {
		t.Fatalf("DisableFilterRule: %v", err)
	}
	disabled, _ := reg.GetFilterRule(ctx, rule.ID)
	if disabled.Enabled {
		t.Error("DisableFilterRule should set Enabled=false")
	}

	err = reg.EnableFilterRule(ctx, rule.ID)
	if err != nil {
		t.Fatalf("EnableFilterRule: %v", err)
	}
	enabled, _ := reg.GetFilterRule(ctx, rule.ID)
	if !enabled.Enabled {
		t.Error("EnableFilterRule should set Enabled=true")
	}

	// Test DeleteFilterRule
	err = reg.DeleteFilterRule(ctx, rule.ID)
	if err != nil {
		t.Fatalf("DeleteFilterRule: %v", err)
	}

	_, err = reg.GetFilterRule(ctx, rule.ID)
	if err == nil {
		t.Error("GetFilterRule should fail after delete")
	}
}

func TestFilterRules_MultipleRules(t *testing.T) {
	reg, cleanup := newTestRegistry(t)
	defer cleanup()

	ctx := context.Background()

	websiteID := createTestWebsite(t, reg, ctx)

	// Add multiple rules of different types
	testRules := []struct {
		ruleType  filter.RuleType
		ruleValue string
	}{
		{filter.RuleTypeExtension, ".jpg"},
		{filter.RuleTypeExtension, ".png"},
		{filter.RuleTypePattern, "*/media/*"},
		{filter.RuleTypeStatusCode, "404"},
	}

	for _, tr := range testRules {
		_, err := reg.AddFilterRule(ctx, websiteID, tr.ruleType, tr.ruleValue)
		if err != nil {
			t.Fatalf("AddFilterRule %s: %v", tr.ruleValue, err)
		}
	}

	// Also add a disabled one
	rule, _ := reg.AddFilterRule(ctx, websiteID, filter.RuleTypeExtension, ".gif")
	_ = reg.DisableFilterRule(ctx, rule.ID)

	// List all rules
	listed, err := reg.ListFilterRules(ctx, websiteID)
	if err != nil {
		t.Fatalf("ListFilterRules: %v", err)
	}
	if len(listed) != 5 {
		t.Errorf("ListFilterRules count = %d, want 5", len(listed))
	}

	// List enabled rules only
	enabledRules, err := reg.ListEnabledFilterRules(ctx, websiteID)
	if err != nil {
		t.Fatalf("ListEnabledFilterRules: %v", err)
	}
	if len(enabledRules) != 4 {
		t.Errorf("ListEnabledFilterRules count = %d, want 4", len(enabledRules))
	}

	// Count by type
	extCount, patCount, statusCount := 0, 0, 0
	for _, r := range listed {
		switch r.RuleType {
		case filter.RuleTypeExtension:
			extCount++
		case filter.RuleTypePattern:
			patCount++
		case filter.RuleTypeStatusCode:
			statusCount++
		}
	}

	if extCount != 3 {
		t.Errorf("Extension rules count = %d, want 3", extCount)
	}
	if patCount != 1 {
		t.Errorf("Pattern rules count = %d, want 1", patCount)
	}
	if statusCount != 1 {
		t.Errorf("StatusCode rules count = %d, want 1", statusCount)
	}
}

func TestFilterRules_AddWithPriority(t *testing.T) {
	reg, cleanup := newTestRegistry(t)
	defer cleanup()

	ctx := context.Background()

	websiteID := createTestWebsite(t, reg, ctx)

	// Add rule with custom priority
	rule, err := reg.AddFilterRuleWithPriority(ctx, websiteID, filter.RuleTypePattern, "*/vendor/*", 200)
	if err != nil {
		t.Fatalf("AddFilterRuleWithPriority: %v", err)
	}

	if rule.Priority != 200 {
		t.Errorf("Priority = %d, want 200", rule.Priority)
	}
}

func TestFilterRules_GlobalRules(t *testing.T) {
	reg, cleanup := newTestRegistry(t)
	defer cleanup()

	ctx := context.Background()

	// Global rules have empty websiteID
	rule, err := reg.AddFilterRule(ctx, "", filter.RuleTypeExtension, ".exe")
	if err != nil {
		t.Fatalf("AddFilterRule (global): %v", err)
	}

	got, err := reg.GetFilterRule(ctx, rule.ID)
	if err != nil {
		t.Fatalf("GetFilterRule: %v", err)
	}
	if got.WebsiteID != "" {
		t.Errorf("Global rule WebsiteID = %q, want empty", got.WebsiteID)
	}

	// List global rules (empty websiteID)
	rules, err := reg.ListFilterRules(ctx, "")
	if err != nil {
		t.Fatalf("ListFilterRules (global): %v", err)
	}
	if len(rules) != 1 {
		t.Errorf("ListFilterRules count = %d, want 1", len(rules))
	}
}

func TestLoadFilterConfig(t *testing.T) {
	reg, cleanup := newTestRegistry(t)
	defer cleanup()

	ctx := context.Background()

	websiteID := createTestWebsite(t, reg, ctx)

	// Add some rules
	testRules := []struct {
		ruleType  filter.RuleType
		ruleValue string
	}{
		{filter.RuleTypeExtension, ".jpg"},
		{filter.RuleTypePattern, "*/media/*"},
		{filter.RuleTypeStatusCode, "404"},
	}

	for _, tr := range testRules {
		_, err := reg.AddFilterRule(ctx, websiteID, tr.ruleType, tr.ruleValue)
		if err != nil {
			t.Fatalf("AddFilterRule: %v", err)
		}
	}

	// Also add a disabled one
	rule, _ := reg.AddFilterRule(ctx, websiteID, filter.RuleTypeExtension, ".gif")
	_ = reg.DisableFilterRule(ctx, rule.ID)

	// Load filter config (with nil global config)
	config, err := reg.LoadFilterConfig(ctx, websiteID, nil)
	if err != nil {
		t.Fatalf("LoadFilterConfig: %v", err)
	}

	// Check extensions (only enabled rules)
	if len(config.SkipExtensions) != 1 || config.SkipExtensions[0] != ".jpg" {
		t.Errorf("SkipExtensions = %v, want [.jpg]", config.SkipExtensions)
	}

	// Check patterns
	if len(config.SkipPatterns) != 1 || config.SkipPatterns[0] != "*/media/*" {
		t.Errorf("SkipPatterns = %v, want [*/media/*]", config.SkipPatterns)
	}

	// Check status codes
	if len(config.SkipStatusCodes) != 1 || config.SkipStatusCodes[0] != 404 {
		t.Errorf("SkipStatusCodes = %v, want [404]", config.SkipStatusCodes)
	}

	// Check rules (should include only enabled)
	if len(config.Rules) != 3 {
		t.Errorf("Rules count = %d, want 3 (enabled only)", len(config.Rules))
	}
}

func TestLoadFilterConfig_Empty(t *testing.T) {
	reg, cleanup := newTestRegistry(t)
	defer cleanup()

	ctx := context.Background()

	websiteID := createTestWebsite(t, reg, ctx)

	// Load config with no rules
	config, err := reg.LoadFilterConfig(ctx, websiteID, nil)
	if err != nil {
		t.Fatalf("LoadFilterConfig: %v", err)
	}

	if !config.IsEmpty() {
		t.Error("LoadFilterConfig should return empty config when no rules exist")
	}
}

func TestSeedDefaultFilterRules(t *testing.T) {
	reg, cleanup := newTestRegistry(t)
	defer cleanup()

	ctx := context.Background()

	// Create website with defaults (don't use createTestWebsite which clears them)
	websiteID := createTestWebsiteWithDefaults(t, reg, ctx)

	// Load rules
	rules, err := reg.ListFilterRules(ctx, websiteID)
	if err != nil {
		t.Fatalf("ListFilterRules: %v", err)
	}

	// Should have default rules (at least the extension rules from DefaultFilterConfig)
	defaults := filter.DefaultFilterConfig()
	expectedCount := len(defaults.SkipExtensions) + len(defaults.SkipPatterns) + len(defaults.SkipStatusCodes)

	if len(rules) != expectedCount {
		t.Errorf("Expected %d default rules, got %d", expectedCount, len(rules))
	}

	// All rules should be enabled
	for _, rule := range rules {
		if !rule.Enabled {
			t.Errorf("Default rule %s should be enabled", rule.RuleValue)
		}
	}

	// Check we have extension rules
	extensionCount := 0
	for _, rule := range rules {
		if rule.RuleType == filter.RuleTypeExtension {
			extensionCount++
		}
	}
	if extensionCount != len(defaults.SkipExtensions) {
		t.Errorf("Expected %d extension rules, got %d", len(defaults.SkipExtensions), extensionCount)
	}
}
