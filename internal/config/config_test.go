package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAppliesDefaultsFromLocalConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(`
server:
  host: 127.0.0.1
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Server.Host != "127.0.0.1" {
		t.Fatalf("host = %q, want default localhost", cfg.Server.Host)
	}
	if cfg.Server.Port != 9900 {
		t.Fatalf("port = %d, want 9900", cfg.Server.Port)
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

func TestLoadRejectsTelegramConfig(t *testing.T) {
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

	_, err = Load(configPath)
	if err == nil || !strings.Contains(err.Error(), "field telegram not found") {
		t.Fatalf("Load error = %v, want unknown telegram field error", err)
	}
}

func TestLoadGeneratesDefaultConfigAtExplicitMissingPath(t *testing.T) {
	dir := t.TempDir()
	work := filepath.Join(dir, "work")
	if err := os.Mkdir(work, 0o755); err != nil {
		t.Fatal(err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(work); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	configPath := filepath.Join(dir, "missing", "config.yaml")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Server.Host != "127.0.0.1" {
		t.Fatalf("generated host = %q, want 127.0.0.1", cfg.Server.Host)
	}
	if cfg.Storage.Path != "data" {
		t.Fatalf("generated storage path = %q, want data", cfg.Storage.Path)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("generated config not written: %v", err)
	}
	generated, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read generated config: %v", err)
	}
	if strings.Contains(string(generated), "telegram:") || strings.Contains(string(generated), "api_hash") || strings.Contains(string(generated), "api_id") {
		t.Fatalf("generated config contains Telegram API credentials:\n%s", string(generated))
	}
	if err := EnsureRuntimeDirs(cfg); err != nil {
		t.Fatalf("EnsureRuntimeDirs with generated config returned error: %v", err)
	}
}

func TestLoadGeneratesLocalDefaultConfigWhenNoConfigExists(t *testing.T) {
	dir := t.TempDir()
	work := filepath.Join(dir, "work")
	if err := os.Mkdir(work, 0o755); err != nil {
		t.Fatal(err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(work); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	t.Setenv("HOME", dir)
	oldDefaultPath := defaultConfigPath
	oldLocalPath := localConfigPath
	defaultConfigPath = filepath.Join(dir, "unwritable-parent", "config.yaml")
	localConfigPath = filepath.Join(dir, "config.yaml")
	t.Cleanup(func() {
		defaultConfigPath = oldDefaultPath
		localConfigPath = oldLocalPath
	})

	if err := os.WriteFile(filepath.Join(dir, "unwritable-parent"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Storage.Path != "data" {
		t.Fatalf("generated storage path = %q, want data", cfg.Storage.Path)
	}
	if _, err := os.Stat(localConfigPath); err != nil {
		t.Fatalf("local config was not generated: %v", err)
	}
	generated, err := os.ReadFile(localConfigPath)
	if err != nil {
		t.Fatalf("read local config: %v", err)
	}
	if strings.Contains(string(generated), "telegram:") || strings.Contains(string(generated), "api_hash") || strings.Contains(string(generated), "api_id") {
		t.Fatalf("local generated config contains Telegram API credentials:\n%s", string(generated))
	}
	if err := EnsureRuntimeDirs(cfg); err != nil {
		t.Fatalf("EnsureRuntimeDirs with generated config returned error: %v", err)
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

func TestRuntimeDirectoryValidation(t *testing.T) {
	cfg := Config{}
	cfg.Storage.Path = t.TempDir()

	if err := ValidateRuntimeDirs(cfg); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("ValidateRuntimeDirs returned %v, want missing directory error", err)
	}

	if err := EnsureRuntimeDirs(cfg); err != nil {
		t.Fatalf("EnsureRuntimeDirs returned error: %v", err)
	}
	if err := ValidateRuntimeDirs(cfg); err != nil {
		t.Fatalf("ValidateRuntimeDirs returned error after ensure: %v", err)
	}

	logs := filepath.Join(cfg.Storage.Path, "logs")
	if err := os.Remove(logs); err != nil {
		t.Fatalf("remove logs dir: %v", err)
	}
	if err := os.WriteFile(logs, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write logs file: %v", err)
	}
	if err := ValidateRuntimeDirs(cfg); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("ValidateRuntimeDirs returned %v, want not a directory error", err)
	}
}

func TestRuntimeDirectoryValidationDetectsUnwritableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write to read-only directories")
	}
	cfg := Config{}
	cfg.Storage.Path = t.TempDir()
	if err := EnsureRuntimeDirs(cfg); err != nil {
		t.Fatalf("EnsureRuntimeDirs returned error: %v", err)
	}
	index := filepath.Join(cfg.Storage.Path, "index")
	if err := os.Chmod(index, 0o555); err != nil {
		t.Fatalf("chmod index: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(index, 0o755) })

	if err := ValidateRuntimeDirs(cfg); err == nil || !strings.Contains(err.Error(), "not writable") {
		t.Fatalf("ValidateRuntimeDirs returned %v, want not writable error", err)
	}
}

func TestDatabasePathUsesProductDatabaseName(t *testing.T) {
	cfg := Config{}
	cfg.Storage.Path = "/data/tg-search"

	if got := DatabasePath(cfg); got != "/data/tg-search/tg-search.db" {
		t.Fatalf("DatabasePath = %q, want /data/tg-search/tg-search.db", got)
	}
}
