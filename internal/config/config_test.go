package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	if time.Duration(cfg.Sync.TelegramRequestInterval) != 2*time.Second {
		t.Fatalf("telegram request interval = %s, want 2s", cfg.Sync.TelegramRequestInterval)
	}
	if cfg.Storage.Path != "/data/tg-search" {
		t.Fatalf("storage path = %q, want /data/tg-search", cfg.Storage.Path)
	}
	if cfg.Storage.MaxDBSize != Size(10*1024*1024*1024) {
		t.Fatalf("max db size = %d, want 10GB", cfg.Storage.MaxDBSize)
	}
	if cfg.Storage.MaxMediaCache != Size(20*1024*1024*1024) {
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
	if cfg.Telegram.Media.Concurrency != 2 {
		t.Fatalf("telegram media concurrency = %d, want 2", cfg.Telegram.Media.Concurrency)
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
  media:
    concurrency: 4
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
	if cfg.Telegram.Media.Concurrency != 4 {
		t.Fatalf("telegram media concurrency = %d, want 4", cfg.Telegram.Media.Concurrency)
	}
}

func TestApplyRuntimeSettingsOverridesOperationalConfig(t *testing.T) {
	cfg := defaultConfig()
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 9900
	cfg.Storage.Path = "data"

	settings := RuntimeSettings{
		Sync: RuntimeSyncSettings{
			Workers:                 8,
			HistoryBatchSize:        250,
			TelegramRequestInterval: Duration(1500 * time.Millisecond),
		},
		Storage: RuntimeStorageSettings{
			MaxDBSize:     Size(30 * sizeGB),
			MaxMediaCache: Size(40 * sizeGB),
		},
		Telegram: RuntimeTelegramSettings{
			Proxy:            "socks5://127.0.0.1:1080",
			ReconnectTimeout: Duration(6 * time.Minute),
			DialTimeout:      Duration(15 * time.Second),
			RateLimit: TelegramRateLimitConfig{
				Enabled:       false,
				RatePerSecond: 12,
				Burst:         6,
			},
			Stream: TelegramStreamConfig{
				Concurrency:  4,
				Buffers:      8,
				ChunkTimeout: Duration(30 * time.Second),
			},
			Media: TelegramMediaConfig{
				Concurrency: 3,
			},
		},
	}

	got, err := ApplyRuntimeSettings(cfg, settings)
	if err != nil {
		t.Fatalf("ApplyRuntimeSettings returned error: %v", err)
	}

	if got.Server.Host != "127.0.0.1" || got.Server.Port != 9900 || got.Storage.Path != "data" {
		t.Fatalf("base server/storage paths changed: %+v", got)
	}
	if got.Sync.Workers != 8 || got.Sync.HistoryBatchSize != 250 || time.Duration(got.Sync.TelegramRequestInterval) != 1500*time.Millisecond {
		t.Fatalf("sync config = %+v", got.Sync)
	}
	if got.Storage.MaxDBSize != Size(30*sizeGB) || got.Storage.MaxMediaCache != Size(40*sizeGB) {
		t.Fatalf("storage limits = %+v", got.Storage)
	}
	if got.Telegram.Proxy != "socks5://127.0.0.1:1080" {
		t.Fatalf("telegram proxy = %q", got.Telegram.Proxy)
	}
	if got.Telegram.RateLimit.Enabled || got.Telegram.RateLimit.RatePerSecond != 12 || got.Telegram.RateLimit.Burst != 6 {
		t.Fatalf("rate limit = %+v", got.Telegram.RateLimit)
	}
	if got.Telegram.Stream.Concurrency != 4 || got.Telegram.Stream.Buffers != 8 || time.Duration(got.Telegram.Stream.ChunkTimeout) != 30*time.Second {
		t.Fatalf("stream config = %+v", got.Telegram.Stream)
	}
	if got.Telegram.Media.Concurrency != 3 {
		t.Fatalf("media concurrency = %d, want 3", got.Telegram.Media.Concurrency)
	}
}

func TestRuntimeSettingsPreserveAIMediaMetadataAPIKey(t *testing.T) {
	defaults := Config{}
	applyDefaults(&defaults)
	existing := RuntimeSettingsFromConfig(defaults)
	existing.AI.MediaMetadata = AIMediaMetadataSettings{
		Enabled:         true,
		Provider:        "groq",
		BaseURL:         "https://api.example.com/v1",
		APIKey:          "stored-key",
		Model:           "movie-model",
		FallbackEnabled: true,
	}
	incoming := existing
	incoming.AI.MediaMetadata.APIKey = ""

	merged := PreserveRuntimeSecrets(incoming, existing)

	if merged.AI.MediaMetadata.APIKey != "stored-key" {
		t.Fatalf("api key = %q, want stored-key", merged.AI.MediaMetadata.APIKey)
	}
	if merged.AI.MediaMetadata.Provider != "groq" {
		t.Fatalf("provider = %q, want groq", merged.AI.MediaMetadata.Provider)
	}
	if !merged.AI.MediaMetadata.FallbackEnabled {
		t.Fatal("fallback_enabled = false, want true")
	}
}

func TestRuntimeSettingsPreserveAIMediaMetadataProviderAPIKeys(t *testing.T) {
	defaults := Config{}
	applyDefaults(&defaults)
	existing := RuntimeSettingsFromConfig(defaults)
	existing.AI.MediaMetadata = AIMediaMetadataSettings{
		Enabled:         true,
		FallbackEnabled: true,
		Providers: []AIMediaMetadataProviderSettings{
			{ID: "groq-main", Provider: "groq", BaseURL: "https://api.groq.com/openai/v1", APIKey: "groq-key", Model: "llama-3.3-70b-versatile", Enabled: true},
			{ID: "ollama-local", Provider: "ollama", BaseURL: "http://localhost:11434/v1", Model: "qwen2.5:7b", Enabled: true},
		},
	}
	incoming := RuntimeSettingsFromConfig(defaults)
	incoming.AI.MediaMetadata = AIMediaMetadataSettings{
		Enabled:         true,
		FallbackEnabled: true,
		Providers: []AIMediaMetadataProviderSettings{
			{ID: "groq-main", Provider: "groq", BaseURL: "https://api.groq.com/openai/v1", Model: "llama-3.3-70b-versatile", Enabled: true},
			{ID: "ollama-local", Provider: "ollama", BaseURL: "http://localhost:11434/v1", Model: "qwen2.5:7b", Enabled: true},
		},
	}

	merged := PreserveRuntimeSecrets(incoming, existing)

	if merged.AI.MediaMetadata.Providers[0].APIKey != "groq-key" {
		t.Fatalf("provider api key = %q, want groq-key", merged.AI.MediaMetadata.Providers[0].APIKey)
	}
	if merged.AI.MediaMetadata.Providers[1].APIKey != "" {
		t.Fatalf("ollama api key = %q, want empty", merged.AI.MediaMetadata.Providers[1].APIKey)
	}
}

func TestAIMediaMetadataEffectiveProvidersBackfillsLegacySettings(t *testing.T) {
	settings := AIMediaMetadataSettings{
		Enabled:  true,
		Provider: "groq",
		BaseURL:  "https://api.groq.com/openai/v1",
		APIKey:   "secret",
		Model:    "llama-3.3-70b-versatile",
	}

	providers := settings.EffectiveProviders()

	if len(providers) != 1 {
		t.Fatalf("providers len = %d, want 1", len(providers))
	}
	if providers[0].ID != "groq" || providers[0].Provider != "groq" || providers[0].APIKey != "secret" || !providers[0].Enabled {
		t.Fatalf("provider = %+v", providers[0])
	}
}

func TestApplyRuntimeSettingsRejectsEnabledAIMediaMetadataWithoutRequiredFields(t *testing.T) {
	cfg := defaultConfig()
	settings := RuntimeSettingsFromConfig(cfg)
	settings.AI.MediaMetadata.Enabled = true

	_, err := ApplyRuntimeSettings(cfg, settings)

	if err == nil || !strings.Contains(err.Error(), "ai.media_metadata.base_url") {
		t.Fatalf("ApplyRuntimeSettings error = %v, want base_url validation", err)
	}

	settings.AI.MediaMetadata.BaseURL = "https://api.example.com/v1"
	_, err = ApplyRuntimeSettings(cfg, settings)
	if err == nil || !strings.Contains(err.Error(), "ai.media_metadata.api_key") {
		t.Fatalf("ApplyRuntimeSettings error = %v, want api_key validation", err)
	}

	settings.AI.MediaMetadata.APIKey = "secret"
	_, err = ApplyRuntimeSettings(cfg, settings)
	if err == nil || !strings.Contains(err.Error(), "ai.media_metadata.model") {
		t.Fatalf("ApplyRuntimeSettings error = %v, want model validation", err)
	}
}

func TestApplyRuntimeSettingsAllowsOllamaAIMediaMetadataWithoutAPIKey(t *testing.T) {
	cfg := defaultConfig()
	settings := RuntimeSettingsFromConfig(cfg)
	settings.AI.MediaMetadata.Enabled = true
	settings.AI.MediaMetadata.Provider = "ollama"
	settings.AI.MediaMetadata.BaseURL = "http://localhost:11434/v1"
	settings.AI.MediaMetadata.Model = "qwen2.5:7b"
	settings.AI.MediaMetadata.APIKey = ""

	_, err := ApplyRuntimeSettings(cfg, settings)
	if err != nil {
		t.Fatalf("ApplyRuntimeSettings error = %v, want nil", err)
	}
}

func TestRuntimeSettingsFromConfigExcludesStartupOnlyFields(t *testing.T) {
	cfg := defaultConfig()
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 9901
	cfg.Storage.Path = "data"

	settings := RuntimeSettingsFromConfig(cfg)

	if settings.Sync.Workers != cfg.Sync.Workers {
		t.Fatalf("settings sync workers = %d, want %d", settings.Sync.Workers, cfg.Sync.Workers)
	}
	if settings.Storage.MaxDBSize != cfg.Storage.MaxDBSize {
		t.Fatalf("settings max db size = %d, want %d", settings.Storage.MaxDBSize, cfg.Storage.MaxDBSize)
	}
	if settings.Telegram.Media.Concurrency != cfg.Telegram.Media.Concurrency {
		t.Fatalf("settings media concurrency = %d, want %d", settings.Telegram.Media.Concurrency, cfg.Telegram.Media.Concurrency)
	}
}

func TestApplyRuntimeSettingsRejectsStorageLimitsBelowMinimum(t *testing.T) {
	cfg := defaultConfig()
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 9900
	cfg.Storage.Path = "data"

	settings := RuntimeSettingsFromConfig(cfg)
	settings.Storage.MaxDBSize = Size(99 * 1024 * 1024)

	if _, err := ApplyRuntimeSettings(cfg, settings); err == nil {
		t.Fatal("ApplyRuntimeSettings returned nil error for max_db_size below 100MB")
	}

	settings = RuntimeSettingsFromConfig(cfg)
	settings.Storage.MaxMediaCache = Size(99 * 1024 * 1024)

	if _, err := ApplyRuntimeSettings(cfg, settings); err == nil {
		t.Fatal("ApplyRuntimeSettings returned nil error for max_media_cache below 100MB")
	}
}

func TestRuntimeSettingsJSONAcceptsReadableDurations(t *testing.T) {
	var settings RuntimeSettings
	err := json.Unmarshal([]byte(`{
		"sync":{"workers":8,"history_batch_size":250,"telegram_request_interval":"1500ms"},
		"storage":{"max_db_size":30000000000,"max_media_cache":40000000000},
		"telegram":{
			"proxy":"socks5://127.0.0.1:1080",
			"reconnect_timeout":"6m",
			"dial_timeout":"15s",
			"rate_limit":{"enabled":false,"rate_per_second":12,"burst":6},
			"stream":{"concurrency":4,"buffers":8,"chunk_timeout":"30s"},
			"media":{"concurrency":3}
		}
	}`), &settings)
	if err != nil {
		t.Fatalf("decode runtime settings JSON: %v", err)
	}
	if settings.Sync.TelegramRequestInterval != Duration(1500*time.Millisecond) {
		t.Fatalf("telegram request interval = %s, want 1500ms", settings.Sync.TelegramRequestInterval)
	}
	if settings.Telegram.Stream.ChunkTimeout != Duration(30*time.Second) {
		t.Fatalf("chunk timeout = %s, want 30s", settings.Telegram.Stream.ChunkTimeout)
	}
	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("marshal runtime settings JSON: %v", err)
	}
	if !strings.Contains(string(data), `"telegram_request_interval":"1.5s"`) || !strings.Contains(string(data), `"chunk_timeout":"30s"`) {
		t.Fatalf("runtime settings JSON uses unreadable durations: %s", data)
	}
}

