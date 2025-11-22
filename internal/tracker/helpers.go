package tracker

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
)

//go:embed schema.sql
var schemaFS embed.FS

// applySchema applies the SQLite schema to the database and sets appropriate pragmas.
func applySchema(db *sql.DB) error {
	// Set pragmas for better performance and safety
	pragmas := []string{
		"PRAGMA journal_mode=WAL",           // Write-Ahead Logging for better concurrency
		"PRAGMA synchronous=NORMAL",         // Balance between safety and performance
		"PRAGMA foreign_keys=ON",            // Enable foreign key constraints
		"PRAGMA busy_timeout=5000",          // Wait up to 5 seconds on locked database
		"PRAGMA cache_size=-64000",          // 64MB cache (negative means KB)
		"PRAGMA temp_store=MEMORY",          // Store temp tables in memory
		"PRAGMA mmap_size=268435456",        // 256MB memory-mapped I/O
		"PRAGMA auto_vacuum=INCREMENTAL",    // Incremental auto-vacuum
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("failed to set pragma %q: %w", pragma, err)
		}
	}

	// Read and execute schema
	schemaSQL, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("failed to read schema.sql: %w", err)
	}

	if _, err := db.Exec(string(schemaSQL)); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	return nil
}

// computeTextDiffJSON computes a diff between two byte slices and returns it as a JSON string.
// This is a placeholder implementation that will be replaced with actual diffing logic.
//
// TODO: Implement actual text diffing:
// - Line-based unified diff
// - Word-level diff for smaller changes
// - DOM-aware diff for HTML content
// - Header normalization and canonicalization
func computeTextDiffJSON(baseID, headID string, base, head []byte) (string, error) {
	// Placeholder: return empty diff indicating no changes detected
	// In a real implementation, this would:
	// 1. Parse both base and head as text or HTML
	// 2. Compute line-by-line or token-by-token differences
	// 3. Generate a structured diff with chunks
	// 4. Return as JSON
	
	diff := struct {
		BaseID string  `json:"base_id,omitempty"`
		HeadID string  `json:"head_id,omitempty"`
		Chunks []chunk `json:"chunks"`
	}{
		BaseID: baseID,
		HeadID: headID,
		Chunks: []chunk{},
	}

	// Example placeholder logic: if content is different, mark as modified
	if string(base) != string(head) {
		diff.Chunks = append(diff.Chunks, chunk{
			Type:    "modified",
			Path:    "",
			Content: "Content changed (diff not yet implemented)",
		})
	}

	data, err := json.Marshal(diff)
	if err != nil {
		return "", fmt.Errorf("failed to marshal diff: %w", err)
	}

	return string(data), nil
}

// chunk represents a single change in a diff
type chunk struct {
	Type    string `json:"type"`              // "added", "removed", "modified"
	Path    string `json:"path,omitempty"`    // optional path/selector
	Content string `json:"content,omitempty"` // content for the chunk
}

// normalizeHeaders normalizes HTTP headers for consistent comparison.
// TODO: Implement header canonicalization:
// - Remove volatile headers (Date, Set-Cookie, Expires, etc.)
// - Normalize header names (lowercase)
// - Sort headers alphabetically
// - Normalize whitespace in header values
func normalizeHeaders(headers map[string]string) map[string]string {
	// Placeholder: return headers as-is
	// Real implementation would filter and normalize
	return headers
}

// computeHeaderDiff compares two sets of headers and returns differences.
// TODO: Implement header-specific diffing that accounts for:
// - Semantic equivalence (e.g., different representations of same value)
// - Volatile vs. stable headers
// - Security-relevant headers (CSP, CORS, etc.)
func computeHeaderDiff(base, head map[string]string) string {
	// Placeholder: return empty diff
	return "{}"
}
