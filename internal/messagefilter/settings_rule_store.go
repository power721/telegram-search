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
	globalRules, hasGlobalRules, err := s.globalListenRules(ctx)
	if err != nil {
		return model.WatchRule{}, err
	}
	globalIgnored := DefaultIgnoredLinkPatterns()
	if hasGlobalRules {
		globalIgnored = ignoredPatternsOrDefault(globalRules.IgnoredLinkPatterns)
	}
	if s.channelRules != nil {
		rule, err := s.channelRules.FindByChannelID(ctx, channelID)
		if err == nil {
			rule.IgnoredLinkPatterns = append([]string(nil), globalIgnored...)
			return rule, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return model.WatchRule{}, err
		}
	}
	if !hasGlobalRules {
		return model.WatchRule{}, sql.ErrNoRows
	}
	return model.WatchRule{
		ChannelID:           channelID,
		Enabled:             true,
		Includes:            globalRules.Includes,
		Excludes:            globalRules.Excludes,
		MessageTypes:        globalRules.MessageTypes,
		LinkTypes:           globalRules.LinkTypes,
		IgnoredLinkPatterns: append([]string(nil), globalIgnored...),
	}, nil
}

func (s *SettingsRuleStore) globalListenRules(ctx context.Context) (model.ListenRules, bool, error) {
	if s == nil || s.settings == nil {
		return model.ListenRules{}, false, nil
	}
	raw, ok, err := s.settings.Get(ctx, GlobalListenRulesSettingKey)
	if err != nil {
		return model.ListenRules{}, false, err
	}
	if !ok {
		return model.ListenRules{}, false, nil
	}
	var rules model.ListenRules
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return model.ListenRules{}, false, fmt.Errorf("decode global listen rules: %w", err)
	}
	if rules.IgnoredLinkPatterns == nil {
		rules.IgnoredLinkPatterns = DefaultIgnoredLinkPatterns()
	}
	return rules, true, nil
}
