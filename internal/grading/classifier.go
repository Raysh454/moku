package grading

import "github.com/raysh454/moku/internal/webclient"

// ClassifyRule binds a Signal to the Outcome it implies when it fires.
type ClassifyRule struct {
	Signal  Signal
	Outcome Outcome
}

// ClassifyResult is the verdict for one response plus the signals that fired.
type ClassifyResult struct {
	Outcome   Outcome        `json:"outcome"`
	Triggered []SignalResult `json:"triggered,omitempty"`
}

// Classifier maps a fetched response to an Outcome by evaluating an ordered set
// of rules. When several rules fire on one response the most severe outcome
// wins, but every fired signal is reported as evidence. A nil response is an
// error (the fetch never produced a response to classify).
type Classifier struct {
	rules []ClassifyRule
}

// NewClassifier builds a Classifier from the given rules. With no rules every
// non-nil response classifies as OutcomeOK.
func NewClassifier(rules ...ClassifyRule) *Classifier {
	return &Classifier{rules: rules}
}

// Classify evaluates every rule against resp and returns the worst implied
// outcome together with the evidence for each rule that fired.
func (c *Classifier) Classify(resp *webclient.Response) ClassifyResult {
	if resp == nil {
		return ClassifyResult{Outcome: OutcomeError}
	}

	result := ClassifyResult{Outcome: OutcomeOK}
	for _, rule := range c.rules {
		sr := rule.Signal.Evaluate(resp)
		if !sr.Detected {
			continue
		}
		result.Triggered = append(result.Triggered, sr)
		if rule.Outcome.severity() > result.Outcome.severity() {
			result.Outcome = rule.Outcome
		}
	}
	return result
}
