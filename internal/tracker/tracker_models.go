package tracker

import (
	"time"

	"github.com/raysh454/moku/internal/assessor"
)

// Snapshot represents a captured document (HTML bytes + metadata).
type Snapshot struct {
	// ID is an opaque identifier (e.g., uuid or incremental string) assigned by the tracker.
	ID string `json:"id,omitempty"`

	// Status Code indicates the result of the snapshot attempt (e.g., 200, 404, error codes).
	StatusCode int `json:"status_code,omitempty"`

	// URL is the source URL for the snapshot when available.
	URL string `json:"url,omitempty"`

	// Body contains the raw HTML bytes for the snapshot.
	Body []byte `json:"body,omitempty"`

	// Headers contains optional header data (headers, content-type, etc).
	Headers map[string][]string `json:"headers,omitempty"`

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

// chunk represents a single change in a diff
type Chunk struct {
	Type    string `json:"type"`              // "added", "removed", "modified"
	Path    string `json:"path,omitempty"`    // optional path/selector
	Content string `json:"content,omitempty"` // content for the chunk
}

// BodyDiff represents the structured body diff.
type BodyDiff struct {
	BaseID string  `json:"base_id,omitempty"`
	HeadID string  `json:"head_id,omitempty"`
	Chunks []Chunk `json:"chunks"`
}

// CombinedDiff represents both body and header diffs combined.
type CombinedDiff struct {
	BodyDiff    BodyDiff   `json:"body_diff"`
	HeadersDiff HeaderDiff `json:"headers_diff"`
}

// HeaderDiff represents differences in headers between two versions.
type HeaderDiff struct {
	Added    map[string][]string `json:"added,omitempty"`
	Removed  map[string][]string `json:"removed,omitempty"`
	Changed  map[string]Change   `json:"changed,omitempty"`
	Redacted []string            `json:"redacted,omitempty"`
}

// Change represents a value change for a specific header.
type Change struct {
	From []string `json:"from"`
	To   []string `json:"to"`
}

type CommitResult struct {
	// Created Version (ID, Parent, SnapshotID, etc)
	Version Version

	// Parent version id (convenience; may be empty for initial commit)
	ParentVersionID string

	// Diff info: id of the diffs row (if created) and/or the serialized diff JSON.
	// DiffJSON is optional if the tracker will load the diff by DiffID.
	DiffID   string
	DiffJSON string

	// Head body data:
	// - HeadBody: optional in-memory body bytes (preferred if available)
	// - HeadBlobID: optional blob id in the tracker blob store (used to lazily load body)
	// - HeadFilePath: optional working-tree file path (if tracker keeps one)
	//
	// At least one of HeadBody or HeadBlobID / HeadFilePath should be provided
	// so scoring code can obtain the head HTML to run selectors / byte/line mapping.
	HeadBody     []byte
	HeadBlobID   string
	HeadFilePath string

	// Caller-provided default ScoreOptions to use when scoring this commit.
	// Callers may override at enqueue/run time.
	Opts assessor.ScoreOptions
}
