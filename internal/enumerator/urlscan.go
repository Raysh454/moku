package enumerator

import (
	"net/url"
	"regexp"
	"strings"
)

// urlCandidateRegex matches http(s) URLs in free-form text. It rejects the
// obvious structural delimiters (whitespace, quotes, angle brackets,
// backticks) so the match boundary is close to the URL's own boundary.
var urlCandidateRegex = regexp.MustCompile(`https?://[^\s"'<>` + "`" + `]+`)

// urlTrailingTrimSet is the set of punctuation characters we strip from the
// right end of a candidate before validation. These commonly appear after a
// URL in prose or source code and are almost never part of the URL itself.
const urlTrailingTrimSet = ".,;:)]}\"'\\`>"

// findURLsInText extracts parseable http(s) URLs from an arbitrary text blob.
// It is deliberately pragmatic: it does not understand JS template literals,
// HTML entities, or URL-encoded nesting. Candidates that fail url.Parse or
// lack a Host are dropped.
func findURLsInText(text string) []string {
	if text == "" {
		return nil
	}
	matches := urlCandidateRegex.FindAllString(text, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	for _, candidate := range matches {
		trimmed := strings.TrimRight(candidate, urlTrailingTrimSet)
		if trimmed == "" {
			continue
		}
		u, err := url.Parse(trimmed)
		if err != nil || u.Host == "" {
			continue
		}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
