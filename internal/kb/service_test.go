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
