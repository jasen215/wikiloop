//go:build fts5

package distill

import (
	"strings"
	"testing"
)

func TestFindRelatedNotes_NoVecIndex(t *testing.T) {
	// findRelatedNotes always returns "" until FTS-based lookup is implemented.
	result := findRelatedNotes()
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestBuildRelatedContext(t *testing.T) {
	notes := []relatedNote{
		{Path: "wiki/source-notes/foo.md", Title: "Foo", Description: "About foo"},
		{Path: "wiki/source-notes/bar.md", Title: "Bar", Description: "About bar"},
	}
	result := buildRelatedContext(notes)
	if !strings.Contains(result, "wiki/source-notes/foo.md") {
		t.Errorf("context missing foo path: %s", result)
	}
	if !strings.Contains(result, "Foo") {
		t.Errorf("context missing Foo title: %s", result)
	}
	if !strings.Contains(result, "related_to") {
		t.Errorf("context missing related_to instruction: %s", result)
	}
}
