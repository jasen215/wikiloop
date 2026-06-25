//go:build fts5

package webui

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jasen215/wikiloop/internal/config"
	"github.com/jasen215/wikiloop/internal/kb"
)

// kbErrToHTTP writes the appropriate HTTP status code and JSON error body.
func kbErrToHTTP(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	var e *kb.KBError
	if errors.As(err, &e) {
		code = e.Code
	}
	w.WriteHeader(code)
	writeJSON(w, map[string]interface{}{"error": err.Error()})
}

// handleStatus returns document/embedding counts, by-layer breakdown, and index file size.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	result, err := kb.KBStatus(s.kbRoot)
	if err != nil {
		kbErrToHTTP(w, err)
		return
	}
	writeJSON(w, result)
}

// handleSearch runs layered FTS search and returns results with related docs.
// Uses the same SearchLayered logic as the MCP kb_search tool.
// Query params: q (required), layer (optional), kind (optional), limit (optional, default 10).
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, map[string]interface{}{"results": []kb.SearchResult{}})
		return
	}
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	var layer *string
	if l := r.URL.Query().Get("layer"); l != "" {
		layer = &l
	}
	var kind *string
	if k := r.URL.Query().Get("kind"); k != "" {
		kind = &k
	}
	sourceLimit := limit
	synthLimit := min(3, sourceLimit/2)
	if synthLimit < 1 {
		synthLimit = 1
	}
	resp, err := kb.KBSearch(s.kbRoot, q, layer, kind, sourceLimit, synthLimit)
	if err != nil {
		kbErrToHTTP(w, err)
		return
	}
	writeJSON(w, resp)
}

// handleFiles lists files in the raw/ subdirectory of kbRoot.
func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	files, err := kb.KBListFiles(s.kbRoot)
	if err != nil {
		kbErrToHTTP(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{"files": files})
}

// handleUpload saves an uploaded file to raw/ under kbRoot.
// Accepts multipart/form-data with a "file" field.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		kbErrToHTTP(w, &kb.KBError{Code: 400, Message: "parse form: " + err.Error()})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		kbErrToHTTP(w, &kb.KBError{Code: 400, Message: "read file field: " + err.Error()})
		return
	}
	defer file.Close()

	if err := kb.KBUpload(s.kbRoot, header.Filename, file); err != nil {
		kbErrToHTTP(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{"ok": true, "filename": filepath.Base(header.Filename)})
}

// handleImportLark imports a Lark/Feishu Wiki URL and expands embedded Base
// tables into searchable local text datasets.
func (s *Server) handleImportLark(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"error": "invalid JSON: " + err.Error()})
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		writeJSON(w, map[string]interface{}{"error": "Lark Wiki URL is required"})
		return
	}
	result, err := s.importLark(r.Context(), s.kbRoot, req.URL, req.Name)
	if err != nil {
		writeJSON(w, map[string]interface{}{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{
		"ok":                 true,
		"document_path":      result.DocumentPath,
		"table_paths":        result.TablePaths,
		"table_rows":         result.TableRows,
		"dataset_path":       result.DatasetPath,
		"total_rows":         result.TotalRows,
		"unique_rows":        result.UniqueRows,
		"duplicates_removed": result.DuplicatesRemoved,
	})
}

// handleSettings reads (GET) or writes (PUT) config.yaml.
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, err := config.Load(s.kbRoot)
		if err != nil {
			writeJSON(w, map[string]interface{}{"error": err.Error()})
			return
		}
		writeJSON(w, map[string]interface{}{
			"server": map[string]interface{}{
				"host": cfg.Server.Host,
				"port": cfg.Server.Port,
			},
			"distill": map[string]interface{}{
				"base_url":         cfg.Distill.BaseURL,
				"model":            cfg.Distill.Model,
				"api_type":         cfg.Distill.APIType,
				"token_configured": cfg.Distill.Token != "",
			},
			"embedding": map[string]interface{}{
				"idle_timeout": cfg.Embedding.IdleTimeout.String(),
			},
			"ui": map[string]interface{}{
				"language": cfg.UI.Language,
			},
		})

	case http.MethodPut:
		var req config.SettingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, map[string]interface{}{"error": "invalid JSON: " + err.Error()})
			return
		}
		cfg, err := config.Load(s.kbRoot)
		if err != nil {
			writeJSON(w, map[string]interface{}{"error": err.Error()})
			return
		}
		if req.Distill.BaseURL != nil {
			cfg.Distill.BaseURL = *req.Distill.BaseURL
		}
		if req.Distill.Token != nil {
			cfg.Distill.Token = *req.Distill.Token
		}
		if req.Distill.Model != nil {
			cfg.Distill.Model = *req.Distill.Model
		}
		if req.Distill.APIType != nil {
			cfg.Distill.APIType = *req.Distill.APIType
		}
		if req.Embedding.IdleTimeout != nil {
			if d, err := time.ParseDuration(*req.Embedding.IdleTimeout); err == nil {
				cfg.Embedding.IdleTimeout = d
			}
		}
		if req.UI.Language != nil {
			if *req.UI.Language == "zh" || *req.UI.Language == "en" {
				cfg.UI.Language = *req.UI.Language
			}
		}
		if err := config.Save(s.kbRoot, cfg); err != nil {
			writeJSON(w, map[string]interface{}{"error": "save config: " + err.Error()})
			return
		}
		writeJSON(w, map[string]interface{}{"ok": true})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// writeJSON encodes v as JSON and writes it to w with Content-Type application/json.
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// handleReindex triggers FTS index rebuild. POST /api/reindex?full=true
func (s *Server) handleReindex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	full := r.URL.Query().Get("full") == "true"
	result, err := kb.KBReindex(s.kbRoot, full)
	if err != nil {
		kbErrToHTTP(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{"ok": true, "written": result.Written})
}

// handleLint runs health checks over wiki pages. GET /api/lint
func (s *Server) handleLint(w http.ResponseWriter, r *http.Request) {
	result, err := kb.KBLint(s.kbRoot)
	if err != nil {
		kbErrToHTTP(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{"ok": true, "warnings": result.Warnings, "count": result.Count})
}
