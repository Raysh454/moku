package grading

import (
	"fmt"
	"slices"
	"strings"

	"github.com/raysh454/moku/internal/webclient"
)

// Signal is a single bot-detection tell evaluated against a fetched response.
// Implementations are strategies: each looks for one observable symptom (a body
// marker, a status code, a header value) so the panel of tells stays open for
// extension without touching the classifier.
type Signal interface {
	// Name identifies the tell in reports.
	Name() string
	// Evaluate reports whether the tell is present in resp. A nil resp must
	// never report a detection.
	Evaluate(resp *webclient.Response) SignalResult
}

// SignalResult is the outcome of evaluating one Signal against one response.
type SignalResult struct {
	Name     string `json:"name"`
	Detected bool   `json:"detected"`
	Evidence string `json:"evidence,omitempty"`
}

// bodyMarkerSignal detects any of a set of substrings in the response body,
// case-insensitively. Used for interstitial/challenge fingerprints.
type bodyMarkerSignal struct {
	name    string
	markers []string
}

// NewBodyMarkerSignal builds a Signal that fires when the response body contains
// any of the given markers (case-insensitive).
func NewBodyMarkerSignal(name string, markers ...string) Signal {
	return &bodyMarkerSignal{name: name, markers: markers}
}

func (s *bodyMarkerSignal) Name() string { return s.name }

func (s *bodyMarkerSignal) Evaluate(resp *webclient.Response) SignalResult {
	if resp == nil {
		return SignalResult{Name: s.name}
	}
	haystack := strings.ToLower(string(resp.Body))
	for _, marker := range s.markers {
		if strings.Contains(haystack, strings.ToLower(marker)) {
			return SignalResult{
				Name:     s.name,
				Detected: true,
				Evidence: fmt.Sprintf("body contains %q", marker),
			}
		}
	}
	return SignalResult{Name: s.name}
}

// statusSignal detects a response whose status code is one of a configured set.
type statusSignal struct {
	name  string
	codes []int
}

// NewStatusSignal builds a Signal that fires when the response status code
// matches any of the given codes.
func NewStatusSignal(name string, codes ...int) Signal {
	return &statusSignal{name: name, codes: codes}
}

func (s *statusSignal) Name() string { return s.name }

func (s *statusSignal) Evaluate(resp *webclient.Response) SignalResult {
	if resp == nil {
		return SignalResult{Name: s.name}
	}
	if slices.Contains(s.codes, resp.StatusCode) {
		return SignalResult{
			Name:     s.name,
			Detected: true,
			Evidence: fmt.Sprintf("status %d", resp.StatusCode),
		}
	}
	return SignalResult{Name: s.name}
}

// headerMarkerSignal detects any of a set of substrings in a named response
// header, case-insensitively (e.g. cf-mitigated: challenge).
type headerMarkerSignal struct {
	name    string
	header  string
	markers []string
}

// NewHeaderMarkerSignal builds a Signal that fires when the named header's value
// contains any of the given markers (case-insensitive).
func NewHeaderMarkerSignal(name, header string, markers ...string) Signal {
	return &headerMarkerSignal{name: name, header: header, markers: markers}
}

func (s *headerMarkerSignal) Name() string { return s.name }

func (s *headerMarkerSignal) Evaluate(resp *webclient.Response) SignalResult {
	if resp == nil || resp.Headers == nil {
		return SignalResult{Name: s.name}
	}
	value := strings.ToLower(resp.Headers.Get(s.header))
	if value == "" {
		return SignalResult{Name: s.name}
	}
	for _, marker := range s.markers {
		if strings.Contains(value, strings.ToLower(marker)) {
			return SignalResult{
				Name:     s.name,
				Detected: true,
				Evidence: fmt.Sprintf("%s: %s", s.header, resp.Headers.Get(s.header)),
			}
		}
	}
	return SignalResult{Name: s.name}
}
