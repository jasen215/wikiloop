//go:build fts5

package kb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// KBError is a typed error returned by all KB service functions.
// Code follows HTTP semantics: 400 = client error, 500 = server error.
type KBError struct {
	Code    int
	Message string
}

func (e *KBError) Error() string { return e.Message }

// AppendQueryLog writes a JSONL entry to wiki/query_log_YYYY-MM-DD.jsonl.
// extra is alternating key/value pairs appended to the JSON object.
// Non-fatal: errors are silently ignored.
func AppendQueryLog(kbRoot, tool, query string, extra ...string) {
	now := time.Now().UTC()
	date := now.Format("2006-01-02")
	logPath := filepath.Join(kbRoot, "wiki", "query_log_"+date+".jsonl")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	ts := now.Format("2006-01-02T15:04:05Z")
	if len(extra) >= 2 {
		// Build extra JSON fields: "key": <value>
		var extraJSON string
		for i := 0; i+1 < len(extra); i += 2 {
			extraJSON += fmt.Sprintf(",%q:%s", extra[i], extra[i+1])
		}
		fmt.Fprintf(f, "{\"ts\":%q,\"tool\":%q,\"query\":%q%s}\n", ts, tool, query, extraJSON)
	} else {
		fmt.Fprintf(f, "{\"ts\":%q,\"tool\":%q,\"query\":%q}\n", ts, tool, query)
	}
}

// StatusResult is the unified status response for both MCP and WebUI.
type StatusResult struct {
	Documents    int            `json:"documents"`
	ByLayer      map[string]int `json:"by_layer"`
	ByKind       map[string]int `json:"by_kind"`
	IndexPath    string         `json:"index_path"`
	IndexSize    int64          `json:"index_size"`
	DistillQueue map[string]int `json:"distill_queue"`
}

// SearchResponse wraps SearchLayered results.
type SearchResponse struct {
	Results   []SearchResult `json:"results"`
	Conflicts []Conflict     `json:"conflicts"`
}

// distillQueueStats returns task counts by status from distill_queue.
// Inlined here to avoid an import cycle between kb ↔ distill.
func distillQueueStats(db *sql.DB) (map[string]int, error) {
	rows, err := db.Query(`SELECT status, COUNT(*) FROM distill_queue GROUP BY status`)
	if err != nil {
		return map[string]int{"pending": 0, "processing": 0, "done": 0, "failed": 0}, nil
	}
	defer rows.Close()
	counts := map[string]int{"pending": 0, "processing": 0, "done": 0, "failed": 0}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return counts, err
		}
		counts[status] = n
	}
	return counts, rows.Err()
}

// KBStatus returns unified KB status (documents, layers, kinds, queue, index size).
func KBStatus(kbRoot string) (*StatusResult, error) {
	AppendQueryLog(kbRoot, "kb_status", "")
	dbPath := filepath.Join(kbRoot, "index", "kb.sqlite")
	db, err := OpenDB(kbRoot)
	if err != nil {
		return nil, &KBError{Code: 500, Message: err.Error()}
	}
	defer db.Close()

	byLayer, total, _ := LayerCounts(db)
	byKind, _ := KindCounts(db)
	queueStats, _ := distillQueueStats(db)

	var indexSize int64
	if fi, err := os.Stat(dbPath); err == nil {
		indexSize = fi.Size()
	}

	return &StatusResult{
		Documents:    total,
		ByLayer:      byLayer,
		ByKind:       byKind,
		IndexPath:    dbPath,
		IndexSize:    indexSize,
		DistillQueue: queueStats,
	}, nil
}

// KBSearch runs layered FTS search and returns results with related docs.
func KBSearch(kbRoot, query string, layer, kind *string, sourceLimit, synthLimit int) (*SearchResponse, error) {
	db, err := OpenDB(kbRoot)
	if err != nil {
		return nil, &KBError{Code: 500, Message: err.Error()}
	}
	defer db.Close()

	results, conflicts, err := SearchLayered(db, kbRoot, query, layer, kind, sourceLimit, synthLimit)
	if err != nil {
		return nil, &KBError{Code: 500, Message: err.Error()}
	}

	// Log with related IDs.
	var relatedIDs []string
	seen := make(map[string]bool)
	for _, r := range results {
		for _, rel := range r.Related {
			if !seen[rel.ID] {
				seen[rel.ID] = true
				relatedIDs = append(relatedIDs, rel.ID)
			}
		}
	}
	if len(relatedIDs) > 0 {
		b, _ := json.Marshal(relatedIDs)
		AppendQueryLog(kbRoot, "kb_search", query, "related", string(b))
	} else {
		AppendQueryLog(kbRoot, "kb_search", query)
	}

	if results == nil {
		results = []SearchResult{}
	}
	if conflicts == nil {
		conflicts = []Conflict{}
	}
	return &SearchResponse{Results: results, Conflicts: conflicts}, nil
}

// KBPage fetches full content for one or more wiki pages by ID (max 5).
func KBPage(kbRoot string, ids []string, full bool) ([]PageResult, error) {
	if len(ids) == 0 {
		return nil, &KBError{Code: 400, Message: "ids is required"}
	}
	if len(ids) > 5 {
		ids = ids[:5]
	}
	AppendQueryLog(kbRoot, "kb_page", strings.Join(ids, ","))
	db, err := OpenDB(kbRoot)
	if err != nil {
		return nil, &KBError{Code: 500, Message: err.Error()}
	}
	defer db.Close()

	pages, err := FetchPages(db, kbRoot, ids, full)
	if err != nil {
		return nil, &KBError{Code: 500, Message: err.Error()}
	}
	return pages, nil
}
