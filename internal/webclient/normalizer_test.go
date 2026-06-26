package webclient_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/raysh454/moku/internal/webclient"
)

func TestNormalizer_StripsNonceSoNonceOnlyDifferenceNormalizesAway(t *testing.T) {
	t.Parallel()
	n := webclient.NewNormalizer() // default strips the CSP nonce attribute

	a := []byte(`<html><head></head><body><script nonce="abc123">x</script><p>hello</p></body></html>`)
	b := []byte(`<html><head></head><body><script nonce="zzz999">x</script><p>hello</p></body></html>`)

	na, err := n.Normalize(a)
	if err != nil {
		t.Fatalf("Normalize(a): %v", err)
	}
	nb, err := n.Normalize(b)
	if err != nil {
		t.Fatalf("Normalize(b): %v", err)
	}

	if !bytes.Equal(na, nb) {
		t.Errorf("expected a nonce-only difference to normalize away:\n a=%s\n b=%s", na, nb)
	}
	if bytes.Contains(na, []byte("nonce")) {
		t.Errorf("expected nonce attribute stripped, got %s", na)
	}
}

func TestNormalizer_PreservesRealContent(t *testing.T) {
	t.Parallel()
	n := webclient.NewNormalizer()

	out, err := n.Normalize([]byte(`<html><head></head><body><h1>Title</h1><a href="/x">link</a></body></html>`))
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	for _, want := range []string{"Title", `href="/x"`, "link"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("expected normalized output to preserve %q, got %s", want, out)
		}
	}
}

func TestNormalizer_RemovesConfiguredSelectors(t *testing.T) {
	t.Parallel()
	n := webclient.NewNormalizer(webclient.WithRemoveSelectors(".ad", "#clock"))

	out, err := n.Normalize([]byte(`<html><body><div class="ad">buy</div><span id="clock">12:00</span><p>real</p></body></html>`))
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "buy") || strings.Contains(s, "12:00") {
		t.Errorf("expected configured selectors removed, got %s", s)
	}
	if !strings.Contains(s, "real") {
		t.Errorf("expected non-matching content kept, got %s", s)
	}
}

func TestNormalizer_StripsConfiguredAttributes(t *testing.T) {
	t.Parallel()
	n := webclient.NewNormalizer(webclient.WithStripAttributes("data-csrf", "data-timestamp"))

	out, err := n.Normalize([]byte(`<html><body><form data-csrf="tok" data-timestamp="999" action="/go">f</form></body></html>`))
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "data-csrf") || strings.Contains(s, "data-timestamp") {
		t.Errorf("expected configured attributes stripped, got %s", s)
	}
	if !strings.Contains(s, `action="/go"`) {
		t.Errorf("expected non-stripped attribute kept, got %s", s)
	}
}

func TestNormalizer_IsIdempotent(t *testing.T) {
	t.Parallel()
	n := webclient.NewNormalizer(webclient.WithRemoveSelectors(".ad"))
	in := []byte(`<html><head></head><body><div class="ad" nonce="q">x</div><p>keep</p></body></html>`)

	once, err := n.Normalize(in)
	if err != nil {
		t.Fatalf("Normalize once: %v", err)
	}
	twice, err := n.Normalize(once)
	if err != nil {
		t.Fatalf("Normalize twice: %v", err)
	}
	if !bytes.Equal(once, twice) {
		t.Errorf("normalization must be idempotent:\n once=%s\n twice=%s", once, twice)
	}
}
