package tracker

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"strings"

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
	chunks := make([]chunk, 0)
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
			chunks = append(chunks, chunk{
				Type:    chunkType,
				Path:    "",
				Content: d.Text,
			})
		}
	}

	result := struct {
		BaseID string  `json:"base_id,omitempty"`
		HeadID string  `json:"head_id,omitempty"`
		Chunks []chunk `json:"chunks"`
	}{
		BaseID: baseID,
		HeadID: headID,
		Chunks: chunks,
	}

	data, err := json.Marshal(result)
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

// TODO: Future header normalization and diffing functions can be added here:
// - normalizeHeaders: Remove volatile headers, normalize names, sort alphabetically
// - computeHeaderDiff: Compare headers accounting for semantic equivalence
