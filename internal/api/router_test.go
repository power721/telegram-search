package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFrontendFallback(t *testing.T) {
	deps := testDeps(t)
	router := NewRouter(deps)

	for _, path := range []string{"/", "/channels"} {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			router.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("%s status = %d body=%s, want 200", path, w.Code, w.Body.String())
			}
			if contentType := w.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
				t.Fatalf("%s content type = %q, want text/html", path, contentType)
			}
			if !strings.Contains(w.Body.String(), `<div id="app">`) {
				t.Fatalf("%s body missing Vue app shell: %s", path, w.Body.String())
			}
		})
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	withAdminSession(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("/api/status status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	if contentType := w.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("/api/status content type = %q, want application/json", contentType)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("/api/status invalid JSON: %v", err)
	}
	if body["service"] != "ok" {
		t.Fatalf("/api/status body = %+v, want service ok", body)
	}
}
