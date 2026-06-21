//go:build fts5

package synthesize

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSynthState_NewOrChanged_Empty(t *testing.T) {
	state := &SynthState{Processed: map[string]string{}}
	notes := []SourceNote{
		{Path: "wiki/source-notes/a.md", Title: "A"},
		{Path: "wiki/source-notes/b.md", Title: "B"},
	}
	changed := state.NewOrChanged(notes)
	if len(changed) != 2 {
		t.Errorf("empty state: expected 2 changed, got %d", len(changed))
	}
}

func TestSynthState_NewOrChanged_AllSame(t *testing.T) {
	state := &SynthState{Processed: map[string]string{
		"wiki/source-notes/a.md": hashNote(SourceNote{Path: "wiki/source-notes/a.md", Title: "A"}),
	}}
	notes := []SourceNote{{Path: "wiki/source-notes/a.md", Title: "A"}}
	changed := state.NewOrChanged(notes)
	if len(changed) != 0 {
		t.Errorf("same hash: expected 0 changed, got %d", len(changed))
	}
}

func TestSynthState_SaveLoad(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "index"), 0o755); err != nil {
		t.Fatal(err)
	}
	state := &SynthState{Processed: map[string]string{"a.md": "abc123"}}
	if err := state.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := LoadSynthState(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Processed["a.md"] != "abc123" {
		t.Errorf("loaded hash mismatch: got %q", loaded.Processed["a.md"])
	}
}

func TestFilterViablePlans(t *testing.T) {
	plans := []PagePlan{
		{Type: "concept", Sources: []string{"a", "b"}},      // 2 sources → keep
		{Type: "concept", Sources: []string{"a", "b", "c"}}, // 3 sources → keep
		{Type: "comparison", Sources: []string{"a", "b"}},   // 2 sources → keep
		{Type: "decision", Sources: []string{"a"}},           // 1 source  → keep
		{Type: "decision", Sources: []string{"a", "b"}},      // 2 sources → keep
	}
	result := filterViablePlans(plans)
	if len(result) != 5 {
		t.Errorf("expected 5 viable plans (all have ≥1 source), got %d", len(result))
	}
}

func TestFilterViablePlans_SingleSourceAllowed(t *testing.T) {
	plans := []PagePlan{
		{Type: "concept", Title: "A", Sources: []string{"s1"}},
		{Type: "comparison", Title: "B", Sources: []string{"s1"}},
		{Type: "decision", Title: "C", Sources: []string{"s1"}},
		{Type: "concept", Title: "D", Sources: []string{}}, // empty — must be filtered
	}
	got := filterViablePlans(plans)
	if len(got) != 3 {
		t.Errorf("want 3 plans (single-source allowed), got %d", len(got))
	}
	for _, p := range got {
		if len(p.Sources) == 0 {
			t.Errorf("empty-source plan %q should be filtered", p.Title)
		}
	}
}
