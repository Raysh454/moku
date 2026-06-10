package moku

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/raysh454/moku/acceptance/internal/harness"
)

const newVersionTimeout = 90 * time.Second

// EndpointRef names one endpoint of a monitored website. It performs no I/O
// itself; observations are taken from it.
type EndpointRef struct {
	website *Website
	url     string
}

// Observation is one point-in-time read of an endpoint's latest tracked
// state: the snapshot moku serves plus the diffs it computed against the
// previous version.
type Observation struct {
	ref       *EndpointRef
	versionID string
	details   endpointDetails
}

// endpointDetails mirrors the documented GET .../endpoints/details response.
// The structs are defined here, not imported from the server, so the suite
// asserts the wire contract — if the API drifts, these tests fail.
type endpointDetails struct {
	Snapshot *struct {
		VersionID  string              `json:"version_id"`
		StatusCode int                 `json:"status_code"`
		Body       []byte              `json:"body"`
		Headers    map[string][]string `json:"headers"`
	} `json:"snapshot"`
	SecurityDiff *struct {
		BaseVersionID        string `json:"base_version_id"`
		HeadVersionID        string `json:"head_version_id"`
		AttackSurfaceChanged bool   `json:"attack_surface_changed"`
	} `json:"security_diff"`
	Diff *struct {
		BodyDiff struct {
			Chunks []struct {
				Type string `json:"type"`
			} `json:"chunks"`
		} `json:"body_diff"`
	} `json:"diff"`
}

// ThenHasEndpoint asserts that the website's endpoint index contains the
// given path after enumeration and fetching.
func (w *Website) ThenHasEndpoint(t *testing.T, path string) {
	t.Helper()

	var endpoints []struct {
		CanonicalURL string `json:"canonical_url"`
	}
	w.server().api.MustStatus(t, http.StatusOK, http.MethodGet,
		w.path("/endpoints?status=*&limit=200"), nil, &endpoints)

	wanted := w.origin + path
	for _, endpoint := range endpoints {
		if endpoint.CanonicalURL == wanted {
			return
		}
	}
	t.Fatalf("endpoint %q not found among %d indexed endpoint(s): %+v", wanted, len(endpoints), endpoints)
}

// Endpoint names an endpoint by path, e.g. "/" or "/admin".
func (w *Website) Endpoint(path string) *EndpointRef {
	return &EndpointRef{website: w, url: w.origin + path}
}

// Snapshot reads the endpoint's current tracked state and fails if no
// snapshot exists yet.
func (e *EndpointRef) Snapshot(t *testing.T) Observation {
	t.Helper()

	details := e.fetchDetails(t)
	if details.Snapshot == nil {
		t.Fatalf("endpoint %s has no snapshot yet", e.url)
	}
	return Observation{ref: e, versionID: details.Snapshot.VersionID, details: details}
}

// WaitForNewVersion blocks until the endpoint's tracked snapshot version
// differs from the given earlier observation, then returns the new state.
func (e *EndpointRef) WaitForNewVersion(t *testing.T, since Observation) Observation {
	t.Helper()

	var details endpointDetails
	harness.WaitUntil(t, newVersionTimeout, func() (bool, string, error) {
		details = e.fetchDetails(t)
		if details.Snapshot == nil {
			return false, "waiting for a snapshot to appear", nil
		}
		if details.Snapshot.VersionID == since.versionID {
			return false, "waiting for snapshot version to change", nil
		}
		return true, "new version observed", nil
	})
	return Observation{ref: e, versionID: details.Snapshot.VersionID, details: details}
}

func (e *EndpointRef) fetchDetails(t *testing.T) endpointDetails {
	t.Helper()
	var details endpointDetails
	e.website.server().api.MustStatus(t, http.StatusOK, http.MethodGet,
		e.website.path("/endpoints/details?url="+url.QueryEscape(e.url)), nil, &details)
	return details
}

// ThenServedOK asserts the snapshot recorded an HTTP 200 from the target.
func (o Observation) ThenServedOK(t *testing.T) {
	t.Helper()
	if o.details.Snapshot.StatusCode != http.StatusOK {
		t.Fatalf("endpoint %s: snapshot status = %d, want %d", o.ref.url, o.details.Snapshot.StatusCode, http.StatusOK)
	}
}

// ThenBodyContains asserts the snapshot body contains the given marker.
func (o Observation) ThenBodyContains(t *testing.T, marker string) {
	t.Helper()
	body := string(o.details.Snapshot.Body)
	if !strings.Contains(body, marker) {
		t.Fatalf("endpoint %s: body does not contain %q; body=%s", o.ref.url, marker, body)
	}
}

// ThenBodyDiffRecorded asserts moku computed a non-empty body diff against
// the previous version.
func (o Observation) ThenBodyDiffRecorded(t *testing.T) {
	t.Helper()
	if o.details.Diff == nil {
		t.Fatalf("endpoint %s: expected a diff against the previous version, got none", o.ref.url)
	}
	if len(o.details.Diff.BodyDiff.Chunks) == 0 {
		t.Fatalf("endpoint %s: body diff has no chunks", o.ref.url)
	}
}

// ThenAttackSurfaceChangedSince asserts moku's security diff spans exactly
// the two observed versions and flags an attack-surface change.
func (o Observation) ThenAttackSurfaceChangedSince(t *testing.T, base Observation) {
	t.Helper()

	diff := o.details.SecurityDiff
	if diff == nil {
		t.Fatalf("endpoint %s: expected a security diff, got none", o.ref.url)
	}
	if diff.BaseVersionID != base.versionID {
		t.Fatalf("endpoint %s: security diff base version = %q, want %q", o.ref.url, diff.BaseVersionID, base.versionID)
	}
	if diff.HeadVersionID != o.versionID {
		t.Fatalf("endpoint %s: security diff head version = %q, want %q", o.ref.url, diff.HeadVersionID, o.versionID)
	}
	if !diff.AttackSurfaceChanged {
		t.Fatalf("endpoint %s: expected attack surface to be flagged as changed between %q and %q", o.ref.url, base.versionID, o.versionID)
	}
}

// ThenHasSecurityHeader asserts the snapshot recorded the given response
// header with the given value.
func (o Observation) ThenHasSecurityHeader(t *testing.T, name, value string) {
	t.Helper()
	values, ok := o.details.Snapshot.Headers[name]
	if !ok || len(values) == 0 || values[0] != value {
		t.Fatalf("endpoint %s: expected header %s=%s; headers=%v", o.ref.url, name, value, o.details.Snapshot.Headers)
	}
}
