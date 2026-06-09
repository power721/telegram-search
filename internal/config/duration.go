package config

import (
	"fmt"
	"time"
)

type Duration time.Duration

func (d *Duration) UnmarshalYAML(unmarshal func(any) error) error {
	var raw any
	if err := unmarshal(&raw); err != nil {
		return err
	}
	switch value := raw.(type) {
	case int:
		*d = Duration(time.Duration(value))
		return nil
	case int64:
		*d = Duration(time.Duration(value))
		return nil
	case string:
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("parse duration %q: %w", value, err)
		}
		*d = Duration(parsed)
		return nil
	default:
		return fmt.Errorf("unsupported duration value %T", raw)
	}
}

func (d Duration) MarshalYAML() (any, error) {
	return time.Duration(d).String(), nil
}

func (d Duration) Std() time.Duration {
	return time.Duration(d)
}

func (d Duration) String() string {
	return time.Duration(d).String()
}

func (c *TelegramRateLimitConfig) UnmarshalYAML(unmarshal func(any) error) error {
	type rawConfig TelegramRateLimitConfig
	defaults := defaultConfig().Telegram.RateLimit
	raw := rawConfig(defaults)
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*c = TelegramRateLimitConfig(raw)
	return nil
}
