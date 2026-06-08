package messagefilter

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"tg-search/internal/model"
)

const GlobalListenRulesSettingKey = "setup.listen_rules"

type SettingsStore interface {
	Get(context.Context, string) (string, bool, error)
}

type SettingsRuleStore struct {
	channelRules RuleStore
	settings     SettingsStore
}

func NewSettingsRuleStore(channelRules RuleStore, settings SettingsStore) *SettingsRuleStore {
	return &SettingsRuleStore{channelRules: channelRules, settings: settings}
}

func (s *SettingsRuleStore) FindByChannelID(ctx context.Context, channelID int64) (model.WatchRule, error) {
	if s == nil {
		return model.WatchRule{}, sql.ErrNoRows
	}
	if s.channelRules != nil {
		rule, err := s.channelRules.FindByChannelID(ctx, channelID)
		if err == nil {
			return rule, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return model.WatchRule{}, err
		}
	}
	if s.settings == nil {
		return model.WatchRule{}, sql.ErrNoRows
	}
	raw, ok, err := s.settings.Get(ctx, GlobalListenRulesSettingKey)
	if err != nil {
		return model.WatchRule{}, err
	}
	if !ok {
		return model.WatchRule{}, sql.ErrNoRows
	}
	var rules model.ListenRules
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return model.WatchRule{}, fmt.Errorf("decode global listen rules: %w", err)
	}
	return model.WatchRule{
		ChannelID:    channelID,
		Enabled:      true,
		Includes:     rules.Includes,
		Excludes:     rules.Excludes,
		MessageTypes: rules.MessageTypes,
		LinkTypes:    rules.LinkTypes,
	}, nil
}
