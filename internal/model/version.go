package model

import "time"

// Snapshot represents a captured document (HTML bytes + metadata).
type Snapshot struct {
	// ID is an opaque identifier (e.g., uuid or incremental string) assigned by the tracker.
	ID string `json:"id,omitempty"`

	// URL is the source URL for the snapshot when available.
	URL string `json:"url,omitempty"`

	// Body contains the raw HTML bytes for the snapshot.
	Body []byte `json:"body,omitempty"`

	// Meta contains optional metadata (headers, content-type, etc).
	Meta map[string]string `json:"meta,omitempty"`

	// CreatedAt is the capture timestamp (if known).
	CreatedAt time.Time `json:"created_at"`
}

// Version represents a commit/entry in the tracker history.
type Version struct {
	// ID is the version identifier (string).
	ID string `json:"id"`

	// Parent is the parent version ID (empty if none).
	Parent string `json:"parent,omitempty"`

	// Message is the commit message.
	Message string `json:"message,omitempty"`

	// Author is optional metadata about who committed.
	Author string `json:"author,omitempty"`

	// SnapshotID references the stored snapshot (may match ID).
	SnapshotID string `json:"snapshot_id,omitempty"`

	// Timestamp is the commit time.
	Timestamp time.Time `json:"timestamp"`
}

// DiffChunk represents a single change chunk between two snapshots.
// For a web-focused tracker, chunks could be at the HTML/text level or DOM-level.
// Keep it simple initially: Type = "added"|"removed"|"modified", Path = optional selector.
type DiffChunk struct {
	Type    string `json:"type"`              // e.g., "added", "removed", "modified"
	Path    string `json:"path,omitempty"`    // optional DOM selector or descriptor
	Content string `json:"content,omitempty"` // content for added/modified chunks
}

// DiffResult summarizes differences between two versions.
type DiffResult struct {
	BaseID string      `json:"base_id,omitempty"`
	HeadID string      `json:"head_id,omitempty"`
	Chunks []DiffChunk `json:"chunks,omitempty"`
}
