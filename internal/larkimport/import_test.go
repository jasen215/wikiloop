package larkimport

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type fakeRunner struct {
	calls int
}

func (f *fakeRunner) Run(_ context.Context, args ...string) ([]byte, error) {
	f.calls++
	if len(args) >= 2 && args[0] == "docs" && args[1] == "+fetch" {
		return []byte(`{"ok":true,"data":{"document":{"content":"<title>Test &amp; Import</title>\n# Results\n<bitable table-id=\"tblA\" token=\"baseA\"></bitable>","document_id":"docA","revision_id":7}}}`), nil
	}
	if len(args) >= 2 && args[0] == "base" && args[1] == "+record-list" {
		offset := argValue(args, "--offset")
		switch offset {
		case "0":
			return []byte(`{"ok":true,"data":{"data":[["Title 1","Alice"],["Title 2","Bob"]],"fields":["Title","Name"],"has_more":true}}`), nil
		case "2":
			return []byte(`{"ok":true,"data":{"data":[["Title 3","Carol"]],"fields":["Title","Name"],"has_more":false}}`), nil
		}
	}
	return nil, fmt.Errorf("unexpected args: %v", args)
}

func TestImportExpandsBitableIntoSearchableDataset(t *testing.T) {
	kbRoot := t.TempDir()
	runner := &fakeRunner{}

	result, err := Import(context.Background(), kbRoot, "https://example.larkoffice.com/wiki/abc", "", runner)
	if err != nil {
		t.Fatal(err)
	}
	if result.DocumentPath != "raw/lark/test-import/document.md" {
		t.Fatalf("document path = %q", result.DocumentPath)
	}
	if len(result.TableRows) != 1 || result.TableRows[0] != 3 {
		t.Fatalf("table rows = %v, want [3]", result.TableRows)
	}

	document, err := os.ReadFile(filepath.Join(kbRoot, filepath.FromSlash(result.DocumentPath)))
	if err != nil {
		t.Fatal(err)
	}
	docText := string(document)
	for _, want := range []string{
		`source_url: "https://example.larkoffice.com/wiki/abc"`,
		"Rows: 3",
		"table-01-tblA.txt",
	} {
		if !strings.Contains(docText, want) {
			t.Errorf("document missing %q:\n%s", want, docText)
		}
	}
	if strings.Contains(docText, "<bitable") {
		t.Fatal("document still contains an unexpanded bitable tag")
	}

	dataset, err := os.ReadFile(filepath.Join(kbRoot, filepath.FromSlash(result.TablePaths[0])))
	if err != nil {
		t.Fatal(err)
	}
	dataText := string(dataset)
	if !strings.Contains(dataText, "Title\tName\n") || !strings.Contains(dataText, "Title 3\tCarol") {
		t.Fatalf("dataset content is incomplete:\n%s", dataText)
	}
	if runner.calls != 3 {
		t.Fatalf("runner calls = %d, want 3", runner.calls)
	}
}

func TestParseTableRefsSupportsAttributeOrder(t *testing.T) {
	content := `<bitable table-id="one" token="base1"></bitable>
<bitable token="base2" table-id="two"></bitable>`
	refs := parseTableRefs(content)
	if len(refs) != 2 {
		t.Fatalf("refs = %d, want 2", len(refs))
	}
	if refs[0].TableID != "one" || refs[0].BaseToken != "base1" {
		t.Fatalf("first ref = %+v", refs[0])
	}
	if refs[1].TableID != "two" || refs[1].BaseToken != "base2" {
		t.Fatalf("second ref = %+v", refs[1])
	}
}

func argValue(args []string, name string) string {
	for i := range args {
		if args[i] == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return strconv.Itoa(-1)
}
