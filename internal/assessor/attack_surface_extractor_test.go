package assessor

import (
	"testing"
)

func TestBuildAttackSurfaceFromHTML_BasicParsing(t *testing.T) {
	html := []byte(`
		<!DOCTYPE html>
		<html>
		<head>
			<title>Test Page</title>
			<script src="https://example.com/app.js"></script>
			<script>var inline = true;</script>
		</head>
		<body>
			<form action="/submit" method="POST">
				<input type="text" name="username" required>
				<input type="password" name="password">
				<textarea name="comment"></textarea>
				<select name="country">
					<option>US</option>
				</select>
				<button type="submit">Submit</button>
			</form>
		</body>
		</html>
	`)

	headers := map[string]string{
		"Content-Type": "text/html; charset=utf-8",
		"Set-Cookie":   "session=abc123; Path=/; Secure; HttpOnly; SameSite=Strict",
	}

	as, err := BuildAttackSurfaceFromHTML(
		"https://example.com/test?param1=value1&param2=value2",
		"snap-123",
		200,
		headers,
		html,
	)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Check basic fields
	if as.URL != "https://example.com/test?param1=value1&param2=value2" {
		t.Errorf("Expected URL to be set, got %s", as.URL)
	}
	if as.SnapshotID != "snap-123" {
		t.Errorf("Expected SnapshotID to be snap-123, got %s", as.SnapshotID)
	}
	if as.StatusCode != 200 {
		t.Errorf("Expected StatusCode to be 200, got %d", as.StatusCode)
	}
	if as.ContentType != "text/html; charset=utf-8" {
		t.Errorf("Expected ContentType to be text/html; charset=utf-8, got %s", as.ContentType)
	}

	// Check query parameters
	if len(as.GetParams) != 2 {
		t.Errorf("Expected 2 GET params, got %d", len(as.GetParams))
	}
	foundParam1 := false
	foundParam2 := false
	for _, p := range as.GetParams {
		if p.Name == "param1" && p.Origin == "query" {
			foundParam1 = true
		}
		if p.Name == "param2" && p.Origin == "query" {
			foundParam2 = true
		}
	}
	if !foundParam1 || !foundParam2 {
		t.Errorf("Expected to find param1 and param2 in GetParams")
	}

	// Check cookies
	if len(as.Cookies) != 1 {
		t.Fatalf("Expected 1 cookie, got %d", len(as.Cookies))
	}
	cookie := as.Cookies[0]
	if cookie.Name != "session" {
		t.Errorf("Expected cookie name to be session, got %s", cookie.Name)
	}
	if !cookie.Secure {
		t.Errorf("Expected cookie to be Secure")
	}
	if !cookie.HttpOnly {
		t.Errorf("Expected cookie to be HttpOnly")
	}
	if cookie.SameSite != "Strict" {
		t.Errorf("Expected SameSite to be Strict, got %s", cookie.SameSite)
	}

	// Check forms
	if len(as.Forms) != 1 {
		t.Fatalf("Expected 1 form, got %d", len(as.Forms))
	}
	form := as.Forms[0]
	if form.Action != "/submit" {
		t.Errorf("Expected form action to be /submit, got %s", form.Action)
	}
	if form.Method != "POST" {
		t.Errorf("Expected form method to be POST, got %s", form.Method)
	}
	if len(form.Inputs) != 4 {
		t.Errorf("Expected 4 inputs, got %d", len(form.Inputs))
	}

	// Check specific input
	foundUsername := false
	for _, input := range form.Inputs {
		if input.Name == "username" && input.Type == "text" && input.Required {
			foundUsername = true
		}
	}
	if !foundUsername {
		t.Errorf("Expected to find required username input")
	}

	// Check scripts
	if len(as.Scripts) != 2 {
		t.Fatalf("Expected 2 scripts, got %d", len(as.Scripts))
	}
	foundExternalScript := false
	foundInlineScript := false
	for _, script := range as.Scripts {
		if script.Src == "https://example.com/app.js" && !script.Inline {
			foundExternalScript = true
		}
		if script.Src == "" && script.Inline {
			foundInlineScript = true
		}
	}
	if !foundExternalScript {
		t.Errorf("Expected to find external script")
	}
	if !foundInlineScript {
		t.Errorf("Expected to find inline script")
	}

	// Check post params from form
	if len(as.PostParams) != 4 {
		t.Errorf("Expected 4 POST params from form, got %d", len(as.PostParams))
	}
}

func TestBuildAttackSurfaceFromHTML_EmptyHTML(t *testing.T) {
	as, err := BuildAttackSurfaceFromHTML(
		"https://example.com",
		"snap-456",
		200,
		nil,
		[]byte(""),
	)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if as == nil {
		t.Fatal("Expected AttackSurface to be non-nil")
	}

	if len(as.Forms) != 0 {
		t.Errorf("Expected 0 forms, got %d", len(as.Forms))
	}
	if len(as.Scripts) != 0 {
		t.Errorf("Expected 0 scripts, got %d", len(as.Scripts))
	}
}

func TestBuildAttackSurfaceFromHTML_NoURL(t *testing.T) {
	as, err := BuildAttackSurfaceFromHTML(
		"",
		"snap-789",
		404,
		nil,
		[]byte("<html></html>"),
	)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if as.URL != "" {
		t.Errorf("Expected empty URL, got %s", as.URL)
	}
	if as.StatusCode != 404 {
		t.Errorf("Expected StatusCode to be 404, got %d", as.StatusCode)
	}
}

func TestParseCookie_CompleteAttributes(t *testing.T) {
	cookie := parseCookie("sessionid=xyz789; Domain=example.com; Path=/app; Secure; HttpOnly; SameSite=Lax")

	if cookie == nil {
		t.Fatal("Expected cookie to be non-nil")
	}

	if cookie.Name != "sessionid" {
		t.Errorf("Expected name to be sessionid, got %s", cookie.Name)
	}
	if cookie.Domain != "example.com" {
		t.Errorf("Expected domain to be example.com, got %s", cookie.Domain)
	}
	if cookie.Path != "/app" {
		t.Errorf("Expected path to be /app, got %s", cookie.Path)
	}
	if !cookie.Secure {
		t.Errorf("Expected cookie to be Secure")
	}
	if !cookie.HttpOnly {
		t.Errorf("Expected cookie to be HttpOnly")
	}
	if cookie.SameSite != "Lax" {
		t.Errorf("Expected SameSite to be Lax, got %s", cookie.SameSite)
	}
}

func TestParseCookie_MinimalCookie(t *testing.T) {
	cookie := parseCookie("token=abc")

	if cookie == nil {
		t.Fatal("Expected cookie to be non-nil")
	}

	if cookie.Name != "token" {
		t.Errorf("Expected name to be token, got %s", cookie.Name)
	}
	if cookie.Secure {
		t.Errorf("Expected cookie to not be Secure")
	}
	if cookie.HttpOnly {
		t.Errorf("Expected cookie to not be HttpOnly")
	}
}

func TestParseCookie_Empty(t *testing.T) {
	cookie := parseCookie("")
	if cookie != nil {
		t.Errorf("Expected nil cookie for empty string")
	}

	cookie = parseCookie("   ")
	if cookie != nil {
		t.Errorf("Expected nil cookie for whitespace string")
	}
}
