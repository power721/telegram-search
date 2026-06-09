package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestLoadAppliesTelegramRuntimeDefaults(t *testing.T) {
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

	if cfg.Telegram.Proxy != "" {
		t.Fatalf("telegram proxy = %q, want empty default", cfg.Telegram.Proxy)
	}
	if time.Duration(cfg.Telegram.ReconnectTimeout) != 5*time.Minute {
		t.Fatalf("telegram reconnect timeout = %s, want 5m", cfg.Telegram.ReconnectTimeout)
	}
	if time.Duration(cfg.Telegram.DialTimeout) != 10*time.Second {
		t.Fatalf("telegram dial timeout = %s, want 10s", cfg.Telegram.DialTimeout)
	}
	if !cfg.Telegram.RateLimit.Enabled {
		t.Fatal("telegram rate limit enabled = false, want true")
	}
	if cfg.Telegram.RateLimit.RatePerSecond != 10 {
		t.Fatalf("telegram rate = %d, want 10", cfg.Telegram.RateLimit.RatePerSecond)
	}
	if cfg.Telegram.RateLimit.Burst != 5 {
		t.Fatalf("telegram burst = %d, want 5", cfg.Telegram.RateLimit.Burst)
	}
	if cfg.Telegram.Stream.Concurrency != 2 {
		t.Fatalf("telegram stream concurrency = %d, want 2", cfg.Telegram.Stream.Concurrency)
	}
	if cfg.Telegram.Stream.Buffers != 4 {
		t.Fatalf("telegram stream buffers = %d, want 4", cfg.Telegram.Stream.Buffers)
	}
	if time.Duration(cfg.Telegram.Stream.ChunkTimeout) != 20*time.Second {
		t.Fatalf("telegram stream chunk timeout = %s, want 20s", cfg.Telegram.Stream.ChunkTimeout)
	}
}

func TestLoadAppliesTelegramRuntimeConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(`
telegram:
  proxy: socks5://127.0.0.1:1080
  reconnect_timeout: 2m
  dial_timeout: 3s
  rate_limit:
    enabled: false
    rate_per_second: 7
    burst: 2
  stream:
    concurrency: 3
    buffers: 6
    chunk_timeout: 15s
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Telegram.Proxy != "socks5://127.0.0.1:1080" {
		t.Fatalf("telegram proxy = %q", cfg.Telegram.Proxy)
	}
	if time.Duration(cfg.Telegram.ReconnectTimeout) != 2*time.Minute {
		t.Fatalf("telegram reconnect timeout = %s, want 2m", cfg.Telegram.ReconnectTimeout)
	}
	if time.Duration(cfg.Telegram.DialTimeout) != 3*time.Second {
		t.Fatalf("telegram dial timeout = %s, want 3s", cfg.Telegram.DialTimeout)
	}
	if cfg.Telegram.RateLimit.Enabled {
		t.Fatal("telegram rate limit enabled = true, want false")
	}
	if cfg.Telegram.RateLimit.RatePerSecond != 7 {
		t.Fatalf("telegram rate = %d, want 7", cfg.Telegram.RateLimit.RatePerSecond)
	}
	if cfg.Telegram.RateLimit.Burst != 2 {
		t.Fatalf("telegram burst = %d, want 2", cfg.Telegram.RateLimit.Burst)
	}
	if cfg.Telegram.Stream.Concurrency != 3 {
		t.Fatalf("telegram stream concurrency = %d, want 3", cfg.Telegram.Stream.Concurrency)
	}
	if cfg.Telegram.Stream.Buffers != 6 {
		t.Fatalf("telegram stream buffers = %d, want 6", cfg.Telegram.Stream.Buffers)
	}
	if time.Duration(cfg.Telegram.Stream.ChunkTimeout) != 15*time.Second {
		t.Fatalf("telegram stream chunk timeout = %s, want 15s", cfg.Telegram.Stream.ChunkTimeout)
	}
}

func TestLoadRejectsTelegramCredentialConfig(t *testing.T) {
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
	if err == nil || !strings.Contains(err.Error(), "field api_id not found") {
		t.Fatalf("Load error = %v, want unknown telegram credential field error", err)
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
	if strings.Contains(string(generated), "api_hash") || strings.Contains(string(generated), "api_id") {
		t.Fatalf("generated config contains Telegram API credentials:\n%s", string(generated))
	}
	if !strings.Contains(string(generated), "reconnect_timeout: 5m0s") || !strings.Contains(string(generated), "dial_timeout: 10s") {
		t.Fatalf("generated config contains unreadable telegram durations:\n%s", string(generated))
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
	if strings.Contains(string(generated), "api_hash") || strings.Contains(string(generated), "api_id") {
		t.Fatalf("local generated config contains Telegram API credentials:\n%s", string(generated))
	}
	if !strings.Contains(string(generated), "reconnect_timeout: 5m0s") || !strings.Contains(string(generated), "dial_timeout: 10s") {
		t.Fatalf("local generated config contains unreadable telegram durations:\n%s", string(generated))
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
