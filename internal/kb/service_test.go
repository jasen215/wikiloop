//go:build fts5

package kb

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKBError(t *testing.T) {
	err := &KBError{Code: 400, Message: "filename is required"}
	if err.Error() != "filename is required" {
		t.Errorf("got %q", err.Error())
	}
	if err.Code != 400 {
		t.Errorf("expected code 400, got %d", err.Code)
	}
	// errors.As unwrap
	var kbe *KBError
	if !errors.As(err, &kbe) {
		t.Error("errors.As should match *KBError")
	}
}

func TestAppendQueryLog(t *testing.T) {
	dir := t.TempDir()
	wikiDir := filepath.Join(dir, "wiki")
	os.MkdirAll(wikiDir, 0o755)

	AppendQueryLog(dir, "kb_search", "test query")

	files, _ := filepath.Glob(filepath.Join(wikiDir, "query_log_*.jsonl"))
	if len(files) != 1 {
		t.Fatalf("expected 1 log file, got %d", len(files))
	}
	content, _ := os.ReadFile(files[0])
	if !strings.Contains(string(content), `"kb_search"`) {
		t.Errorf("log missing tool: %s", content)
	}
	if !strings.Contains(string(content), `"test query"`) {
		t.Errorf("log missing query: %s", content)
	}
}

func TestAppendQueryLogWithExtra(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "wiki"), 0o755)

	AppendQueryLog(dir, "kb_search", "q", "related", `["wiki/a.md"]`)

	files, _ := filepath.Glob(filepath.Join(dir, "wiki", "query_log_*.jsonl"))
	content, _ := os.ReadFile(files[0])
	if !strings.Contains(string(content), `"related"`) {
		t.Errorf("extra field missing: %s", content)
	}
}

func TestKBStatus(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "wiki"), 0o755)
	// OpenDB creates the DB
	db, err := OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	result, err := KBStatus(dir)
	if err != nil {
		t.Fatalf("KBStatus: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Documents < 0 {
		t.Errorf("documents should be >= 0")
	}
	if result.ByLayer == nil {
		t.Error("by_layer should not be nil")
	}
	if result.ByKind == nil {
		t.Error("by_kind should not be nil")
	}
	if result.IndexSize < 0 {
		t.Error("index_size should be >= 0")
	}
	if result.DistillQueue == nil {
		t.Error("distill_queue should not be nil")
	}
}

func TestKBSearch(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "wiki"), 0o755)
	db, err := OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	resp, err := KBSearch(dir, "test", nil, nil, 5, 2)
	if err != nil {
		t.Fatalf("KBSearch: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	// Empty KB → empty results, not error
	if resp.Results == nil {
		t.Error("results should be non-nil slice (may be empty)")
	}
}

func TestKBPageEmptyIDs(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "wiki"), 0o755)
	OpenDB(dir) // init DB

	_, err := KBPage(dir, nil, false)
	var kbe *KBError
	if !errors.As(err, &kbe) || kbe.Code != 400 {
		t.Errorf("expected KBError 400 for empty ids, got %v", err)
	}
}
