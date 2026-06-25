//go:build fts5

package kb

import (
	"fmt"
	"os"
	"path/filepath"
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
