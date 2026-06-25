package grading

// Outcome is the verdict for a single fetch: did the backend get the real
// content, get challenged, get blocked, or fail to fetch at all.
type Outcome string

const (
	// OutcomeOK means the fetch returned real content with no challenge or block.
	OutcomeOK Outcome = "ok"
	// OutcomeChallenged means an anti-bot interstitial (e.g. a Cloudflare
	// "Just a moment..." page) was served instead of the content.
	OutcomeChallenged Outcome = "challenged"
	// OutcomeBlocked means the request was rejected outright (e.g. 403/429/503).
	OutcomeBlocked Outcome = "blocked"
	// OutcomeError means the fetch itself failed (transport error, nil response).
	OutcomeError Outcome = "error"
)

// severity orders outcomes from least to most concerning so the classifier can
// pick the worst outcome when several signals fire on one response.
func (o Outcome) severity() int {
	switch o {
	case OutcomeOK:
		return 0
	case OutcomeChallenged:
		return 1
	case OutcomeBlocked:
		return 2
	case OutcomeError:
		return 3
	default:
		return 0
	}
}
