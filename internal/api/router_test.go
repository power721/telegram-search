package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
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

func TestRouterLogsInternalServerErrors(t *testing.T) {
	core, observed := observer.New(zapcore.DebugLevel)
	deps := testDeps(t)
	deps.Logger = zap.New(core)
	router := NewRouter(deps)
	router.GET("/api/test-internal-error", func(c *gin.Context) {
		errorJSON(c, http.StatusInternalServerError, errors.New("database failed"))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/test-internal-error", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d body=%s, want 500", w.Code, w.Body.String())
	}
	entries := observed.FilterMessage("api request failed").All()
	if len(entries) != 1 {
		t.Fatalf("logged internal errors = %d, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["method"] != http.MethodGet || fields["path"] != "/api/test-internal-error" || fields["status"] != int64(http.StatusInternalServerError) || fields["error"] != "database failed" {
		t.Fatalf("log fields = %+v", fields)
	}
}

func TestRouterLogsServiceUnavailableErrors(t *testing.T) {
	core, observed := observer.New(zapcore.DebugLevel)
	deps := testDeps(t)
	deps.Logger = zap.New(core)
	router := NewRouter(deps)
	router.GET("/api/test-service-unavailable", func(c *gin.Context) {
		errorText(c, http.StatusServiceUnavailable, "tasks are unavailable")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/test-service-unavailable", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body=%s, want 503", w.Code, w.Body.String())
	}
	entries := observed.FilterMessage("api request failed").All()
	if len(entries) != 1 {
		t.Fatalf("logged service unavailable errors = %d, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["method"] != http.MethodGet || fields["path"] != "/api/test-service-unavailable" || fields["status"] != int64(http.StatusServiceUnavailable) || fields["error"] != "tasks are unavailable" {
		t.Fatalf("log fields = %+v", fields)
	}
}
