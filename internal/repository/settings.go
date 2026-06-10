package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"tg-search/internal/config"
	"tg-search/internal/model"
	"tg-search/internal/telegram"
)

type SettingsRepository struct {
	db *sql.DB
}

const (
	telegramAPISettingKey = "telegram_api"
	telegramBotSettingKey = "telegram_bot"
	runtimeSettingKey     = "runtime"
)

type telegramAPISettingsJSON struct {
	AppID   int    `json:"app_id"`
	AppHash string `json:"app_hash"`
}

type telegramBotSettingsJSON struct {
	Enabled      bool            `json:"enabled"`
	Token        string          `json:"token"`
	PollInterval config.Duration `json:"poll_interval"`
}

func NewSettingsRepository(db *sql.DB) *SettingsRepository {
	return &SettingsRepository{db: db}
}

func (r *SettingsRepository) Set(ctx context.Context, key string, valueJSON string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
INSERT INTO settings (key, value_json, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(key) DO UPDATE SET
  value_json = excluded.value_json,
  updated_at = excluded.updated_at`,
		key, valueJSON, now)
	if err != nil {
		return fmt.Errorf("set setting: %w", err)
	}
	return nil
}

func (r *SettingsRepository) Get(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := r.db.QueryRowContext(ctx, `SELECT value_json FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get setting: %w", err)
	}
	return value, true, nil
}

func (r *SettingsRepository) SaveTelegramAPI(ctx context.Context, settings model.TelegramAPISettings) error {
	data, err := json.Marshal(telegramAPISettingsJSON{
		AppID:   settings.AppID,
		AppHash: settings.AppHash,
	})
	if err != nil {
		return fmt.Errorf("marshal telegram api settings: %w", err)
	}
	if err := r.Set(ctx, telegramAPISettingKey, string(data)); err != nil {
		return fmt.Errorf("save telegram api settings: %w", err)
	}
	return nil
}

func (r *SettingsRepository) LoadTelegramAPI(ctx context.Context) (model.TelegramAPISettings, error) {
	raw, ok, err := r.Get(ctx, telegramAPISettingKey)
	if err != nil {
		return model.TelegramAPISettings{}, fmt.Errorf("load telegram api settings: %w", err)
	}
	if !ok {
		return defaultTelegramAPISettings(), nil
	}
	var settings model.TelegramAPISettings
	var stored telegramAPISettingsJSON
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return model.TelegramAPISettings{}, fmt.Errorf("decode telegram api settings: %w", err)
	}
	settings.AppID = stored.AppID
	settings.AppHash = stored.AppHash
	return settings, nil
}

func (r *SettingsRepository) SaveTelegramBot(ctx context.Context, settings config.BotConfig) error {
	data, err := json.Marshal(telegramBotSettingsJSON{
		Enabled:      settings.Enabled,
		Token:        settings.Token,
		PollInterval: settings.PollInterval,
	})
	if err != nil {
		return fmt.Errorf("marshal telegram bot settings: %w", err)
	}
	if err := r.Set(ctx, telegramBotSettingKey, string(data)); err != nil {
		return fmt.Errorf("save telegram bot settings: %w", err)
	}
	return nil
}

func (r *SettingsRepository) LoadTelegramBot(ctx context.Context, defaults config.BotConfig) (config.BotConfig, error) {
	raw, ok, err := r.Get(ctx, telegramBotSettingKey)
	if err != nil {
		return config.BotConfig{}, fmt.Errorf("load telegram bot settings: %w", err)
	}
	settings := defaults
	if !ok {
		return settings, nil
	}
	var stored telegramBotSettingsJSON
	stored.Enabled = settings.Enabled
	stored.Token = settings.Token
	stored.PollInterval = settings.PollInterval
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return config.BotConfig{}, fmt.Errorf("decode telegram bot settings: %w", err)
	}
	settings.Enabled = stored.Enabled
	settings.Token = stored.Token
	settings.PollInterval = stored.PollInterval
	return settings, nil
}

func (r *SettingsRepository) SaveRuntimeSettings(ctx context.Context, settings config.RuntimeSettings) error {
	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal runtime settings: %w", err)
	}
	if err := r.Set(ctx, runtimeSettingKey, string(data)); err != nil {
		return fmt.Errorf("save runtime settings: %w", err)
	}
	return nil
}

func (r *SettingsRepository) LoadRuntimeSettings(ctx context.Context, defaults config.Config) (config.RuntimeSettings, error) {
	raw, ok, err := r.Get(ctx, runtimeSettingKey)
	if err != nil {
		return config.RuntimeSettings{}, fmt.Errorf("load runtime settings: %w", err)
	}
	if !ok {
		return config.RuntimeSettingsFromConfig(defaults), nil
	}
	settings := config.RuntimeSettingsFromConfig(defaults)
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return config.RuntimeSettings{}, fmt.Errorf("decode runtime settings: %w", err)
	}
	return settings, nil
}

func RedactTelegramAPI(settings model.TelegramAPISettings) model.TelegramAPISettingsResponse {
	if isDefaultTelegramAPI(settings) {
		return model.TelegramAPISettingsResponse{}
	}
	return model.TelegramAPISettingsResponse{
		Configured: settings.AppID > 0 && settings.AppHash != "",
		AppID:      settings.AppID,
		AppHashSet: settings.AppHash != "",
	}
}

func RedactTelegramBot(settings config.BotConfig) model.TelegramBotSettingsResponse {
	return model.TelegramBotSettingsResponse{
		Enabled:      settings.Enabled,
		Configured:   settings.Enabled && settings.Token != "",
		TokenSet:     settings.Token != "",
		PollInterval: settings.PollInterval.String(),
	}
}

type TelegramCredentialsProvider struct {
	settings *SettingsRepository
}

func NewTelegramCredentialsProvider(settings *SettingsRepository) *TelegramCredentialsProvider {
	return &TelegramCredentialsProvider{settings: settings}
}

func (p *TelegramCredentialsProvider) TelegramCredentials(ctx context.Context) (telegram.Credentials, error) {
	if p == nil || p.settings == nil {
		return telegram.Credentials{}, telegram.ErrCredentialsNotConfigured
	}
	settings, err := p.settings.LoadTelegramAPI(ctx)
	if err != nil {
		return telegram.Credentials{}, err
	}
	if settings.AppID <= 0 || settings.AppHash == "" {
		return telegram.Credentials{}, telegram.ErrCredentialsNotConfigured
	}
	return telegram.Credentials{APIID: settings.AppID, APIHash: settings.AppHash}, nil
}

func defaultTelegramAPISettings() model.TelegramAPISettings {
	return model.TelegramAPISettings{
		AppID:   telegram.DefaultAPIID,
		AppHash: telegram.DefaultAPIHash,
	}
}

func isDefaultTelegramAPI(settings model.TelegramAPISettings) bool {
	return settings.AppID == telegram.DefaultAPIID && settings.AppHash == telegram.DefaultAPIHash
}
