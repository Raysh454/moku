package registry

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/raysh454/moku/internal/filter"
	"github.com/raysh454/moku/internal/logging"
)

var ErrFilterRuleNotFound = fmt.Errorf("filter rule not found")

// AddFilterRule creates a new filter rule for a website.
func (r *Registry) AddFilterRule(ctx context.Context, websiteID string, ruleType filter.RuleType, ruleValue string) (*filter.Rule, error) {
	rule := &filter.Rule{
		ID:        uuid.New().String(),
		WebsiteID: websiteID,
		RuleType:  ruleType,
		RuleValue: ruleValue,
		Enabled:   true,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	// Set default priority based on rule type
	rule.Priority = rule.DefaultPriority()

	// Validate the rule
	if err := rule.Validate(); err != nil {
		return nil, fmt.Errorf("invalid filter rule: %w", err)
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO filter_rules (id, website_id, rule_type, rule_value, priority, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		rule.ID, rule.WebsiteID, rule.RuleType, rule.RuleValue, rule.Priority, rule.Enabled, rule.CreatedAt, rule.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert filter rule: %w", err)
	}

	return rule, nil
}

// AddFilterRuleWithPriority creates a new filter rule with a custom priority.
func (r *Registry) AddFilterRuleWithPriority(ctx context.Context, websiteID string, ruleType filter.RuleType, ruleValue string, priority int) (*filter.Rule, error) {
	rule := &filter.Rule{
		ID:        uuid.New().String(),
		WebsiteID: websiteID,
		RuleType:  ruleType,
		RuleValue: ruleValue,
		Priority:  priority,
		Enabled:   true,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	// Validate the rule
	if err := rule.Validate(); err != nil {
		return nil, fmt.Errorf("invalid filter rule: %w", err)
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO filter_rules (id, website_id, rule_type, rule_value, priority, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		rule.ID, rule.WebsiteID, rule.RuleType, rule.RuleValue, rule.Priority, rule.Enabled, rule.CreatedAt, rule.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert filter rule: %w", err)
	}

	return rule, nil
}

// ListFilterRules returns all filter rules for a website.
func (r *Registry) ListFilterRules(ctx context.Context, websiteID string) ([]filter.Rule, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, website_id, rule_type, rule_value, priority, enabled, created_at, updated_at
		 FROM filter_rules
		 WHERE website_id = ?
		 ORDER BY priority DESC, created_at ASC`,
		websiteID,
	)
	if err != nil {
		return nil, fmt.Errorf("query filter rules: %w", err)
	}
	defer rows.Close()

	var rules []filter.Rule
	for rows.Next() {
		var rule filter.Rule
		if err := rows.Scan(&rule.ID, &rule.WebsiteID, &rule.RuleType, &rule.RuleValue, &rule.Priority, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan filter rule: %w", err)
		}
		rules = append(rules, rule)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate filter rules: %w", err)
	}

	return rules, nil
}

// ListEnabledFilterRules returns only enabled filter rules for a website.
func (r *Registry) ListEnabledFilterRules(ctx context.Context, websiteID string) ([]filter.Rule, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, website_id, rule_type, rule_value, priority, enabled, created_at, updated_at
		 FROM filter_rules
		 WHERE website_id = ? AND enabled = 1
		 ORDER BY priority DESC, created_at ASC`,
		websiteID,
	)
	if err != nil {
		return nil, fmt.Errorf("query filter rules: %w", err)
	}
	defer rows.Close()

	var rules []filter.Rule
	for rows.Next() {
		var rule filter.Rule
		if err := rows.Scan(&rule.ID, &rule.WebsiteID, &rule.RuleType, &rule.RuleValue, &rule.Priority, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan filter rule: %w", err)
		}
		rules = append(rules, rule)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate filter rules: %w", err)
	}

	return rules, nil
}

// GetFilterRule returns a single filter rule by ID.
func (r *Registry) GetFilterRule(ctx context.Context, ruleID string) (*filter.Rule, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, website_id, rule_type, rule_value, priority, enabled, created_at, updated_at
		 FROM filter_rules
		 WHERE id = ?`,
		ruleID,
	)

	var rule filter.Rule
	if err := row.Scan(&rule.ID, &rule.WebsiteID, &rule.RuleType, &rule.RuleValue, &rule.Priority, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrFilterRuleNotFound
		}
		return nil, fmt.Errorf("scan filter rule: %w", err)
	}

	return &rule, nil
}

// UpdateFilterRule updates an existing filter rule.
func (r *Registry) UpdateFilterRule(ctx context.Context, ruleID string, ruleType filter.RuleType, ruleValue string, enabled bool) error {
	// Create a rule struct for validation
	rule := &filter.Rule{
		RuleType:  ruleType,
		RuleValue: ruleValue,
	}
	if err := rule.Validate(); err != nil {
		return fmt.Errorf("invalid filter rule: %w", err)
	}

	now := time.Now().Unix()
	result, err := r.db.ExecContext(ctx,
		`UPDATE filter_rules
		 SET rule_type = ?, rule_value = ?, enabled = ?, updated_at = ?
		 WHERE id = ?`,
		ruleType, ruleValue, enabled, now, ruleID,
	)
	if err != nil {
		return fmt.Errorf("update filter rule: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return ErrFilterRuleNotFound
	}

	return nil
}

// UpdateFilterRulePriority updates the priority of a filter rule.
func (r *Registry) UpdateFilterRulePriority(ctx context.Context, ruleID string, priority int) error {
	now := time.Now().Unix()
	result, err := r.db.ExecContext(ctx,
		`UPDATE filter_rules SET priority = ?, updated_at = ? WHERE id = ?`,
		priority, now, ruleID,
	)
	if err != nil {
		return fmt.Errorf("update filter rule priority: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return ErrFilterRuleNotFound
	}

	return nil
}

// DeleteFilterRule removes a filter rule by ID.
func (r *Registry) DeleteFilterRule(ctx context.Context, ruleID string) error {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM filter_rules WHERE id = ?`,
		ruleID,
	)
	if err != nil {
		return fmt.Errorf("delete filter rule: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return ErrFilterRuleNotFound
	}

	return nil
}

// DeleteAllFilterRules removes all filter rules for a website.
func (r *Registry) DeleteAllFilterRules(ctx context.Context, websiteID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM filter_rules WHERE website_id = ?`,
		websiteID,
	)
	if err != nil {
		return fmt.Errorf("delete filter rules: %w", err)
	}
	return nil
}

// EnableFilterRule enables a filter rule.
func (r *Registry) EnableFilterRule(ctx context.Context, ruleID string) error {
	now := time.Now().Unix()
	result, err := r.db.ExecContext(ctx,
		`UPDATE filter_rules SET enabled = 1, updated_at = ? WHERE id = ?`,
		now, ruleID,
	)
	if err != nil {
		return fmt.Errorf("enable filter rule: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return ErrFilterRuleNotFound
	}

	return nil
}

// DisableFilterRule disables a filter rule.
func (r *Registry) DisableFilterRule(ctx context.Context, ruleID string) error {
	now := time.Now().Unix()
	result, err := r.db.ExecContext(ctx,
		`UPDATE filter_rules SET enabled = 0, updated_at = ? WHERE id = ?`,
		now, ruleID,
	)
	if err != nil {
		return fmt.Errorf("disable filter rule: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return ErrFilterRuleNotFound
	}

	return nil
}

// SeedDefaultFilterRules creates default filter rules for a website.
// This seeds the default skip extensions so they appear in the UI as toggleable rules.
func (r *Registry) SeedDefaultFilterRules(ctx context.Context, websiteID string) error {
	defaults := filter.DefaultConfig()
	now := time.Now().Unix()

	// Begin transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO filter_rules (id, website_id, rule_type, rule_value, priority, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	// Seed extension rules
	for _, ext := range defaults.SkipExtensions {
		rule := &filter.Rule{
			ID:        uuid.New().String(),
			WebsiteID: websiteID,
			RuleType:  filter.RuleTypeExtension,
			RuleValue: ext,
			Enabled:   true,
			CreatedAt: now,
			UpdatedAt: now,
		}
		rule.Priority = rule.DefaultPriority()

		_, err = stmt.ExecContext(ctx,
			rule.ID, rule.WebsiteID, rule.RuleType, rule.RuleValue, rule.Priority, rule.Enabled, rule.CreatedAt, rule.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert extension rule %s: %w", ext, err)
		}
	}

	// Seed pattern rules (if any defaults exist)
	for _, pattern := range defaults.SkipPatterns {
		rule := &filter.Rule{
			ID:        uuid.New().String(),
			WebsiteID: websiteID,
			RuleType:  filter.RuleTypePattern,
			RuleValue: pattern,
			Enabled:   true,
			CreatedAt: now,
			UpdatedAt: now,
		}
		rule.Priority = rule.DefaultPriority()

		_, err = stmt.ExecContext(ctx,
			rule.ID, rule.WebsiteID, rule.RuleType, rule.RuleValue, rule.Priority, rule.Enabled, rule.CreatedAt, rule.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert pattern rule %s: %w", pattern, err)
		}
	}

	// Seed status code rules (if any defaults exist)
	for _, code := range defaults.SkipStatusCodes {
		rule := &filter.Rule{
			ID:        uuid.New().String(),
			WebsiteID: websiteID,
			RuleType:  filter.RuleTypeStatusCode,
			RuleValue: fmt.Sprintf("%d", code),
			Enabled:   true,
			CreatedAt: now,
			UpdatedAt: now,
		}
		rule.Priority = rule.DefaultPriority()

		_, err = stmt.ExecContext(ctx,
			rule.ID, rule.WebsiteID, rule.RuleType, rule.RuleValue, rule.Priority, rule.Enabled, rule.CreatedAt, rule.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert status code rule %d: %w", code, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// SeedDefaultsForAllWebsites seeds default filter rules for all websites that have no rules.
// This is intended to be called at startup for backwards compatibility with existing websites.
func (r *Registry) SeedDefaultsForAllWebsites(ctx context.Context) error {
	// Get all websites
	rows, err := r.db.QueryContext(ctx,
		`SELECT w.id FROM websites w
		 WHERE NOT EXISTS (
			SELECT 1 FROM filter_rules fr WHERE fr.website_id = w.id
		 )`)
	if err != nil {
		return fmt.Errorf("query websites without rules: %w", err)
	}
	defer rows.Close()

	var websiteIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scan website id: %w", err)
		}
		websiteIDs = append(websiteIDs, id)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate websites: %w", err)
	}

	// Seed defaults for each website without rules
	for _, websiteID := range websiteIDs {
		if err := r.SeedDefaultFilterRules(ctx, websiteID); err != nil {
			r.logger.Warn("failed to seed defaults for website",
				logging.Field{Key: "website_id", Value: websiteID},
				logging.Field{Key: "error", Value: err.Error()})
			// Continue with other websites
		} else {
			r.logger.Info("seeded default filter rules for website",
				logging.Field{Key: "website_id", Value: websiteID})
		}
	}

	return nil
}
