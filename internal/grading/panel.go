package grading

// Cloudflare and generic anti-bot fingerprints observed in a served body when a
// request is challenged rather than answered. Kept as named constants so the
// detection vocabulary is reviewable in one place.
var challengeBodyMarkers = []string{
	"just a moment",
	"checking your browser",
	"verifying you are human",
	"enable javascript and cookies to continue",
	"cf-browser-verification",
	"cf_chl_opt",
	"challenge-platform",
	"__cf_chl",
	"turnstile",
}

// blockedStatusCodes are the status codes anti-bot edges return on an outright
// rejection.
var blockedStatusCodes = []int{403, 429, 503}

// DefaultClassifier builds the standard Cloudflare-aware classifier: a blocked
// status or a block-tagged cf-mitigated header is OutcomeBlocked, an
// interstitial body marker or a challenge-tagged cf-mitigated header is
// OutcomeChallenged, and anything else is OutcomeOK.
func DefaultClassifier() *Classifier {
	return NewClassifier(
		ClassifyRule{
			Signal:  NewStatusSignal("blocked-status", blockedStatusCodes...),
			Outcome: OutcomeBlocked,
		},
		ClassifyRule{
			Signal:  NewHeaderMarkerSignal("cf-mitigated-block", "Cf-Mitigated", "block"),
			Outcome: OutcomeBlocked,
		},
		ClassifyRule{
			Signal:  NewBodyMarkerSignal("challenge-body", challengeBodyMarkers...),
			Outcome: OutcomeChallenged,
		},
		ClassifyRule{
			Signal:  NewHeaderMarkerSignal("cf-mitigated-challenge", "Cf-Mitigated", "challenge"),
			Outcome: OutcomeChallenged,
		},
	)
}

// DefaultPanel is a starting panel of public bot-detection and challenge pages
// plus a JS-free control. It is a baseline for comparing backends; callers
// should add their own in-scope targets (only test targets you are authorized
// to access) since real Cloudflare behavior is target-specific.
func DefaultPanel() []Probe {
	return []Probe{
		{Name: "sannysoft", URL: "https://bot.sannysoft.com/"},
		{Name: "creepjs", URL: "https://abrahamjuliot.github.io/creepjs/"},
		{Name: "incolumitas", URL: "https://bot.incolumitas.com/"},
		{Name: "browserleaks-canvas", URL: "https://browserleaks.com/canvas"},
		{Name: "scrapfly-automation", URL: "https://scrapfly.io/web-scraping-tools/automation-detector/"},
		{Name: "cloudflare-challenge-demo", URL: "https://www.scrapingcourse.com/cloudflare-challenge"},
		{Name: "nowsecure-cf", URL: "https://nowsecure.nl/"},
		{Name: "httpbin-headers", URL: "https://httpbin.org/headers"},
	}
}
