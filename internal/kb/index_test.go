//go:build fts5

package kb

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestKB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, d := range []string{"raw", "raw/converted", "wiki/source-notes", "schema", "index"} {
		os.MkdirAll(filepath.Join(dir, d), 0755)
	}
	return dir
}

func TestUpsertDocument_New(t *testing.T) {
	dir := setupTestKB(t)
	db, err := OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rawPath := filepath.Join(dir, "raw", "test.md")
	os.WriteFile(rawPath, []byte("---\ntitle: Hello\nkind: source-note\n---\nBody text"), 0644)

	n, err := IndexFiles(db, dir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("indexed %d files, want 1", n)
	}

	var title string
	err = db.QueryRow("SELECT title FROM documents WHERE id = ?", "raw/test.md").Scan(&title)
	if err != nil {
		t.Fatal(err)
	}
	if title != "Hello" {
		t.Errorf("title = %q, want 'Hello'", title)
	}
}

func TestUpsertDocument_SkipUnchanged(t *testing.T) {
	dir := setupTestKB(t)
	db, _ := OpenDB(dir)
	defer db.Close()

	rawPath := filepath.Join(dir, "raw", "test.md")
	os.WriteFile(rawPath, []byte("---\ntitle: Hello\n---\nBody"), 0644)

	n1, _ := IndexFiles(db, dir)
	n2, _ := IndexFiles(db, dir)

	if n1 != 1 {
		t.Errorf("first index: %d, want 1", n1)
	}
	if n2 != 0 {
		t.Errorf("second index: %d, want 0 (unchanged)", n2)
	}
}

// TestIndexFilesFull_ReindexesUnchanged verifies that full reindex rewrites
// documents whose content is unchanged — the behavior kb_reindex(full=true)
// promises. If full silently degraded to incremental, this would return 0 and
// fail, which is exactly the regression we want to catch.
func TestIndexFilesFull_ReindexesUnchanged(t *testing.T) {
	dir := setupTestKB(t)
	db, _ := OpenDB(dir)
	defer db.Close()

	rawPath := filepath.Join(dir, "raw", "test.md")
	os.WriteFile(rawPath, []byte("---\ntitle: Hello\n---\nBody"), 0644)

	IndexFiles(db, dir) // initial index

	// Incremental would skip (hash unchanged); full must still rewrite it.
	n, err := IndexFilesFull(db, dir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("full reindex wrote %d, want 1 (must rewrite unchanged docs)", n)
	}
}

func TestPurgeDeletedDocuments(t *testing.T) {
	dir := setupTestKB(t)
	db, _ := OpenDB(dir)
	defer db.Close()

	rawPath := filepath.Join(dir, "raw", "test.md")
	os.WriteFile(rawPath, []byte("---\ntitle: Hello\n---\nBody"), 0644)
	IndexFiles(db, dir)

	os.Remove(rawPath)
	IndexFiles(db, dir)

	var count int
	db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&count)
	if count != 0 {
		t.Errorf("document count = %d after purge, want 0", count)
	}
}

func TestUpsertDocumentTagsFromFrontmatter(t *testing.T) {
	dir := t.TempDir()
	db, err := OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// 先插入一个文档（document_tags 有外键约束）
	_, err = db.Exec(`INSERT INTO documents
		(id, path, layer, kind, title, description, content, content_hash, updated_at, authority, doc_timestamp)
		VALUES ('raw/a.md','raw/a.md','raw','','title','','content','abc',1,3,0)`)
	if err != nil {
		t.Fatal(err)
	}

	tags := []string{"RAG", "向量数据库", "Qdrant"}
	upsertDocumentTags(db, "raw/a.md", "no entities here", tags)

	rows, err := db.Query("SELECT tag, source FROM document_tags WHERE doc_id='raw/a.md' ORDER BY tag")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	type row struct{ tag, source string }
	var got []row
	for rows.Next() {
		var r row
		rows.Scan(&r.tag, &r.source)
		got = append(got, r)
	}

	if len(got) != 3 {
		t.Fatalf("expected 3 tags, got %d: %v", len(got), got)
	}
	for _, r := range got {
		if r.source != "tag" {
			t.Errorf("expected source=tag, got %q", r.source)
		}
	}
}

func TestUpsertDocumentTagsFromEntities(t *testing.T) {
	dir := t.TempDir()
	db, err := OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`INSERT INTO documents
		(id, path, layer, kind, title, description, content, content_hash, updated_at, authority, doc_timestamp)
		VALUES ('raw/b.md','raw/b.md','raw','','title','','content','def',1,3,0)`)
	if err != nil {
		t.Fatal(err)
	}

	content := "【Karpathy|人物】提出【LLM Wiki|概念】，由【Anthropic|组织】验证"
	upsertDocumentTags(db, "raw/b.md", content, nil)

	rows, err := db.Query("SELECT tag, source FROM document_tags WHERE doc_id='raw/b.md' ORDER BY tag")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	type row struct{ tag, source string }
	var got []row
	for rows.Next() {
		var r row
		rows.Scan(&r.tag, &r.source)
		got = append(got, r)
	}

	// 期望：Karpathy, LLM Wiki, Anthropic（3个实体）
	if len(got) != 3 {
		t.Fatalf("expected 3 entity tags, got %d: %v", len(got), got)
	}
	for _, r := range got {
		if r.source != "entity" {
			t.Errorf("expected source=entity, got %q", r.source)
		}
	}
}

func TestUpsertDocumentTagsFiltersNoise(t *testing.T) {
	dir := t.TempDir()
	db, err := OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`INSERT INTO documents
		(id, path, layer, kind, title, description, content, content_hash, updated_at, authority, doc_timestamp)
		VALUES ('raw/c.md','raw/c.md','raw','','title','','content','ghi',1,3,0)`)
	if err != nil {
		t.Fatal(err)
	}

	// 【技术|库】：name="技术" 是 type 词，应过滤
	// 【A|产品】：name 长度 1，应过滤
	// 【OpenAI|组织】：有效
	content := "【技术|库】做了【A|产品】，基于【OpenAI|组织】"
	upsertDocumentTags(db, "raw/c.md", content, nil)

	var count int
	db.QueryRow("SELECT COUNT(*) FROM document_tags WHERE doc_id='raw/c.md'").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 valid entity (OpenAI), got %d", count)
	}
}
