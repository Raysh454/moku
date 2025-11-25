package model

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
	Opts ScoreOptions
}