func TestLoadAppliesSyncTelegramRequestInterval(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(`
sync:
  telegram_request_interval: 750ms
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if time.Duration(cfg.Sync.TelegramRequestInterval) != 750*time.Millisecond {
		t.Fatalf("telegram request interval = %s, want 750ms", cfg.Sync.TelegramRequestInterval)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Fatalf("server host = %q, want 0.0.0.0", cfg.Server.Host)
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
	assertGeneratedConfigIsStartupOnly(t, generated, "127.0.0.1", "data")
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
	assertGeneratedConfigIsStartupOnly(t, generated, "127.0.0.1", "data")
	if err := EnsureRuntimeDirs(cfg); err != nil {
		t.Fatalf("EnsureRuntimeDirs with generated config returned error: %v", err)
	}
}

func assertGeneratedConfigIsStartupOnly(t *testing.T, generated []byte, wantHost string, wantStoragePath string) {
	t.Helper()
	text := string(generated)
	for _, required := range []string{
		"server:",
		"host: " + wantHost,
		"port: 9900",
		"storage:",
		"path: " + wantStoragePath,
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("generated config missing %q:\n%s", required, text)
		}
	}
	for _, forbidden := range []string{
		"api_hash",
		"api_id",
		"sync:",
		"max_db_size",
		"max_media_cache",
		"telegram:",
		"proxy:",
		"reconnect_timeout",
		"dial_timeout",
		"rate_limit",
		"stream:",
		"media:",
		"concurrency",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("generated config contains settings-managed field %q:\n%s", forbidden, text)
		}
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

func TestDefaultConfig_TaskRetention(t *testing.T) {
	cfg := defaultConfig()

	assert.Equal(t, 7, cfg.TaskRetention.SucceededDays)
	assert.Equal(t, 30, cfg.TaskRetention.FailedDays)
	assert.Equal(t, 7, cfg.TaskRetention.CanceledDays)
	assert.Equal(t, 30, cfg.TaskRetention.PausedDays)
	assert.Equal(t, 30, cfg.TaskRetention.FloodWaitDays)
	assert.Equal(t, 7, cfg.TaskRetention.ReconnectingDays)
}

func TestValidate_TaskRetention(t *testing.T) {
	tests := []struct {
		name      string
		retention TaskRetentionConfig
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid all positive",
			retention: TaskRetentionConfig{
				SucceededDays:    7,
				FailedDays:       30,
				CanceledDays:     7,
				PausedDays:       30,
				FloodWaitDays:    30,
				ReconnectingDays: 7,
			},
			wantError: false,
		},
		{
			name: "valid with zeros",
			retention: TaskRetentionConfig{
				SucceededDays:    0,
				FailedDays:       0,
				CanceledDays:     0,
				PausedDays:       0,
				FloodWaitDays:    0,
				ReconnectingDays: 0,
			},
			wantError: false,
		},
		{
			name: "invalid succeeded_days negative",
			retention: TaskRetentionConfig{
				SucceededDays:    -1,
				FailedDays:       30,
				CanceledDays:     7,
				PausedDays:       30,
				FloodWaitDays:    30,
				ReconnectingDays: 7,
			},
			wantError: true,
			errorMsg:  "task_retention.succeeded_days must be >= 0",
		},
		{
			name: "invalid failed_days negative",
			retention: TaskRetentionConfig{
				SucceededDays:    7,
				FailedDays:       -1,
				CanceledDays:     7,
				PausedDays:       30,
				FloodWaitDays:    30,
				ReconnectingDays: 7,
			},
			wantError: true,
			errorMsg:  "task_retention.failed_days must be >= 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultConfig()
			cfg.TaskRetention = tt.retention

			err := validate(cfg)
			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

