package tracker

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/raysh454/moku/internal/model"
	"github.com/sergi/go-diff/diffmatchpatch"
)

//go:embed schema.sql
var schemaFS embed.FS

// applySchema applies the SQLite schema to the database and sets appropriate pragmas.
func applySchema(db *sql.DB) error {
	// Set pragmas for better performance and safety
	pragmas := []string{
		"PRAGMA journal_mode=WAL",        // Write-Ahead Logging for better concurrency
		"PRAGMA synchronous=NORMAL",      // Balance between safety and performance
		"PRAGMA foreign_keys=ON",         // Enable foreign key constraints
		"PRAGMA busy_timeout=5000",       // Wait up to 5 seconds on locked database
		"PRAGMA cache_size=-64000",       // 64MB cache (negative means KB)
		"PRAGMA temp_store=MEMORY",       // Store temp tables in memory
		"PRAGMA mmap_size=268435456",     // 256MB memory-mapped I/O
		"PRAGMA auto_vacuum=INCREMENTAL", // Incremental auto-vacuum
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
// Uses the diffmatchpatch library for efficient text diffing.
//
// The diff is computed at the character level for HTML content, which allows for
// precise change detection. For very large documents, consider line-based diffing.
func computeTextDiffJSON(baseID, headID string, base, head []byte) (string, error) {
	dmp := diffmatchpatch.New()

	// Convert to strings for diffing
	baseStr := string(base)
	headStr := string(head)

	// Compute diffs at character level
	diffs := dmp.DiffMain(baseStr, headStr, true)

	// Clean up the diffs for better readability
	diffs = dmp.DiffCleanupSemantic(diffs)

	// Convert diffs to our chunk format
	chunks := make([]model.Chunk, 0)
	for _, d := range diffs {
		var chunkType string
		switch d.Type {
		case diffmatchpatch.DiffInsert:
			chunkType = "added"
		case diffmatchpatch.DiffDelete:
			chunkType = "removed"
		case diffmatchpatch.DiffEqual:
			// Skip equal chunks unless we want to show context
			continue
		}

		// Only include non-empty chunks
		if strings.TrimSpace(d.Text) != "" {
			chunks = append(chunks, model.Chunk{
				Type:    chunkType,
				Path:    "",
				Content: d.Text,
			})
		}
	}

	// Return structured body diff
	bodyDiff := model.BodyDiff{
		BaseID: baseID,
		HeadID: headID,
		Chunks: chunks,
	}

	data, err := json.Marshal(bodyDiff)
	if err != nil {
		return "", fmt.Errorf("failed to marshal diff: %w", err)
	}

	return string(data), nil
}

// computemodel.CombinedDiff computes both body and header diffs and combines them.
// If redactSensitive is true, sensitive headers are marked as redacted in the diff.
func computeCombinedDiff(baseID, headID string, baseBody, headBody []byte, baseHeaders, headHeaders map[string][]string, redactSensitive bool) (string, error) {
	// Compute body diff
	bodyDiffJSON, err := computeTextDiffJSON(baseID, headID, baseBody, headBody)
	if err != nil {
		return "", fmt.Errorf("failed to compute body diff: %w", err)
	}

	var bodyDiff model.BodyDiff
	if err := json.Unmarshal([]byte(bodyDiffJSON), &bodyDiff); err != nil {
		return "", fmt.Errorf("failed to unmarshal body diff: %w", err)
	}

	// Compute header diff
	headersDiff := diffHeaders(baseHeaders, headHeaders, redactSensitive)

	// Combine both diffs
	combined := model.CombinedDiff{
		BodyDiff:    bodyDiff,
		HeadersDiff: headersDiff,
	}

	data, err := json.Marshal(combined)
	if err != nil {
		return "", fmt.Errorf("failed to marshal combined diff: %w", err)
	}

	return string(data), nil
}

// normalizeHeaders normalizes HTTP headers for consistent storage and comparison.
// It lowercases header names, trims whitespace from values, and sorts multi-value
// headers where order doesn't matter. For headers where order is significant
// (like Set-Cookie), the original order is preserved.
// Headers with the same normalized name are merged into a single entry, which is
// correct behavior per RFC 7230 (HTTP headers are case-insensitive).
// If redactSensitive is true, sensitive headers are replaced with "[REDACTED]".
func normalizeHeaders(h map[string][]string, redactSensitive bool) map[string][]string {
	if h == nil {
		return make(map[string][]string)
	}

	// First pass: collect all values for each normalized header name
	// This correctly merges headers with different cases (e.g., "Content-Type" and "content-type")
	// into a single normalized entry, per HTTP specification
	normalized := make(map[string][]string)
	for name, values := range h {
		// Lowercase the header name for case-insensitive comparison
		normName := strings.ToLower(name)

		// Trim whitespace from values
		for _, v := range values {
			trimmed := strings.TrimSpace(v)
			if trimmed != "" {
				normalized[normName] = append(normalized[normName], trimmed)
			}
		}
	}

	// Second pass: redact sensitive headers (if enabled) and sort values
	for normName, values := range normalized {
		// Redact sensitive headers if enabled
		if redactSensitive && isSensitiveHeader(normName) {
			normalized[normName] = []string{"[REDACTED]"}
			continue
		}

		// For headers where order matters, preserve original order
		// For others, sort values for consistent comparison
		if !isOrderSensitiveHeader(normName) {
			sort.Strings(values)
		}
	}

	return normalized
}

// isSensitiveHeader returns true if the header contains sensitive data that should be redacted.
func isSensitiveHeader(name string) bool {
	name = strings.ToLower(name)
	sensitiveHeaders := []string{
		"authorization",
		"cookie",
		"set-cookie",
		"proxy-authorization",
		"www-authenticate",
		"proxy-authenticate",
		"x-api-key",
		"x-auth-token",
	}

	return slices.Contains(sensitiveHeaders, name)
}

// isOrderSensitiveHeader returns true if the order of header values is significant.
func isOrderSensitiveHeader(name string) bool {
	name = strings.ToLower(name)
	orderSensitiveHeaders := []string{
		"set-cookie",
		"www-authenticate",
		"proxy-authenticate",
	}

	return slices.Contains(orderSensitiveHeaders, name)
}


// diffHeaders computes a structured diff between two sets of normalized headers.
// If redactSensitive is true, sensitive headers are marked as redacted in the diff.
func diffHeaders(base, head map[string][]string, redactSensitive bool) model.HeaderDiff {
	diff := model.HeaderDiff{
		Added:    make(map[string][]string),
		Removed:  make(map[string][]string),
		Changed:  make(map[string]model.Change),
		Redacted: make([]string, 0),
	}

	// Normalize both header sets
	baseNorm := normalizeHeaders(base, redactSensitive)
	headNorm := normalizeHeaders(head, redactSensitive)

	// Track unique redacted headers using a map
	redactedSet := make(map[string]bool)

	// Find added and changed headers
	for name, headValues := range headNorm {
		if isRedacted(headValues) {
			redactedSet[name] = true
			continue
		}

		baseValues, existsInBase := baseNorm[name]
		if !existsInBase {
			// Header added in head
			diff.Added[name] = headValues
		} else if !equalStringSlices(baseValues, headValues) {
			// Header changed
			diff.Changed[name] = model.Change{
				From: baseValues,
				To:   headValues,
			}
		}
	}

	// Find removed headers
	for name, baseValues := range baseNorm {
		if isRedacted(baseValues) {
			if _, existsInHead := headNorm[name]; !existsInHead {
				redactedSet[name] = true
			}
			continue
		}

		if _, existsInHead := headNorm[name]; !existsInHead {
			diff.Removed[name] = baseValues
		}
	}

	// Convert redacted set to sorted slice
	for name := range redactedSet {
		diff.Redacted = append(diff.Redacted, name)
	}
	sort.Strings(diff.Redacted)

	return diff
}

// isRedacted checks if header values are redacted.
func isRedacted(values []string) bool {
	return len(values) == 1 && values[0] == "[REDACTED]"
}

// equalStringSlices compares two string slices for equality.
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
