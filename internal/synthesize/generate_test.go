//go:build fts5

package synthesize

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractSourceCount(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  int
	}{
		{"present", []byte("---\nsource_count: 3\ntitle: foo\n---\n"), 3},
		{"absent", []byte("---\ntitle: foo\n---\n"), 0},
		{"zero", []byte("---\nsource_count: 0\n---\n"), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSourceCount(tt.input)
			if got != tt.want {
				t.Errorf("extractSourceCount(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestIncrementSourceCount(t *testing.T) {
	input := []byte("---\nsource_count: 2\ntitle: foo\n---\n\ncontent")
	got := incrementSourceCount(input, 1)
	if !strings.Contains(string(got), "source_count: 3") {
		t.Errorf("expected source_count: 3, got: %s", got)
	}
}

func TestIncrementSourceCountAbsent(t *testing.T) {
	input := []byte("---\ntitle: foo\n---\n\ncontent")
	got := incrementSourceCount(input, 1)
	if !strings.Contains(string(got), "source_count: 1") {
		t.Errorf("expected source_count: 1 to be inserted, got: %s", got)
	}
}

func TestAppendOrCreate_CreatesNewPage(t *testing.T) {
	dir := t.TempDir()
	// Create minimal KB structure
	os.MkdirAll(filepath.Join(dir, "wiki", "concepts"), 0o755)
	os.MkdirAll(filepath.Join(dir, "schema", "templates"), 0o755)

	p := PagePlan{
		Type:        "concept",
		Title:       "Test Concept",
		Slug:        "test-concept",
		Description: "A test",
		Sources:     []string{},
	}
	// cfg not configured → should skip silently (no error)
	cfg := Config{}
	err := AppendOrCreate(cfg, dir, p)
	if err != nil {
		t.Errorf("unconfigured LLM should skip silently, got error: %v", err)
	}
}
