package filter

import (
	"net/url"
	"path"
	"strings"
)

// matchExtension checks if the URL path has one of the specified extensions.
// Matching is case-insensitive.
func matchExtension(urlStr string, extensions []string) (bool, string) {
	if len(extensions) == 0 {
		return false, ""
	}

	// Parse the URL to get the path
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return false, ""
	}

	// Get the extension from the path
	urlPath := parsed.Path
	if urlPath == "" {
		return false, ""
	}

	// Get the file extension (lowercase for comparison)
	ext := strings.ToLower(path.Ext(urlPath))
	if ext == "" {
		return false, ""
	}

	// Check against each extension
	for _, skipExt := range extensions {
		normalized := normalizeExtension(skipExt)
		if ext == normalized {
			return true, normalized
		}
	}

	return false, ""
}

// matchPattern checks if the URL matches one of the specified glob patterns.
// Supported wildcards:
// - * matches any sequence of characters (including none) within a path segment
// - ** matches any sequence of path segments (including none)
// - ? matches any single character
func matchPattern(urlStr string, patterns []string) (bool, string) {
	if len(patterns) == 0 {
		return false, ""
	}

	for _, pattern := range patterns {
		if globMatch(pattern, urlStr) {
			return true, pattern
		}
	}

	return false, ""
}

// globMatch performs glob pattern matching on the given string.
// This is a simple implementation that supports *, **, and ? wildcards.
func globMatch(pattern, str string) bool {
	// Handle ** (match any path segments)
	if strings.Contains(pattern, "**") {
		return globMatchDoubleStar(pattern, str)
	}

	// Simple glob matching with * and ?
	return simpleGlobMatch(pattern, str)
}

// globMatchDoubleStar handles patterns with ** (match any path segments).
func globMatchDoubleStar(pattern, str string) bool {
	// Split pattern by ** to get segments
	parts := strings.Split(pattern, "**")

	// Check if str matches the pattern with ** expansion
	if len(parts) == 1 {
		// No ** in pattern, use simple matching
		return simpleGlobMatch(pattern, str)
	}

	// For patterns like "*/media/**" or "**/assets/*"
	// We need to check if the string matches the parts around **
	return fuzzyPathMatch(parts, str)
}

// fuzzyPathMatch matches strings against pattern parts separated by **.
func fuzzyPathMatch(parts []string, str string) bool {
	if len(parts) == 0 {
		return true
	}

	// The first part must match the beginning of str (if not empty)
	if parts[0] != "" {
		if !simpleGlobMatchPrefix(parts[0], str) {
			return false
		}
	}

	// The last part must match the end of str (if not empty)
	lastPart := parts[len(parts)-1]
	if lastPart != "" {
		if !simpleGlobMatchSuffix(lastPart, str) {
			return false
		}
	}

	// Middle parts must appear somewhere in the string (in order)
	currentPos := 0
	for i := 1; i < len(parts)-1; i++ {
		part := parts[i]
		if part == "" {
			continue
		}
		idx := indexOfGlobPattern(str[currentPos:], part)
		if idx < 0 {
			return false
		}
		currentPos += idx + len(part)
	}

	return true
}

// simpleGlobMatch performs glob matching with * and ? only.
func simpleGlobMatch(pattern, str string) bool {
	pIdx, sIdx := 0, 0
	pLen, sLen := len(pattern), len(str)
	starIdx, matchIdx := -1, 0

	for sIdx < sLen {
		if pIdx < pLen && (pattern[pIdx] == '?' || pattern[pIdx] == str[sIdx]) {
			// Match single character
			pIdx++
			sIdx++
		} else if pIdx < pLen && pattern[pIdx] == '*' {
			// Star found - remember position
			starIdx = pIdx
			matchIdx = sIdx
			pIdx++
		} else if starIdx >= 0 {
			// No match, but we had a star - backtrack
			pIdx = starIdx + 1
			matchIdx++
			sIdx = matchIdx
		} else {
			// No match and no star to backtrack to
			return false
		}
	}

	// Check if remaining pattern is all stars
	for pIdx < pLen && pattern[pIdx] == '*' {
		pIdx++
	}

	return pIdx == pLen
}

// simpleGlobMatchPrefix checks if str starts with the pattern (with wildcards).
func simpleGlobMatchPrefix(pattern, str string) bool {
	// For prefix matching, we add a * at the end if not present
	if !strings.HasSuffix(pattern, "*") {
		pattern = pattern + "*"
	}
	return simpleGlobMatch(pattern, str)
}

// simpleGlobMatchSuffix checks if str ends with the pattern (with wildcards).
func simpleGlobMatchSuffix(pattern, str string) bool {
	// For suffix matching, we add a * at the beginning if not present
	if !strings.HasPrefix(pattern, "*") {
		pattern = "*" + pattern
	}
	return simpleGlobMatch(pattern, str)
}

// indexOfGlobPattern finds the first occurrence of pattern in str.
// Returns -1 if not found.
func indexOfGlobPattern(str, pattern string) int {
	for i := 0; i <= len(str)-len(pattern); i++ {
		if simpleGlobMatch(pattern, str[i:i+len(pattern)]) {
			return i
		}
	}
	// For patterns longer than remaining string, check if pattern with wildcards can match
	if simpleGlobMatch(pattern, str) {
		return 0
	}
	return -1
}

// matchStatusCode checks if the given status code is in the skip list.
func matchStatusCode(code int, skipCodes []int) bool {
	for _, skipCode := range skipCodes {
		if code == skipCode {
			return true
		}
	}
	return false
}
