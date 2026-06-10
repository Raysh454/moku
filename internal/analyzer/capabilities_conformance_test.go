package analyzer_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/raysh454/moku/internal/analyzer"
)

// manifestCaps mirrors the comparable fields of the shared capabilities
// manifest (testdata/capabilities.json). Version is intentionally omitted: the
// Go and Python sides each stamp their own.
type manifestCaps struct {
	Async               bool `json:"async"`
	SupportsAuth        bool `json:"supports_auth"`
	SupportsScope       bool `json:"supports_scope"`
	SupportsScanProfile bool `json:"supports_scan_profile"`
	MaxConcurrentScans  int  `json:"max_concurrent_scans"`
}

func loadCapabilitiesManifest(t *testing.T) map[string]manifestCaps {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "capabilities.json"))
	if err != nil {
		t.Fatalf("read capabilities manifest: %v", err)
	}
	var rawEntries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawEntries); err != nil {
		t.Fatalf("unmarshal capabilities manifest: %v", err)
	}
	out := make(map[string]manifestCaps)
	for name, entry := range rawEntries {
		if name == "_comment" {
			continue
		}
		var mc manifestCaps
		if err := json.Unmarshal(entry, &mc); err != nil {
			t.Fatalf("unmarshal manifest entry %q: %v", name, err)
		}
		out[name] = mc
	}
	return out
}

// sidecarBackendByAdapter maps the sidecar adapter name to the Go Backend that
// routes to it. Every adapter in the shared manifest must have a Go route.
var sidecarBackendByAdapter = map[string]analyzer.Backend{
	"builtin":    analyzer.BackendDAST,
	"nuclei":     analyzer.BackendNuclei,
	"nikto":      analyzer.BackendNikto,
	"shodan":     analyzer.BackendShodan,
	"virustotal": analyzer.BackendVirusTotal,
	"zap":        analyzer.BackendZAP,
}

// TestSidecarCapabilities_MatchSharedManifest guards against Go/Python
// capability drift: the Go client's Capabilities() for every sidecar backend
// must equal the shared manifest that the Python adapters are also checked
// against (see services/analyzer/tests/test_capabilities_conformance.py).
func TestSidecarCapabilities_MatchSharedManifest(t *testing.T) {
	manifest := loadCapabilitiesManifest(t)
	for adapter, want := range manifest {
		backend, routed := sidecarBackendByAdapter[adapter]
		if !routed {
			t.Errorf("manifest adapter %q has no Go Backend route", adapter)
			continue
		}
		t.Run(adapter, func(t *testing.T) {
			env := newSidecarEnv(t)
			a := env.newAnalyzerWith(t, backend, analyzer.SidecarConfig{BaseURL: env.server.URL})
			got := a.Capabilities()
			if got.Async != want.Async ||
				got.SupportsAuth != want.SupportsAuth ||
				got.SupportsScope != want.SupportsScope ||
				got.SupportsScanProfile != want.SupportsScanProfile ||
				got.MaxConcurrentScans != want.MaxConcurrentScans {
				t.Errorf(
					"adapter %s: Capabilities()=%+v does not match manifest=%+v",
					adapter, got, want,
				)
			}
		})
	}
}
