package webclient

import (
	"bytes"
	"fmt"

	"github.com/PuerkitoBio/goquery"
)

// volatileNonceAttr is the CSP nonce attribute: re-randomized on every response,
// so it is pure diff noise and stripped by default.
const volatileNonceAttr = "nonce"

// Normalizer rewrites rendered HTML into a canonical form so that periodic diffs
// reflect real content changes rather than per-request volatility. It removes
// configured elements (e.g. ads, clocks) and strips volatile attributes (the CSP
// nonce by default, plus any caller-configured CSRF/timestamp attributes). It is
// engine-independent and operates on already-captured HTML, so it composes with
// any backend (see NormalizingClient).
type Normalizer struct {
	stripAttributes []string
	removeSelectors []string
}

// NormalizerOption configures a Normalizer.
type NormalizerOption func(*Normalizer)

// WithStripAttributes adds attribute names to remove from every element, on top
// of the default nonce stripping.
func WithStripAttributes(attrs ...string) NormalizerOption {
	return func(n *Normalizer) { n.stripAttributes = append(n.stripAttributes, attrs...) }
}

// WithRemoveSelectors adds CSS selectors whose matching elements are removed
// (e.g. ".ad", "#clock", "[data-testid=banner]").
func WithRemoveSelectors(selectors ...string) NormalizerOption {
	return func(n *Normalizer) { n.removeSelectors = append(n.removeSelectors, selectors...) }
}

// NewNormalizer builds a Normalizer. By default it strips the CSP nonce
// attribute; further attributes and removal selectors are added via options.
func NewNormalizer(opts ...NormalizerOption) *Normalizer {
	n := &Normalizer{stripAttributes: []string{volatileNonceAttr}}
	for _, opt := range opts {
		opt(n)
	}
	return n
}

// Normalize parses html, applies the removal and attribute-stripping rules, and
// re-serializes. The transform is idempotent.
func (n *Normalizer) Normalize(html []byte) ([]byte, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("normalizer: parse html: %w", err)
	}

	for _, selector := range n.removeSelectors {
		doc.Find(selector).Remove()
	}

	if len(n.stripAttributes) > 0 {
		doc.Find("*").Each(func(_ int, sel *goquery.Selection) {
			for _, attr := range n.stripAttributes {
				sel.RemoveAttr(attr)
			}
		})
	}

	out, err := doc.Html()
	if err != nil {
		return nil, fmt.Errorf("normalizer: render html: %w", err)
	}
	return []byte(out), nil
}
