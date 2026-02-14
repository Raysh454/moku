package blobstore_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raysh454/moku/internal/tracker/blobstore"
)

func newTestBlobstore(t *testing.T) *blobstore.Blobstore {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "blobs")
	bs, err := blobstore.New(dir)
	if err != nil {
		t.Fatalf("New blobstore: %v", err)
	}
	return bs
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// ─── Put / Get round-trip ──────────────────────────────────────────────

func TestPut_ReturnsCorrectSHA256(t *testing.T) {
	t.Parallel()
	bs := newTestBlobstore(t)

	data := []byte("hello blobstore")
	id, err := bs.Put(data)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	want := sha256Hex(data)
	if id != want {
		t.Errorf("Put returned %q, want %q", id, want)
	}
}

func TestGet_ReturnsStoredContent(t *testing.T) {
	t.Parallel()
	bs := newTestBlobstore(t)

	data := []byte("<html><body>snapshot</body></html>")
	id, err := bs.Put(data)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := bs.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("Get returned %q, want %q", got, data)
	}
}

func TestGet_BlobNotFound_ReturnsError(t *testing.T) {
	t.Parallel()
	bs := newTestBlobstore(t)

	_, err := bs.Get("0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Fatal("expected error for missing blob, got nil")
	}
	if !strings.Contains(err.Error(), "blob not found") {
		t.Errorf("expected 'blob not found' in error, got %q", err)
	}
}

// ─── Idempotency ───────────────────────────────────────────────────────

func TestPut_SameContentIsIdempotent(t *testing.T) {
	t.Parallel()
	bs := newTestBlobstore(t)

	data := []byte("duplicate content")
	id1, err := bs.Put(data)
	if err != nil {
		t.Fatalf("first Put: %v", err)
	}

	id2, err := bs.Put(data)
	if err != nil {
		t.Fatalf("second Put: %v", err)
	}

	if id1 != id2 {
		t.Errorf("idempotency broken: first=%q second=%q", id1, id2)
	}
}

// ─── Exists ────────────────────────────────────────────────────────────

func TestExists_TrueAfterPut(t *testing.T) {
	t.Parallel()
	bs := newTestBlobstore(t)

	data := []byte("exists-test")
	id, err := bs.Put(data)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	if !bs.Exists(id) {
		t.Error("Exists returned false for stored blob")
	}
}

func TestExists_FalseForMissing(t *testing.T) {
	t.Parallel()
	bs := newTestBlobstore(t)

	if bs.Exists("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") {
		t.Error("Exists returned true for non-existent blob")
	}
}

// ─── Delete ────────────────────────────────────────────────────────────

func TestDelete_RemovesBlob(t *testing.T) {
	t.Parallel()
	bs := newTestBlobstore(t)

	data := []byte("delete-me")
	id, err := bs.Put(data)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	if err := bs.Delete(id); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if bs.Exists(id) {
		t.Error("blob still exists after Delete")
	}
}

func TestDelete_NonExistent_NoError(t *testing.T) {
	t.Parallel()
	bs := newTestBlobstore(t)

	err := bs.Delete("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err != nil {
		t.Errorf("Delete non-existent blob should not error, got %v", err)
	}
}

// ─── GetReader ─────────────────────────────────────────────────────────

func TestGetReader_ReturnsReadableContent(t *testing.T) {
	t.Parallel()
	bs := newTestBlobstore(t)

	data := []byte("reader-test-content")
	id, err := bs.Put(data)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	rc, err := bs.GetReader(id)
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	if !bytes.Equal(got, data) {
		t.Errorf("GetReader returned %q, want %q", got, data)
	}
}

func TestGetReader_MissingBlob_ReturnsError(t *testing.T) {
	t.Parallel()
	bs := newTestBlobstore(t)

	_, err := bs.GetReader("cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")
	if err == nil {
		t.Fatal("expected error for missing blob in GetReader")
	}
}

// ─── PutReader ─────────────────────────────────────────────────────────

func TestPutReader_StoresFromReader(t *testing.T) {
	t.Parallel()
	bs := newTestBlobstore(t)

	data := []byte("streamed-content-for-putreader")
	reader := bytes.NewReader(data)

	id, err := bs.PutReader(reader)
	if err != nil {
		t.Fatalf("PutReader: %v", err)
	}

	want := sha256Hex(data)
	if id != want {
		t.Errorf("PutReader returned %q, want %q", id, want)
	}

	got, err := bs.Get(id)
	if err != nil {
		t.Fatalf("Get after PutReader: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("content mismatch after PutReader round-trip")
	}
}

func TestPutReader_SameContent_Idempotent(t *testing.T) {
	t.Parallel()
	bs := newTestBlobstore(t)

	data := []byte("idempotent-reader")
	id1, _ := bs.PutReader(bytes.NewReader(data))
	id2, _ := bs.PutReader(bytes.NewReader(data))

	if id1 != id2 {
		t.Errorf("PutReader idempotency broken: %q vs %q", id1, id2)
	}
}

// ─── Empty content ─────────────────────────────────────────────────────

func TestPut_EmptyContent(t *testing.T) {
	t.Parallel()
	bs := newTestBlobstore(t)

	id, err := bs.Put([]byte{})
	if err != nil {
		t.Fatalf("Put empty: %v", err)
	}

	want := sha256Hex([]byte{})
	if id != want {
		t.Errorf("empty blob id = %q, want %q", id, want)
	}

	got, err := bs.Get(id)
	if err != nil {
		t.Fatalf("Get empty blob: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty body, got %d bytes", len(got))
	}
}

// ─── Large content ─────────────────────────────────────────────────────

func TestPut_LargeContent(t *testing.T) {
	t.Parallel()
	bs := newTestBlobstore(t)

	data := bytes.Repeat([]byte("A"), 1<<20) // 1 MiB
	id, err := bs.Put(data)
	if err != nil {
		t.Fatalf("Put large: %v", err)
	}

	got, err := bs.Get(id)
	if err != nil {
		t.Fatalf("Get large: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Error("large content round-trip mismatch")
	}
}

// ─── Subdirectory layout ───────────────────────────────────────────────

func TestPut_CreatesSubdirectory(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "blobs")
	bs, err := blobstore.New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	data := []byte("subdir-check")
	id, err := bs.Put(data)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Blob should be at dir/<first2chars>/<full-hash>
	expectedPath := filepath.Join(dir, id[:2], id)
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("expected blob at %s, got stat error: %v", expectedPath, err)
	}
}
