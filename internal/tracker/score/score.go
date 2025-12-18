package score

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/logging"
	_ "modernc.org/sqlite"
)

// Minimal CombinedDiff view used here to avoid importing tracker.
// Only fields referenced in this file are included.
type CombinedDiff struct {
	BodyDiff struct {
		Chunks []struct {
			Content string `json:"content"`
		} `json:"chunks"`
	} `json:"body_diff"`
}

type ScoreAttributer struct {
	assessor assessor.Assessor
	db       *sql.DB
	logger   logging.Logger
}

func New(assessor assessor.Assessor, db *sql.DB, logger logging.Logger) *ScoreAttributer {
	return &ScoreAttributer{
		assessor: assessor,
		db:       db,
		logger:   logger,
	}
}

func (sa *ScoreAttributer) ScoreAndAttribute(ctx context.Context, opts assessor.ScoreOptions, versionID, parentVersionID, diffID, diffJSON string, headBody []byte) error {
	if sa.assessor == nil {
		return nil
	}

	scoreCtx, cancel := context.WithTimeout(ctx, opts.Timeout*time.Second)
	defer cancel()

	scoreRes, err := sa.assessor.ScoreHTML(scoreCtx, headBody, fmt.Sprintf("version:%s", versionID))
	if err != nil {
		if sa.logger != nil {
			sa.logger.Warn("scoring failed", logging.Field{Key: "version_id", Value: versionID}, logging.Field{Key: "error", Value: err})
		}
		return err
	}

	scoreJSON, err := json.Marshal(scoreRes)
	if err != nil {
		if sa.logger != nil {
			sa.logger.Warn("failed to marshal score result", logging.Field{Key: "err", Value: err})
		}
		scoreJSON = []byte("{}")
	}

	tx, err := sa.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rb := tx.Rollback(); rb != nil && rb != sql.ErrTxDone {
			if sa.logger != nil {
				sa.logger.Warn("rollback failed", logging.Field{Key: "err", Value: rb})
			}
		}
	}()

	scoreID := uuid.New().String()
	if _, err := tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO version_scores
		  (id, version_id, scoring_version, score, normalized, confidence, score_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, scoreID, versionID, scoreRes.Version, scoreRes.Score, scoreRes.Normalized, scoreRes.Confidence, string(scoreJSON), time.Now().Unix()); err != nil {
		return fmt.Errorf("insert version_scores: %w", err)
	}

	velMap, err := sa.persistVersionEvidenceLocations(ctx, tx, versionID, scoreRes)
	if err != nil {
		if sa.logger != nil {
			sa.logger.Warn("persistVersionEvidenceLocations failed", logging.Field{Key: "err", Value: err})
		}
	}

	if parentVersionID != "" && diffID != "" && diffJSON != "" {
		if err := sa.attributeUsingLocations(ctx, tx, diffID, versionID, parentVersionID, diffJSON, headBody, scoreRes, velMap); err != nil {
			if sa.logger != nil {
				sa.logger.Warn("attributeUsingLocations failed", logging.Field{Key: "err", Value: err})
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (sa *ScoreAttributer) persistVersionEvidenceLocations(ctx context.Context, tx *sql.Tx, versionID string, scoreRes *assessor.ScoreResult) (map[string]string, error) {
	if tx == nil {
		return nil, fmt.Errorf("persistVersionEvidenceLocations requires tx")
	}
	now := time.Now().Unix()
	velMap := make(map[string]string)

	keyFor := func(evidenceID string, locIndex int) string {
		return evidenceID + ":" + strconv.Itoa(locIndex)
	}

	for ei, ev := range scoreRes.Evidence {
		eid := ev.ID
		if eid == "" {
			eid = uuid.New().String()
			scoreRes.Evidence[ei].ID = eid
		}
		evJS, _ := json.Marshal(ev)
		evJSON := string(evJS)

		if len(ev.Locations) == 0 {
			id := uuid.New().String()
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO version_evidence_locations
				  (id, version_id, evidence_id, evidence_index, location_index, evidence_key, evidence_json, scoring_version, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, id, versionID, eid, ei, -1, ev.Key, evJSON, scoreRes.Version, now); err != nil {
				return nil, fmt.Errorf("insert version_evidence_locations global: %w", err)
			}
			velMap[keyFor(eid, -1)] = id
			continue
		}

		for li, loc := range ev.Locations {
			id := uuid.New().String()
			locJS, _ := json.Marshal(loc)
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO version_evidence_locations
				  (id, version_id, evidence_id, evidence_index, location_index,
				   evidence_key, selector, xpath, node_id, file_path,
				   byte_start, byte_end, line_start, line_end,
				   loc_confidence, evidence_json, location_json, scoring_version, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, id, versionID, eid, ei, li,
				ev.Key,
				loc.Selector, loc.XPath, loc.NodeID, loc.FilePath,
				nullableInt(loc.ByteStart), nullableInt(loc.ByteEnd), nullableInt(loc.LineStart), nullableInt(loc.LineEnd),
				nullableFloat(loc.Confidence), evJSON, string(locJS), scoreRes.Version, now); err != nil {
				return nil, fmt.Errorf("insert version_evidence_locations: %w", err)
			}
			velMap[keyFor(eid, li)] = id
		}
	}

	return velMap, nil
}

func (sa *ScoreAttributer) attributeUsingLocations(ctx context.Context, tx *sql.Tx, diffID, headVersionID, parentVersionID, diffJSON string, headBody []byte, scoreRes *assessor.ScoreResult, velMap map[string]string) error {
	if tx == nil {
		return fmt.Errorf("attributeUsingLocations requires tx")
	}

	combined := sa.parseCombinedDiff(diffJSON)
	doc := sa.prepareDoc(headBody)
	rows := sa.collectLocRows(combined, doc, headBody, scoreRes)

	if err := sa.insertAttributionRows(ctx, tx, diffID, headVersionID, parentVersionID, scoreRes, rows, velMap); err != nil {
		return err
	}

	return nil
}

type locRow struct {
	EvidenceID  string
	EvidenceKey string
	EvidenceIdx int
	LocationIdx int
	ChunkIdx    int
	Weight      float64
	EvidenceJS  string
	LocationJS  string
	Severity    string
}

func (sa *ScoreAttributer) collectLocRows(combined *CombinedDiff, doc *goquery.Document, headBody []byte, scoreRes *assessor.ScoreResult) []locRow {
	var rows []locRow
	for ei, ev := range scoreRes.Evidence {
		eid := ev.ID
		if eid == "" {
			eid = uuid.New().String()
			scoreRes.Evidence[ei].ID = eid
		}
		evJS, _ := json.Marshal(ev)
		evJSON := string(evJS)

		base := severityWeight(ev.Severity) * scoreRes.Confidence
		if base <= 0 {
			base = 1.0
		}

		if len(ev.Locations) == 0 {
			rows = append(rows, locRow{
				EvidenceID:  eid,
				EvidenceKey: ev.Key,
				EvidenceIdx: ei,
				LocationIdx: -1,
				ChunkIdx:    -1,
				Weight:      base,
				EvidenceJS:  evJSON,
				LocationJS:  "",
				Severity:    ev.Severity,
			})
			continue
		}

		locConfs := make([]float64, len(ev.Locations))
		sum := 0.0
		for i, l := range ev.Locations {
			if l.Confidence != nil && *l.Confidence >= 0 {
				locConfs[i] = *l.Confidence
			} else {
				locConfs[i] = 1.0
			}
			sum += locConfs[i]
		}
		if sum == 0 {
			for i := range locConfs {
				locConfs[i] = 1.0
			}
			sum = float64(len(locConfs))
		}

		for li, loc := range ev.Locations {
			chunkIdx, _ := sa.mapLocationToChunkStrict(combined, doc, headBody, loc)
			locJS, _ := json.Marshal(loc)
			rows = append(rows, locRow{
				EvidenceID:  eid,
				EvidenceKey: ev.Key,
				EvidenceIdx: ei,
				LocationIdx: li,
				ChunkIdx:    chunkIdx,
				Weight:      base * (locConfs[li] / sum),
				EvidenceJS:  evJSON,
				LocationJS:  string(locJS),
				Severity:    ev.Severity,
			})
		}
	}
	return rows
}

func (sa *ScoreAttributer) insertAttributionRows(ctx context.Context, tx *sql.Tx, diffID, headVersionID, parentVersionID string, scoreRes *assessor.ScoreResult, rows []locRow, velMap map[string]string) error {
	total := 0.0
	for _, r := range rows {
		total += r.Weight
	}
	if total <= 0 {
		total = 1.0
	}

	now := time.Now().Unix()
	for _, r := range rows {
		pct := (r.Weight / total) * 100.0
		id := uuid.New().String()

		velID := ""
		if velMap != nil {
			key := r.EvidenceID + ":" + strconv.Itoa(r.LocationIdx)
			if v, ok := velMap[key]; ok {
				velID = v
			}
			if velID == "" {
				if v, ok := velMap[r.EvidenceID+":-1"]; ok {
					velID = v
				}
			}
		}

		_, err := tx.ExecContext(ctx, `
			INSERT INTO diff_attributions
			  (id, diff_id, head_version_id, version_evidence_location_id, evidence_id, evidence_location_index, chunk_index, evidence_key, evidence_json, location_json, scoring_version, contribution, contribution_pct, note, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, id, diffID, headVersionID, nullableString(velID), r.EvidenceID, r.LocationIdx, r.ChunkIdx, r.EvidenceKey, r.EvidenceJS, r.LocationJS, scoreRes.Version, r.Weight, pct, "", now)
		if err != nil && sa.logger != nil {
			sa.logger.Warn("failed to insert diff_attribution", logging.Field{Key: "err", Value: err}, logging.Field{Key: "evidence_id", Value: r.EvidenceID})
		}
	}

	var parentScore sql.NullFloat64
	if err := tx.QueryRowContext(ctx, `SELECT score FROM version_scores WHERE version_id = ? ORDER BY created_at DESC LIMIT 1`, parentVersionID).Scan(&parentScore); err != nil && err != sql.ErrNoRows {
		if sa.logger != nil {
			sa.logger.Warn("failed to query parent score", logging.Field{Key: "err", Value: err})
		}
		parentScore = sql.NullFloat64{}
	}
	if parentScore.Valid && sa.logger != nil {
		delta := scoreRes.Score - parentScore.Float64
		sa.logger.Info("version scored", logging.Field{Key: "version_id", Value: headVersionID}, logging.Field{Key: "score", Value: scoreRes.Score}, logging.Field{Key: "parent_score", Value: parentScore.Float64}, logging.Field{Key: "delta", Value: delta})
	}

	return nil
}

// ---------------- mapping helpers ----------------

// mapLocationToChunkStrict maps a structured EvidenceLocation to a chunk index using only
// selector / byte range / line range matching. Returns chunk index (>=0) or -1 if none matched.
// Strength return value is unused here (kept for parity); returns 1.0 on exact match, 0.0 otherwise.
func (sa *ScoreAttributer) mapLocationToChunkStrict(combined *CombinedDiff, doc *goquery.Document, headBody []byte, loc assessor.EvidenceLocation) (int, float64) {
	// prefer selector -> byte-range -> line-range
	if loc.Selector != "" && doc != nil {
		if idx, ok := sa.matchSelectorToChunks(combined, doc, loc.Selector); ok {
			return idx, 1.0
		}
	}
	if loc.ByteStart != nil && loc.ByteEnd != nil && len(headBody) > 0 {
		if idx, ok := sa.matchByteRangeToChunks(combined, headBody, *loc.ByteStart, *loc.ByteEnd); ok {
			return idx, 1.0
		}
	}
	if loc.LineStart != nil && loc.LineEnd != nil && len(headBody) > 0 {
		if idx, ok := sa.matchLineRangeToChunks(combined, headBody, *loc.LineStart, *loc.LineEnd); ok {
			return idx, 1.0
		}
	}
	return -1, 0.0
}

// matchSelectorToChunks finds the first chunk containing the outer HTML (or text) of the first node matched by selector.
func (sa *ScoreAttributer) matchSelectorToChunks(combined *CombinedDiff, doc *goquery.Document, selector string) (int, bool) {
	if doc == nil {
		return -1, false
	}
	nodes := doc.Find(selector)
	if nodes.Length() == 0 {
		return -1, false
	}
	htmlSnippet, err := nodes.First().Html()
	if err != nil || strings.TrimSpace(htmlSnippet) == "" {
		htmlSnippet = nodes.First().Text()
	}
	sn := strings.ToLower(strings.TrimSpace(htmlSnippet))
	if sn == "" {
		return -1, false
	}
	for i, c := range combined.BodyDiff.Chunks {
		if strings.Contains(strings.ToLower(c.Content), sn) {
			return i, true
		}
	}
	return -1, false
}

// matchByteRangeToChunks extracts the snippet from headBody and finds the first chunk containing it.
func (sa *ScoreAttributer) matchByteRangeToChunks(combined *CombinedDiff, headBody []byte, start, end int) (int, bool) {
	if start < 0 {
		start = 0
	}
	if end > len(headBody) {
		end = len(headBody)
	}
	if start >= end {
		return -1, false
	}
	sn := strings.ToLower(strings.TrimSpace(string(headBody[start:end])))
	if sn == "" {
		return -1, false
	}
	for i, c := range combined.BodyDiff.Chunks {
		if strings.Contains(strings.ToLower(c.Content), sn) {
			return i, true
		}
	}
	return -1, false
}

// matchLineRangeToChunks extracts the lines and finds the first chunk containing that snippet.
func (sa *ScoreAttributer) matchLineRangeToChunks(combined *CombinedDiff, headBody []byte, lineStart, lineEnd int) (int, bool) {
	lines := bytes.Split(headBody, []byte{'\n'})
	ls := lineStart - 1
	le := lineEnd - 1
	if ls < 0 {
		ls = 0
	}
	if le >= len(lines) {
		le = len(lines) - 1
	}
	if ls > le || ls >= len(lines) {
		return -1, false
	}
	sn := strings.ToLower(strings.TrimSpace(string(bytes.Join(lines[ls:le+1], []byte{'\n'}))))
	if sn == "" {
		return -1, false
	}
	for i, c := range combined.BodyDiff.Chunks {
		if strings.Contains(strings.ToLower(c.Content), sn) {
			return i, true
		}
	}
	return -1, false
}

// ---------------- small utilities ----------------

func (sa *ScoreAttributer) parseCombinedDiff(diffJSON string) *CombinedDiff {
	var combined CombinedDiff
	_ = json.Unmarshal([]byte(diffJSON), &combined)
	return &combined
}

func (sa *ScoreAttributer) prepareDoc(headBody []byte) *goquery.Document {
	if len(headBody) == 0 {
		return nil
	}
	if d, err := goquery.NewDocumentFromReader(bytes.NewReader(headBody)); err == nil {
		return d
	}
	return nil
}

func nullableInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullableFloat(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

// severityWeight maps severity string to numeric weight.
func severityWeight(s string) float64 {
	switch strings.ToLower(s) {
	case "critical":
		return 5.0
	case "high":
		return 3.0
	case "medium":
		return 2.0
	case "low":
		return 1.0
	default:
		return 1.0
	}
}

func nullableString(s string) sql.NullString {
	return sql.NullString{
		String: s,
		Valid:  s != "",
	}
}
