//go:build fts5

package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func newTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(dir+"/index", 0o755)
	s := &Server{kbRoot: dir}
	return s, dir
}

func TestSettingsGetIncludesLanguage(t *testing.T) {
	s, dir := newTestServer(t)
	os.WriteFile(dir+"/config.yaml", []byte("ui:\n  language: \"zh\"\n"), 0o644)

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	w := httptest.NewRecorder()
	s.handleSettings(w, req)

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	ui, ok := resp["ui"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing 'ui' key")
	}
	if ui["language"] != "zh" {
		t.Errorf("expected zh, got %v", ui["language"])
	}
}

func TestSettingsPutLanguage(t *testing.T) {
	s, dir := newTestServer(t)
	os.WriteFile(dir+"/config.yaml", []byte("ui:\n  language: \"zh\"\n"), 0o644)

	body := `{"ui":{"language":"en"}}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.handleSettings(w, req)

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["ok"] != true {
		t.Fatalf("PUT failed: %v", resp)
	}

	// Verify written to disk
	data, _ := os.ReadFile(dir + "/config.yaml")
	if !strings.Contains(string(data), `"en"`) {
		t.Errorf("config.yaml should contain en, got: %s", data)
	}
}
