package webclient

import (
	"strings"
	"testing"
)

// TestBrowserGet verifies that BrowserGet loads a simple local page correctly
func TestBrowserGet(t *testing.T) {
	// Call BrowserGet
	html, err := BrowserGet("https://www.all-turtles.com")
	if err != nil {
		t.Fatalf("BrowserGet returned error: %v", err)
	}

	// Verify HTML contents
	if !strings.Contains(html, " of our studio companies.") {
		t.Errorf("expected HTML to contain ' of our studio companies.', got: %s", html)
	}


	t.Logf("Fetched HTML: %s", html)
}

