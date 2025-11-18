package webclient

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/raysh454/moku/internal/model"
)

// TestBrowserGet verifies that BrowserGet loads a simple local page correctly
func TestChromeDPClient(t *testing.T) {

	// Setup ChromeDPClient
	client, err := NewChromeDPClient(2 * time.Second, nil)
	if err != nil {
		t.Fatalf("Failed to set up ChromeDPClient: %v", err)
	}
	defer client.Close()

	// Setup Request
	modelReq := &model.Request{
		Method: "GET",
		URL: "https://www.all-turtles.com",
		Headers: http.Header{},
		Body: nil,
	}

	modelReq.Headers.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (Test) Chrome/58.0.3029.110 Safari/537.3")

	// Call BrowserGet
	ctx := context.Background()
	resp, err := client.Do(ctx, modelReq)
	if err != nil {
		t.Fatalf("BrowserGet returned error: %v", err)
	}

	// Print response status for debugging
	t.Logf("Response Status: %d", resp.StatusCode)
	// Print headers for debugging
	t.Logf("Response Headers: %v", resp.Headers)
	
	respString := string(resp.Body)
	// Verify HTML contents
	if !strings.Contains(respString, " of our studio companies.") {
		t.Errorf("expected HTML to contain ' of our studio companies.', got: %s", resp.Body)
	}
}

