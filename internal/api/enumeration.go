// Package api defines shared request/response types for the Moku API.
package api

// EnumerationConfig specifies which enumerators to run and their settings.
// Each enumerator is optional; presence indicates it should run.
type EnumerationConfig struct {
	Spider  *SpiderConfig  `json:"spider,omitempty" swaggertype:"object"`
	Sitemap *SitemapConfig `json:"sitemap,omitempty" swaggertype:"object"`
	Robots  *RobotsConfig  `json:"robots,omitempty" swaggertype:"object"`
	Wayback *WaybackConfig `json:"wayback,omitempty" swaggertype:"object"`
}

// SpiderConfig configures the spider enumerator.
type SpiderConfig struct {
	MaxDepth    int `json:"max_depth,omitempty" example:"4"`
	Concurrency int `json:"concurrency,omitempty" example:"5"`
}

// SitemapConfig enables sitemap enumeration. Presence indicates enabled.
type SitemapConfig struct{}

// RobotsConfig enables robots.txt enumeration. Presence indicates enabled.
type RobotsConfig struct{}

// WaybackConfig configures wayback/archive enumeration with per-source control.
type WaybackConfig struct {
	UseWaybackMachine *bool `json:"use_wayback_machine,omitempty" example:"true"`
	UseCommonCrawl    *bool `json:"use_common_crawl,omitempty" example:"true"`
}

// FetchConfig configures the fetcher with per-job settings.
type FetchConfig struct {
	Concurrency int `json:"concurrency,omitempty" example:"8"`
}
