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

func EnsureRuntimeDirs(cfg Config) error {
	if cfg.Storage.Path == "" {
		return errors.New("storage.path is required")
	}
	for _, path := range []string{
		cfg.Storage.Path,
		filepath.Join(cfg.Storage.Path, "sessions"),
		filepath.Join(cfg.Storage.Path, "logs"),
		filepath.Join(cfg.Storage.Path, "backup"),
		filepath.Join(cfg.Storage.Path, "uploads"),
		filepath.Join(cfg.Storage.Path, "index"),
		filepath.Join(cfg.Storage.Path, "thumbnails"),
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return fmt.Errorf("create runtime directory %s: %w", path, err)
		}
	}
	return nil
}

func defaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 6000,
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
