package enumerator

import (
	"net/url"
	"reflect"
	"testing"
)

func TestFindURLsInText_AcceptsBareURL(t *testing.T) {
	t.Parallel()
	got := findURLsInText("see https://example.com for details")
	want := []string{"https://example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFindURLsInText_AcceptsMultipleURLs(t *testing.T) {
	t.Parallel()
	got := findURLsInText("first https://a.example then http://b.example done")
	want := []string{"https://a.example", "http://b.example"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFindURLsInText_RejectsTrailingComma(t *testing.T) {
	t.Parallel()
	got := findURLsInText("https://example.com, next")
	want := []string{"https://example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFindURLsInText_RejectsTrailingPeriod(t *testing.T) {
	t.Parallel()
	got := findURLsInText("visit https://example.com.")
	want := []string{"https://example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFindURLsInText_RejectsTrailingCloseParen(t *testing.T) {
	t.Parallel()
	got := findURLsInText("(see https://example.com)")
	want := []string{"https://example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFindURLsInText_StripsTrailingSemicolonInJSLike(t *testing.T) {
	t.Parallel()
	got := findURLsInText(`var u = "https://example.com/api";`)
	want := []string{"https://example.com/api"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFindURLsInText_PreservesQueryString(t *testing.T) {
	t.Parallel()
	got := findURLsInText("GET https://example.com/path?q=1&b=2 now")
	want := []string{"https://example.com/path?q=1&b=2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFindURLsInText_RejectsSchemeOnlyNoHost(t *testing.T) {
	t.Parallel()
	got := findURLsInText("bare http:// nothing")
	if len(got) != 0 {
		t.Errorf("expected no URLs for scheme-only, got %v", got)
	}
}

func TestFindURLsInText_EmptyStringReturnsEmpty(t *testing.T) {
	t.Parallel()
	got := findURLsInText("")
	if len(got) != 0 {
		t.Errorf("expected empty result, got %v", got)
	}
}

func TestFindURLsInText_NoURLsReturnsEmpty(t *testing.T) {
	t.Parallel()
	got := findURLsInText("nothing to see here")
	if len(got) != 0 {
		t.Errorf("expected empty result, got %v", got)
	}
}

func TestFindURLsInText_StripsMultipleTrailingPunctuation(t *testing.T) {
	t.Parallel()
	got := findURLsInText(`"https://example.com/path");`)
	want := []string{"https://example.com/path"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFindURLsInText_DoesNotUnderstandTemplateLiterals_KnownLimitation(t *testing.T) {
	t.Parallel()
	// JS template literals like `https://x.com/${id}` require a tokenizer we
	// don't have. We only promise every emitted URL is parseable, not
	// semantically correct.
	got := findURLsInText("`https://example.com/${id}/more`")
	for _, s := range got {
		if _, err := url.Parse(s); err != nil {
			t.Errorf("every emitted URL must parse; %q did not: %v", s, err)
		}
	}
}
