package logger

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestNewWritesNamedLogFiles(t *testing.T) {
	dir := t.TempDir()
	logs, err := New(dir)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer func() {
		_ = logs.Sync()
	}()

	logs.App.Info("app event")
	logs.SyncLog.Info("sync event")
	logs.Telegram.Info("telegram event")
	logs.App.Error("error event")
	_ = logs.Sync()

	for _, name := range []string{"app.log", "sync.log", "telegram.log", "error.log"} {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if info.Size() == 0 {
			t.Fatalf("%s is empty", name)
		}
	}
}

func TestNopReturnsUsableLoggers(t *testing.T) {
	logs := Nop()
	logs.App.Info("ok")
	logs.SyncLog.Info("ok")
	logs.Telegram.Info("ok")
	logs.Error.Error("ok")
	if logs.App == (*zap.Logger)(nil) {
		t.Fatal("app logger is nil")
	}
}
