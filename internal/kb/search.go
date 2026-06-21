//go:build fts5

package kb

import (
	"database/sql"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// SearchResult holds a single result from FTS or hybrid search.
type SearchResult struct {
	ID           string  `json:"id"`
	DocID        string  `json:"doc_id,omitempty"` // parent doc ID when result is a chunk
	Path         string  `json:"path"`
	Layer        string  `json:"layer"`
	Kind         string  `json:"kind"`
	Title        string  `json:"title"`
	Description  string  `json:"description,omitempty"`
	Snippet      string  `json:"snippet,omitempty"`
	WikiPriority float64 `json:"wiki_priority"`
	FTSRank      float64 `json:"fts_rank,omitempty"`
	FTSScore     float64 `json:"fts_score,omitempty"`
	VecScore     float64 `json:"vec_score,omitempty"`
	HybridScore  float64 `json:"hybrid_score,omitempty"`
	GraphBoost   float64 `json:"graph_boost,omitempty"`
}

// minTrigramLen is the minimum token length for FTS5 trigram tokenizer.
const minTrigramLen = 3

// FTSSearch performs a full-text search over the documents table.
// query supports comma-separated keywords: "Go, Python" → per-keyword search.
// layer optionally filters results to a specific layer (raw/wiki/schema).
// kind optionally filters by page kind (source-note, concept, comparison, decision).
// Returns nil (not error) for empty queries.
//
// FTS5 trigram tokenizer requires tokens ≥ 3 chars. Keywords shorter than
// minTrigramLen fall back to SQL LIKE matching on title and content.
// Results are deduplicated by ID before being returned.
func FTSSearch(db *sql.DB, query string, layer *string, limit int) ([]SearchResult, error) {
	return FTSSearchFiltered(db, query, layer, nil, limit)
}

// FTSSearchFiltered is like FTSSearch but also accepts an optional kind filter.
func FTSSearchFiltered(db *sql.DB, query string, layer, kind *string, limit int) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	keywords := splitKeywords(query)

	// Separate long keywords (FTS5) from short keywords (LIKE fallback).
	var ftsKws, likeKws []string
	for _, kw := range keywords {
		if utf8.RuneCountInString(kw) >= minTrigramLen {
			ftsKws = append(ftsKws, kw)
		} else {
			likeKws = append(likeKws, kw)
		}
	}

	// Expand limit for multi-keyword OR queries.
	effectiveLimit := limit * len(keywords)

	seen := make(map[string]bool)
	var results []SearchResult

	// FTS5 two-phase strategy: AND first (precision), OR fallback (recall).
	if len(ftsKws) > 0 {
		// Phase 1: AND query — all keywords must appear in the document.
		// This eliminates cross-context false positives (e.g. "recall" in
		// Agent memory articles when searching for RAG recall rate).
		if len(ftsKws) >= 2 {
			andQuery := buildFTSAndQuery(ftsKws)
			res, err := ftsQuery_(db, andQuery, layer, kind, effectiveLimit)
			if err != nil {
				return nil, err
			}
			for _, r := range res {
				if !seen[r.ID] {
					seen[r.ID] = true
					results = append(results, r)
				}
			}
		}

		// Phase 2: OR query — any keyword matches (fallback for coverage).
		// Only adds documents not already found by AND query.
		orQuery := buildFTSQuery(ftsKws)
		res, err := ftsQuery_(db, orQuery, layer, kind, effectiveLimit)
		if err != nil {
			return nil, err
		}
		for _, r := range res {
			if !seen[r.ID] {
				seen[r.ID] = true
				results = append(results, r)
			}
		}
	}

	// LIKE fallback for short keywords (e.g., "Go", "C").
	for _, kw := range likeKws {
		res, err := likeSearch(db, kw, layer, kind, effectiveLimit)
		if err != nil {
			return nil, err
		}
		for _, r := range res {
			if !seen[r.ID] {
				seen[r.ID] = true
				results = append(results, r)
			}
		}
	}

	// Sort: wiki first, then by fts_rank (lower is better in FTS5).
	// AND-matched documents naturally rank higher (matched more keywords).
	sortResults(results)

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// ftsQuery_ executes a FTS5 MATCH query and returns SearchResult rows.
func ftsQuery_(db *sql.DB, ftsQuery string, layer, kind *string, limit int) ([]SearchResult, error) {
	var args []interface{}
	args = append(args, ftsQuery)

	layerFilter := ""
	if layer != nil {
		layerFilter = "AND d.layer = ?"
		args = append(args, *layer)
	}
	kindFilter := ""
	if kind != nil {
		kindFilter = "AND d.kind = ?"
		args = append(args, *kind)
	}
	args = append(args, limit)

	sqlStr := `
SELECT
    d.id,
    d.path,
    d.layer,
    COALESCE(d.kind, '') AS kind,
    COALESCE(d.title, '') AS title,
    COALESCE(d.description, '') AS description,
    snippet(document_fts, 2, '[', ']', '...', 10) AS snippet,
    CASE d.layer WHEN 'wiki' THEN 1.0 ELSE 0.0 END AS wiki_priority,
    rank AS fts_rank
FROM document_fts
JOIN documents d ON d.id = document_fts.id
WHERE document_fts MATCH ?
` + layerFilter + kindFilter + `
ORDER BY wiki_priority DESC, rank
LIMIT ?`

	return scanResults(db, sqlStr, args...)
}

// likeSearch performs LIKE-based fallback search for short keywords (< 3 chars).
func likeSearch(db *sql.DB, kw string, layer, kind *string, limit int) ([]SearchResult, error) {
	pattern := "%" + kw + "%"
	var args []interface{}
	args = append(args, pattern, pattern)

	layerFilter := ""
	if layer != nil {
		layerFilter = "AND layer = ?"
		args = append(args, *layer)
	}
	kindFilter := ""
	if kind != nil {
		kindFilter = "AND kind = ?"
		args = append(args, *kind)
	}
	args = append(args, limit)

	sqlStr := `
SELECT
    id,
    path,
    layer,
    COALESCE(kind, '') AS kind,
    COALESCE(title, '') AS title,
    COALESCE(description, '') AS description,
    '' AS snippet,
    CASE layer WHEN 'wiki' THEN 1.0 ELSE 0.0 END AS wiki_priority,
    0.0 AS fts_rank
FROM documents
WHERE (title LIKE ? OR content LIKE ?)
` + layerFilter + kindFilter + `
ORDER BY wiki_priority DESC
LIMIT ?`

	return scanResults(db, sqlStr, args...)
}

// scanResults executes a query and scans rows into SearchResult slice.
func scanResults(db *sql.DB, sqlStr string, args ...interface{}) ([]SearchResult, error) {
	rows, err := db.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(
			&r.ID, &r.Path, &r.Layer, &r.Kind, &r.Title,
			&r.Description, &r.Snippet, &r.WikiPriority, &r.FTSRank,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// sortResults sorts results: wiki layer first, then by FTS rank (ascending, lower = better).
func sortResults(results []SearchResult) {
	for i := 1; i < len(results); i++ {
		for j := i; j > 0; j-- {
			a, b := results[j-1], results[j]
			// Higher wiki_priority first; for equal priority, lower fts_rank first.
			if a.WikiPriority < b.WikiPriority ||
				(a.WikiPriority == b.WikiPriority && a.FTSRank > b.FTSRank) {
				results[j-1], results[j] = b, a
			} else {
				break
			}
		}
	}
}

// splitRe splits on commas (ASCII/fullwidth) or whitespace (one or more).
var splitRe = regexp.MustCompile(`[,，]+|\s+`)

// splitKeywords splits a query on commas and whitespace into trimmed keywords,
// filtering out empty entries.
func splitKeywords(query string) []string {
	parts := splitRe.Split(query, -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// buildFTSQuery converts keywords into a FTS5 OR expression with quoted phrases.
// Single keyword: `"Go"` → `"Go"`.
// Multiple: `"Go" OR "Python"`.
func buildFTSQuery(keywords []string) string {
	quoted := make([]string, len(keywords))
	for i, kw := range keywords {
		// Escape any existing double-quotes inside keyword.
		kw = strings.ReplaceAll(kw, `"`, `""`)
		quoted[i] = `"` + kw + `"`
	}
	return strings.Join(quoted, " OR ")
}

// buildFTSAndQuery converts keywords into a FTS5 AND expression.
// All keywords must appear in the document.
// Example: ["RAG", "召回率"] → `"RAG" AND "召回率"`
func buildFTSAndQuery(keywords []string) string {
	quoted := make([]string, len(keywords))
	for i, kw := range keywords {
		kw = strings.ReplaceAll(kw, `"`, `""`)
		quoted[i] = `"` + kw + `"`
	}
	return strings.Join(quoted, " AND ")
}

// rrfK is the constant in Reciprocal Rank Fusion: score = 1/(k + rank).
// k=60 is the standard value from the original RRF paper (Cormack et al., 2009).
// Higher k reduces the impact of top-rank differences; lower k amplifies them.
const rrfK = 60.0

// HybridRank merges FTS and vector results using Reciprocal Rank Fusion (RRF).
//
// RRF score for each source: 1 / (rrfK + rank_in_that_source)
// Final score = rrfFTS + rrfVec + wikiBoost + graphBoost
//
// Wiki-layer docs get a small additive boost so they surface before raw-layer
// docs of equal relevance. Graph boost rewards docs that are linked from other
// highly-ranked results.
//
// boostMap maps doc ID → graph boost value (may be nil).
// Results are returned sorted by hybrid score descending.
func HybridRank(ftsResults, vecResults []SearchResult, boostMap map[string]float64, recencyMap map[string]int64) []SearchResult {
	type entry struct {
		r        SearchResult
		ftsRank  int // 1-based rank in FTS results (0 = not present)
		vecRank  int // 1-based rank in vec results (0 = not present)
	}
	merged := make(map[string]*entry)

	for i, r := range ftsResults {
		merged[r.ID] = &entry{r: r, ftsRank: i + 1}
	}
	for i, r := range vecResults {
		if e, ok := merged[r.ID]; ok {
			e.vecRank = i + 1
			e.r.VecScore = r.VecScore
		} else {
			merged[r.ID] = &entry{r: r, vecRank: i + 1}
		}
	}

	results := make([]SearchResult, 0, len(merged))
	for _, e := range merged {
		r := e.r

		var rrfScore float64
		if e.ftsRank > 0 {
			rrfScore += 1.0 / (rrfK + float64(e.ftsRank))
		}
		if e.vecRank > 0 {
			rrfScore += 1.0 / (rrfK + float64(e.vecRank))
		}

		// Additive boosts (small relative to RRF scores to preserve rank order).
		if r.Layer == "wiki" {
			rrfScore += 1.0 / (rrfK + 1) * 0.5 // half a top-1 wiki contribution
			if recencyMap != nil {
				if ts, ok := recencyMap[r.ID]; ok {
					rrfScore += recencyBoost(ts)
				}
			}
		}
		if boostMap != nil {
			rrfScore += boostMap[r.ID] * 0.01
		}

		r.HybridScore = rrfScore
		r.GraphBoost = 0
		if boostMap != nil {
			r.GraphBoost = boostMap[r.ID]
		}
		results = append(results, r)
	}

	// Sort by hybrid score descending.
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].HybridScore > results[j-1].HybridScore; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}

	return results
}

// abs64 returns the absolute value of v.
func abs64(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// cosineSim returns the cosine similarity between two vectors.
// Returns 0 if either vector has zero magnitude.
func cosineSim(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, magA, magB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		magA += float64(a[i]) * float64(a[i])
		magB += float64(b[i]) * float64(b[i])
	}
	if magA == 0 || magB == 0 {
		return 0
	}
	return dot / (math.Sqrt(magA) * math.Sqrt(magB))
}

// biEncoderRerank re-scores results using the embedder.
// Each result is scored by cosine similarity between the query embedding
// and the embedding of "title description". Results are sorted descending
// and truncated to limit. Falls back to original order if embedder is nil
// or encoding fails.
func biEncoderRerank(query string, results []SearchResult, embedder Embedder, limit int) []SearchResult {
	if embedder == nil || len(results) == 0 {
		if limit < len(results) {
			return results[:limit]
		}
		return results
	}

	queryVec, err := embedder.Encode(query)
	if err != nil {
		if limit < len(results) {
			return results[:limit]
		}
		return results
	}

	type scored struct {
		r     SearchResult
		score float64
	}
	scored_ := make([]scored, 0, len(results))
	for _, r := range results {
		text := r.Title
		if r.Description != "" {
			text += " " + r.Description
		}
		vec, err := embedder.Encode(text)
		if err != nil {
			scored_ = append(scored_, scored{r, 0})
			continue
		}
		scored_ = append(scored_, scored{r, cosineSim(queryVec, vec)})
	}

	// Insertion sort descending by score.
	for i := 1; i < len(scored_); i++ {
		for j := i; j > 0 && scored_[j].score > scored_[j-1].score; j-- {
			scored_[j], scored_[j-1] = scored_[j-1], scored_[j]
		}
	}

	out := make([]SearchResult, 0, limit)
	for i, s := range scored_ {
		if i >= limit {
			break
		}
		out = append(out, s.r)
	}
	return out
}

// recencyBoost returns a small additive score for recently-updated wiki documents.
// Documents updated within 30 days receive up to half a top-1 RRF contribution,
// decaying linearly to zero at 30 days. Raw/schema docs are not boosted.
func recencyBoost(updatedAt int64) float64 {
	const maxAgeDays = 30.0
	ageDays := float64(time.Now().Unix()-updatedAt) / 86400.0
	if ageDays < 0 {
		ageDays = 0
	}
	if ageDays >= maxAgeDays {
		return 0
	}
	// Linear decay: 1.0 at age=0, 0.0 at age=maxAgeDays
	decay := 1.0 - ageDays/maxAgeDays
	// Max boost = half a top-1 RRF contribution * 0.1 (small relative to relevance)
	return (1.0 / (rrfK + 1)) * 0.5 * 0.1 * decay
}

// fetchRecencyMap queries updated_at for a set of doc IDs.
func fetchRecencyMap(db *sql.DB, ids []string) map[string]int64 {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	rows, err := db.Query(
		"SELECT id, updated_at FROM documents WHERE id IN ("+strings.Join(placeholders, ",")+")",
		args...,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	m := make(map[string]int64, len(ids))
	for rows.Next() {
		var id string
		var ts int64
		if rows.Scan(&id, &ts) == nil {
			m[id] = ts
		}
	}
	return m
}

// Search runs FTS + optional vector hybrid search with graph expansion.
// If embedder != nil and a vec store exists, also runs VecSearch and merges via HybridRank.
// Always performs GraphExpand and ConflictLinks on the result set.
func Search(db *sql.DB, kbRoot string, query string, layer *string, limit int, embedder Embedder) ([]SearchResult, []GraphNeighbor, []Conflict, error) {
	return SearchFiltered(db, kbRoot, query, layer, nil, limit, embedder)
}

// SearchFiltered is like Search but also accepts an optional kind filter.
func SearchFiltered(db *sql.DB, kbRoot string, query string, layer, kind *string, limit int, embedder Embedder) ([]SearchResult, []GraphNeighbor, []Conflict, error) {
	// Over-fetch for rerank: retrieve 4x candidates
	fetchLimit := limit * 4

	ftsResults, err := FTSSearchFiltered(db, query, layer, kind, fetchLimit)
	if err != nil {
		return nil, nil, nil, err
	}

	var results []SearchResult

	if embedder != nil && VecStoreExists(kbRoot) {
		queryVec, encErr := embedder.Encode(query)
		if encErr == nil {
			vecResults, _ := VecSearch(kbRoot, queryVec, layer, fetchLimit)
			// Collect graph boost for merged result set.
			allIDs := collectIDs(ftsResults, vecResults)
			boostMap := GraphBoost(db, allIDs)
			recencyMap := fetchRecencyMap(db, allIDs)
			results = HybridRank(ftsResults, vecResults, boostMap, recencyMap)
		} else {
			// Encoding failed; fall back to FTS only.
			results = applyGraphBoost(db, ftsResults)
		}
	} else {
		results = applyGraphBoost(db, ftsResults)
	}

	// Bi-encoder rerank: re-score candidates and truncate to limit.
	results = biEncoderRerank(query, results, embedder, limit)

	// Graph expansion and conflict detection on final result set.
	ids := make([]string, len(results))
	for i, r := range results {
		ids[i] = r.ID
	}
	neighbors := GraphExpand(db, ids, limit)
	conflicts := ConflictLinks(db, ids)

	return results, neighbors, conflicts, nil
}

// applyGraphBoost applies GraphBoost scores to FTS results and returns them.
func applyGraphBoost(db *sql.DB, ftsResults []SearchResult) []SearchResult {
	if len(ftsResults) == 0 {
		return ftsResults
	}
	ids := make([]string, len(ftsResults))
	for i, r := range ftsResults {
		ids[i] = r.ID
	}
	boostMap := GraphBoost(db, ids)
	for i := range ftsResults {
		ftsResults[i].GraphBoost = boostMap[ftsResults[i].ID]
	}
	return ftsResults
}

// collectIDs returns deduplicated IDs from two result slices.
func collectIDs(a, b []SearchResult) []string {
	seen := make(map[string]bool, len(a)+len(b))
	var ids []string
	for _, r := range a {
		if !seen[r.ID] {
			seen[r.ID] = true
			ids = append(ids, r.ID)
		}
	}
	for _, r := range b {
		if !seen[r.ID] {
			seen[r.ID] = true
			ids = append(ids, r.ID)
		}
	}
	return ids
}
