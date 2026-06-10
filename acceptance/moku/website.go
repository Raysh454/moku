package moku

import "time"

// Website is a monitored website registered under a project.
type Website struct {
	project *Project
	slug    string
	origin  string

	lastFetchStarted time.Time
}

// Slug returns the website's auto-assigned slug.
func (w *Website) Slug() string {
	return w.slug
}

func (w *Website) path(suffix string) string {
	return w.project.path("/websites/" + w.slug + suffix)
}
