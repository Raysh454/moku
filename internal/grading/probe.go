package grading

// Probe is a single target the harness fetches and grades: a human-readable
// name plus the URL to hit.
type Probe struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}
