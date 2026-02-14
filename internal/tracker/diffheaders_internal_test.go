package tracker

import (
	"testing"
)

func TestDiffHeaders_Added(t *testing.T) {
	t.Parallel()
	base := map[string][]string{"content-type": {"text/html"}}
	head := map[string][]string{"content-type": {"text/html"}, "cache-control": {"no-cache"}}

	diff := diffHeaders(base, head, true)

	if len(diff.Added) == 0 {
		t.Fatal("expected added headers")
	}
	if _, ok := diff.Added["cache-control"]; !ok {
		t.Error("expected cache-control in added")
	}
}

func TestDiffHeaders_Removed(t *testing.T) {
	t.Parallel()
	base := map[string][]string{"content-type": {"text/html"}, "cache-control": {"no-cache"}}
	head := map[string][]string{"content-type": {"text/html"}}

	diff := diffHeaders(base, head, true)

	if len(diff.Removed) == 0 {
		t.Fatal("expected removed headers")
	}
	if _, ok := diff.Removed["cache-control"]; !ok {
		t.Error("expected cache-control in removed")
	}
}

func TestDiffHeaders_Changed(t *testing.T) {
	t.Parallel()
	base := map[string][]string{"content-type": {"text/html"}}
	head := map[string][]string{"content-type": {"application/json"}}

	diff := diffHeaders(base, head, false)

	if len(diff.Changed) == 0 {
		t.Fatal("expected changed headers")
	}
	c, ok := diff.Changed["content-type"]
	if !ok {
		t.Fatal("expected content-type in changed")
	}
	if c.From[0] != "text/html" || c.To[0] != "application/json" {
		t.Errorf("unexpected change: from=%v to=%v", c.From, c.To)
	}
}

func TestDiffHeaders_SensitiveRedacted(t *testing.T) {
	t.Parallel()
	base := map[string][]string{"authorization": {"Bearer old"}}
	head := map[string][]string{"authorization": {"Bearer new"}}

	diff := diffHeaders(base, head, true)

	if len(diff.Redacted) == 0 {
		t.Fatal("expected redacted headers when sensitive header changes")
	}
	found := false
	for _, r := range diff.Redacted {
		if r == "authorization" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'authorization' in redacted list")
	}
}

func TestDiffHeaders_NoChanges(t *testing.T) {
	t.Parallel()
	h := map[string][]string{"content-type": {"text/html"}}

	diff := diffHeaders(h, h, false)

	if len(diff.Added) != 0 || len(diff.Removed) != 0 || len(diff.Changed) != 0 {
		t.Errorf("expected no diffs, got added=%d removed=%d changed=%d",
			len(diff.Added), len(diff.Removed), len(diff.Changed))
	}
}

func TestDiffHeaders_MultipleChanges(t *testing.T) {
	t.Parallel()
	base := map[string][]string{
		"content-type":  {"text/html"},
		"cache-control": {"no-cache"},
		"accept":        {"text/html"},
	}
	head := map[string][]string{
		"content-type": {"application/json"},
		"accept":       {"text/html"},
		"server":       {"nginx"},
	}

	diff := diffHeaders(base, head, false)

	if len(diff.Added) == 0 {
		t.Error("expected added headers (server)")
	}
	if len(diff.Removed) == 0 {
		t.Error("expected removed headers (cache-control)")
	}
	if len(diff.Changed) == 0 {
		t.Error("expected changed headers (content-type)")
	}
}

func TestDiffHeaders_NilBase(t *testing.T) {
	t.Parallel()
	head := map[string][]string{"content-type": {"text/html"}}

	diff := diffHeaders(nil, head, false)

	if len(diff.Added) == 0 {
		t.Error("expected all head headers to be marked as added when base is nil")
	}
}

func TestDiffHeaders_NilHead(t *testing.T) {
	t.Parallel()
	base := map[string][]string{"content-type": {"text/html"}}

	diff := diffHeaders(base, nil, false)

	if len(diff.Removed) == 0 {
		t.Error("expected all base headers to be marked as removed when head is nil")
	}
}

func TestDiffHeaders_BothNil(t *testing.T) {
	t.Parallel()
	diff := diffHeaders(nil, nil, false)

	if len(diff.Added) != 0 || len(diff.Removed) != 0 || len(diff.Changed) != 0 {
		t.Error("expected empty diff when both nil")
	}
}

func TestDiffHeaders_EmptyMaps(t *testing.T) {
	t.Parallel()
	diff := diffHeaders(map[string][]string{}, map[string][]string{}, false)

	if len(diff.Added) != 0 || len(diff.Removed) != 0 || len(diff.Changed) != 0 {
		t.Error("expected empty diff for empty header maps")
	}
}
