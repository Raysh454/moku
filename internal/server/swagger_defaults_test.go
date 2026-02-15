package server_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSwagger_StartFetchJobRequest_StatusDefaultIsStar(t *testing.T) {
	t.Parallel()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to locate current file path")
	}

	swaggerPath := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "docs", "swagger", "swagger.json"))
	b, err := os.ReadFile(swaggerPath)
	if err != nil {
		t.Fatalf("read swagger.json: %v", err)
	}

	var spec map[string]any
	if err := json.Unmarshal(b, &spec); err != nil {
		t.Fatalf("unmarshal swagger.json: %v", err)
	}

	definitions, ok := spec["definitions"].(map[string]any)
	if !ok {
		t.Fatal("swagger definitions missing or invalid")
	}

	requestDef, ok := definitions["server.StartFetchJobRequest"].(map[string]any)
	if !ok {
		t.Fatal("server.StartFetchJobRequest definition missing")
	}

	properties, ok := requestDef["properties"].(map[string]any)
	if !ok {
		t.Fatal("StartFetchJobRequest properties missing")
	}

	statusProp, ok := properties["status"].(map[string]any)
	if !ok {
		t.Fatal("StartFetchJobRequest.status missing")
	}

	def, ok := statusProp["default"].(string)
	if !ok {
		t.Fatal("StartFetchJobRequest.status default is missing")
	}

	if def != "*" {
		t.Fatalf("expected StartFetchJobRequest.status default to be '*', got %q", def)
	}
}
