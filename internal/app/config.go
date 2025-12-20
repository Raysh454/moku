package app

import (
	"github.com/raysh454/moku/internal/assessor"
	"github.com/raysh454/moku/internal/fetcher"
	"github.com/raysh454/moku/internal/server"
	"github.com/raysh454/moku/internal/tracker"
	"github.com/raysh454/moku/internal/utils"
	"github.com/raysh454/moku/internal/webclient"
)

// Config contains a minimal set of runtime configuration options required by
// internal modules during initial development. We intentionally keep this small
// for the dev branch — add more fields later as wiring requires them.
type Config struct {
	ServerCfg server.Config

	// StorageRoot is the base path where projects are kept.
	StorageRoot string

	// Tracker Configuration
	trackerCfg tracker.Config

	// Fetcher Configuration
	FetcherCfg fetcher.Config

	// WebClient configuration
	WebClientCfg webclient.WebClientConfig

	// Assessor configuration
	assessorCfg assessor.Config

	// Url Parsing Options
	urlCfg utils.CanonicalizeOptions
}

// DefaultConfig returns a Config populated with sensible development defaults.
func DefaultConfig() *Config {
	return &Config{
		ServerCfg: server.Config{
			ServerAddr: "http://localhost:8080",
		},
		StorageRoot: "~/.config/moku",
		trackerCfg: tracker.Config{
			RedactSensitiveHeaders: false,
			StoragePath:            "",    // Needs to be set! (Website Directory)
			ProjectID:              "",    // Needs to be set! (Project Identifier for website)
			ForceProjectID:         false, // Needs to be set! (Whether to overwrite existing project ID)
		},
		FetcherCfg: fetcher.Config{
			MaxConcurrency: 4,
			CommitSize:     10,
			ScoreTimeout:   30,
		},
		WebClientCfg: webclient.WebClientConfig{
			Client: webclient.ClientNetHTTP,
		},
		assessorCfg: assessor.Config{
			ScoringVersion:    "v0.1.0",
			DefaultConfidence: 0.5,
			ScoreOpts: assessor.ScoreOptions{
				RequestLocations: true,
			},
		},
		urlCfg: utils.CanonicalizeOptions{
			DropTrackingParams:     false,
			StripTrailingSlash:     true,
			DefaultScheme:          "https",
			TrackingParamAllowlist: nil,
		},
	}
}
