package server_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestHandleCreateProject_DuplicateSlugReturns409(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	first := doJSON(t, s, "POST", "/projects", `{"slug":"dup","name":"First"}`)
	if first.Code != http.StatusCreated {
		t.Fatalf("first create expected 201, got %d: %s", first.Code, first.Body.String())
	}

	second := doJSON(t, s, "POST", "/projects", `{"slug":"dup","name":"Second"}`)
	if second.Code != http.StatusConflict {
		t.Fatalf("duplicate create expected 409, got %d: %s", second.Code, second.Body.String())
	}
	if strings.Contains(second.Body.String(), "UNIQUE constraint") {
		t.Fatalf("409 response leaked raw SQL error: %s", second.Body.String())
	}
}

func TestHandleCreateWebsite_DuplicateSlugReturns409(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	doJSON(t, s, "POST", "/projects", `{"slug":"proj","name":"Proj"}`)

	first := doJSON(t, s, "POST", "/projects/proj/websites", `{"slug":"site","origin":"https://example.com"}`)
	if first.Code != http.StatusCreated {
		t.Fatalf("first website create expected 201, got %d: %s", first.Code, first.Body.String())
	}

	second := doJSON(t, s, "POST", "/projects/proj/websites", `{"slug":"site","origin":"https://other.example.com"}`)
	if second.Code != http.StatusConflict {
		t.Fatalf("duplicate website create expected 409, got %d: %s", second.Code, second.Body.String())
	}
	if strings.Contains(second.Body.String(), "UNIQUE constraint") {
		t.Fatalf("409 response leaked raw SQL error: %s", second.Body.String())
	}
}

func TestHandleDeleteProject_NotFoundReturns404(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	rec := doJSON(t, s, "DELETE", "/projects/nonexistent", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete nonexistent project expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleDeleteWebsite_NotFoundReturns404(t *testing.T) {
	t.Parallel()
	s := newTestServer(t)

	doJSON(t, s, "POST", "/projects", `{"slug":"proj","name":"Proj"}`)

	rec := doJSON(t, s, "DELETE", "/projects/proj/websites/nonexistent", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete nonexistent website expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
