package logviewer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListFiltersOrdersAndPaginatesLogs(t *testing.T) {
	dir := t.TempDir()
	writeLog(t, dir, "app.log", strings.Join([]string{
		`{"level":"info","ts":"2026-06-09T10:00:00.000+0800","caller":"api/router.go:1","msg":"server started","address":"127.0.0.1:9900"}`,
		`{"level":"warn","ts":"2026-06-09T10:01:00.000+0800","caller":"api/router.go:2","msg":"slow request","path":"/api/search"}`,
	}, "\n")+"\n")
	writeLog(t, dir, "sync.log", strings.Join([]string{
		`{"level":"info","ts":"2026-06-09T10:02:00.000+0800","caller":"history/service.go:1","msg":"history sync completed","channel_id":100}`,
		`plain fallback line`,
	}, "\n")+"\n")

	result, err := New(dir).List(Query{Level: "info", Text: "sync", Order: OrderDesc, Limit: 1})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("total = %d, want 1", result.Total)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(result.Items))
	}
	item := result.Items[0]
	if item.File != "sync.log" || item.Level != "info" || item.Message != "history sync completed" {
		t.Fatalf("item = %+v, want sync info entry", item)
	}
	if item.Time == nil {
		t.Fatal("item time is nil")
	}
	if item.Fields["channel_id"].(float64) != 100 {
		t.Fatalf("channel_id field = %v, want 100", item.Fields["channel_id"])
	}

	secondPage, err := New(dir).List(Query{Order: OrderAsc, Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("List second page returned error: %v", err)
	}
	if secondPage.Total != 4 {
		t.Fatalf("second page total = %d, want 4", secondPage.Total)
	}
	if got := secondPage.Items[0].Message; got != "slow request" {
		t.Fatalf("second page first message = %q, want slow request", got)
	}
}

func TestPathRejectsInvalidLogFile(t *testing.T) {
	_, err := New(t.TempDir()).Path("../app.log")
	if err != ErrInvalidLogFile {
		t.Fatalf("Path error = %v, want ErrInvalidLogFile", err)
	}
}

func writeLog(t *testing.T, dir string, name string, data string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
