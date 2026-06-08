package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAppliesDefaultsFromLocalConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(`
telegram:
  api_id: 12345
  api_hash: test_hash
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Telegram.APIID != 12345 {
		t.Fatalf("api id = %d, want 12345", cfg.Telegram.APIID)
	}
	if cfg.Telegram.APIHash != "test_hash" {
		t.Fatalf("api hash = %q, want test_hash", cfg.Telegram.APIHash)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Fatalf("host = %q, want default localhost", cfg.Server.Host)
	}
	if cfg.Server.Port != 6000 {
		t.Fatalf("port = %d, want 6000", cfg.Server.Port)
	}
	if cfg.Sync.Workers != 5 {
		t.Fatalf("workers = %d, want 5", cfg.Sync.Workers)
	}
	if cfg.Sync.HistoryBatchSize != 100 {
		t.Fatalf("history batch size = %d, want 100", cfg.Sync.HistoryBatchSize)
	}
	if cfg.Storage.Path != "/data/tg-search" {
		t.Fatalf("storage path = %q, want /data/tg-search", cfg.Storage.Path)
	}
	if cfg.Storage.MaxDBSize != Size(10*1000*1000*1000) {
		t.Fatalf("max db size = %d, want 10GB", cfg.Storage.MaxDBSize)
	}
	if cfg.Storage.MaxMediaCache != Size(20*1000*1000*1000) {
		t.Fatalf("max media cache = %d, want 20GB", cfg.Storage.MaxMediaCache)
	}
}

func TestLoadRequiresTelegramCredentials(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(`
server:
  host: 127.0.0.1
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Load(configPath)
	if err == nil {
		t.Fatal("Load returned nil error, want missing Telegram credentials error")
	}
}

func TestEnsureRuntimeDirsCreatesStorageLayout(t *testing.T) {
	cfg := Config{}
	cfg.Storage.Path = t.TempDir()

	if err := EnsureRuntimeDirs(cfg); err != nil {
		t.Fatalf("EnsureRuntimeDirs returned error: %v", err)
	}

	for _, rel := range []string{"sessions", "logs", "backup", "uploads", "index", "thumbnails"} {
		path := filepath.Join(cfg.Storage.Path, rel)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s is not a directory", path)
		}
	}
}
