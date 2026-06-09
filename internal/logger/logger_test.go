package logger

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

func TestAppLoggerWritesToConsole(t *testing.T) {
	var console bytes.Buffer
	dir := t.TempDir()
	logs, err := newWithConsole(dir, zapcore.AddSync(&console))
	if err != nil {
		t.Fatalf("newWithConsole returned error: %v", err)
	}
	defer func() {
		_ = logs.Sync()
	}()

	logs.App.Info("api server listening", zap.String("address", "0.0.0.0:8080"))
	_ = logs.Sync()

	output := console.String()
	if !strings.Contains(output, "api server listening") || !strings.Contains(output, "0.0.0.0:8080") {
		t.Fatalf("console output = %q, want listening address", output)
	}
}

func TestRotatingWriterUsesRequiredPolicy(t *testing.T) {
	writer := newRotatingWriter(filepath.Join(t.TempDir(), "app.log"))

	if writer.MaxSize != 10 {
		t.Fatalf("MaxSize = %d, want 10", writer.MaxSize)
	}
	if writer.MaxBackups != 5 {
		t.Fatalf("MaxBackups = %d, want 5", writer.MaxBackups)
	}
	if !writer.Compress {
		t.Fatal("Compress = false, want true")
	}
}

func TestNewRotatesAndCompressesLogsAt10MB(t *testing.T) {
	dir := t.TempDir()
	logs, err := New(dir)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer func() {
		_ = logs.Sync()
	}()

	payload := strings.Repeat("x", 1024*1024)
	for i := 0; i < 11; i++ {
		logs.App.Info("large app event", zap.Int("index", i), zap.String("payload", payload))
	}
	_ = logs.Sync()

	var matches []string
	deadline := time.Now().Add(2 * time.Second)
	for {
		matches, err = filepath.Glob(filepath.Join(dir, "app-*.log.gz"))
		if err != nil {
			t.Fatalf("glob rotated logs: %v", err)
		}
		if len(matches) > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(matches) == 0 {
		t.Fatalf("expected compressed rotated app log after exceeding 10MB")
	}
	if len(matches) > 5 {
		t.Fatalf("expected at most 5 compressed app backups, got %d", len(matches))
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
