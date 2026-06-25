package webclient

import (
	"context"
	"errors"
	"fmt"

	"github.com/raysh454/moku/internal/logging"
)

// ShouldEscalate reports whether a tier's response is unsatisfactory — typically
// a bot challenge or block — so the request should be retried on the next, more
// capable tier. It is the Strategy that keeps the EscalatingClient ignorant of
// how a challenge is detected, so the composition root can wire in any detector
// (e.g. the grading package's Cloudflare classifier) without this type
// depending on it.
type ShouldEscalate func(resp *Response) bool

// EscalatingClient is a WebClient that fronts an ordered chain of backend tiers,
// cheapest and least capable first. It returns the first tier whose response is
// satisfactory; on a transport error or a response the ShouldEscalate predicate
// rejects, it falls through to the next tier. This realises the tiered fetch
// strategy (e.g. tls -> chromedp -> remote) behind the single WebClient
// interface, so callers escalate only the minority of targets that need it.
type EscalatingClient struct {
	tiers          []WebClient
	shouldEscalate ShouldEscalate
	logger         logging.Logger
}

// NewEscalatingClient wires an escalation chain. tiers must be ordered from
// cheapest/weakest to most capable and contain at least one backend;
// shouldEscalate must be non-nil.
func NewEscalatingClient(tiers []WebClient, shouldEscalate ShouldEscalate, logger logging.Logger) (WebClient, error) {
	if len(tiers) == 0 {
		return nil, fmt.Errorf("escalating client requires at least one tier")
	}
	if shouldEscalate == nil {
		return nil, fmt.Errorf("escalating client requires a shouldEscalate predicate")
	}
	return &EscalatingClient{
		tiers:          tiers,
		shouldEscalate: shouldEscalate,
		logger:         logger.With(logging.Field{Key: "backend", Value: "escalating"}),
	}, nil
}

// Do tries each tier in order and returns the first satisfactory response. If
// every tier escalates, the last response obtained is returned (so the caller
// still sees a real challenge/block result); if no tier produced any response,
// the last transport error is returned.
func (e *EscalatingClient) Do(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}

	var lastResp *Response
	var lastErr error

	for index, tier := range e.tiers {
		resp, err := tier.Do(ctx, req)
		if err != nil {
			lastErr = err
			e.logger.Debug("tier failed; escalating",
				logging.Field{Key: "tier", Value: index},
				logging.Field{Key: "error", Value: err.Error()})
			continue
		}

		lastResp = resp
		lastErr = nil
		if !e.shouldEscalate(resp) {
			return resp, nil
		}
		e.logger.Debug("tier response unsatisfactory; escalating",
			logging.Field{Key: "tier", Value: index},
			logging.Field{Key: "status", Value: resp.StatusCode})
	}

	if lastResp != nil {
		return lastResp, nil
	}
	return nil, lastErr
}

// Get is a convenience method for simple GET requests.
func (e *EscalatingClient) Get(ctx context.Context, url string) (*Response, error) {
	return e.Do(ctx, &Request{Method: "GET", URL: url})
}

// Close closes every tier, joining any errors so one failure does not hide
// another.
func (e *EscalatingClient) Close() error {
	var errs []error
	for _, tier := range e.tiers {
		if err := tier.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
