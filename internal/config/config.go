package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const DefaultPath = "/data/tg-search/config.yaml"

type Config struct {
	Telegram TelegramConfig `yaml:"telegram"`
	Server   ServerConfig   `yaml:"server"`
	Sync     SyncConfig     `yaml:"sync"`
	Storage  StorageConfig  `yaml:"storage"`
}

type TelegramConfig struct {
	APIID   int    `yaml:"api_id"`
	APIHash string `yaml:"api_hash"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type SyncConfig struct {
	Workers          int `yaml:"workers"`
	HistoryBatchSize int `yaml:"history_batch_size"`
}

type StorageConfig struct {
	Path          string `yaml:"path"`
	MaxDBSize     Size   `yaml:"max_db_size"`
	MaxMediaCache Size   `yaml:"max_media_cache"`
}

func Load(path string) (Config, error) {
	cfg := defaultConfig()
	resolved, err := resolvePath(path)
	if err != nil {
		return Config{}, err
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", resolved, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", resolved, err)
	}
	applyDefaults(&cfg)
	if err := validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Address(cfg Config) string {
	return fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
}

func DatabasePath(cfg Config) string {
	return filepath.Join(cfg.Storage.Path, "tg-search.db")
}

func EnsureRuntimeDirs(cfg Config) error {
	if cfg.Storage.Path == "" {
		return errors.New("storage.path is required")
	}
	for _, path := range RuntimeDirs(cfg) {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return fmt.Errorf("create runtime directory %s: %w", path, err)
		}
	}
	return ValidateRuntimeDirs(cfg)
}

func ValidateRuntimeDirs(cfg Config) error {
	if cfg.Storage.Path == "" {
		return errors.New("storage.path is required")
	}
	for _, path := range RuntimeDirs(cfg) {
		info, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("runtime directory %s is missing", path)
		}
		if err != nil {
			return fmt.Errorf("stat runtime directory %s: %w", path, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("runtime path %s is not a directory", path)
		}
		if err := probeWritable(path); err != nil {
			return fmt.Errorf("runtime directory %s is not writable: %w", path, err)
		}
	}
	return nil
}

func RuntimeDirs(cfg Config) []string {
	root := cfg.Storage.Path
	return []string{
		root,
		filepath.Join(root, "sessions"),
		filepath.Join(root, "logs"),
		filepath.Join(root, "backup"),
		filepath.Join(root, "uploads"),
		filepath.Join(root, "index"),
		filepath.Join(root, "thumbnails"),
	}
}

func probeWritable(dir string) error {
	file, err := os.CreateTemp(dir, ".tg-search-write-test-*")
	if err != nil {
		return err
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Remove(name)
}

func defaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 9900,
		},
		Sync: SyncConfig{
			Workers:          5,
			HistoryBatchSize: 100,
		},
		Storage: StorageConfig{
			Path:          "/data/tg-search",
			MaxDBSize:     Size(10 * 1000 * 1000 * 1000),
			MaxMediaCache: Size(20 * 1000 * 1000 * 1000),
		},
	}
}

func resolvePath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	if _, err := os.Stat(DefaultPath); err == nil {
		return DefaultPath, nil
	}
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml", nil
	}
	return "", fmt.Errorf("config not found at %s or local config.yaml", DefaultPath)
}

func applyDefaults(cfg *Config) {
	defaults := defaultConfig()
	if cfg.Server.Host == "" {
		cfg.Server.Host = defaults.Server.Host
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = defaults.Server.Port
	}
	if cfg.Sync.Workers == 0 {
		cfg.Sync.Workers = defaults.Sync.Workers
	}
	if cfg.Sync.HistoryBatchSize == 0 {
		cfg.Sync.HistoryBatchSize = defaults.Sync.HistoryBatchSize
	}
	if cfg.Storage.Path == "" {
		cfg.Storage.Path = defaults.Storage.Path
	}
	if cfg.Storage.MaxDBSize == 0 {
		cfg.Storage.MaxDBSize = defaults.Storage.MaxDBSize
	}
	if cfg.Storage.MaxMediaCache == 0 {
		cfg.Storage.MaxMediaCache = defaults.Storage.MaxMediaCache
	}
}

func validate(cfg Config) error {
	if cfg.Telegram.APIID == 0 {
		return errors.New("telegram.api_id is required")
	}
	if cfg.Telegram.APIHash == "" {
		return errors.New("telegram.api_hash is required")
	}
	if cfg.Server.Host == "" {
		return errors.New("server.host is required")
	}
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535")
	}
	if cfg.Sync.Workers <= 0 {
		return errors.New("sync.workers must be greater than zero")
	}
	if cfg.Sync.HistoryBatchSize <= 0 {
		return errors.New("sync.history_batch_size must be greater than zero")
	}
	if cfg.Storage.Path == "" {
		return errors.New("storage.path is required")
	}
	return nil
}
