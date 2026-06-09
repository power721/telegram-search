package messagefilter

import (
	"context"
	"path/filepath"
	"testing"

	"tg-search/internal/db"
	"tg-search/internal/model"
	"tg-search/internal/repository"
)

func TestSettingsRuleStoreFallsBackToSetupListenRules(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "tg-search.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	settings := repository.NewSettingsRepository(conn)
	if err := settings.Set(ctx, GlobalListenRulesSettingKey, `{
		"includes":["庆余年"],
		"excludes":["预告"],
		"message_types":["link","text"],
		"link_types":["cloud_drive"],
		"ignored_link_patterns":["t.me","*.t.me"]
	}`); err != nil {
		t.Fatalf("save global rules: %v", err)
	}
	store := NewSettingsRuleStore(fakeRules{byChannel: map[int64]model.WatchRule{}}, settings)

	rule, err := store.FindByChannelID(ctx, 9)
	if err != nil {
		t.Fatalf("FindByChannelID returned error: %v", err)
	}
	if !rule.Enabled || rule.ChannelID != 9 || len(rule.Includes) != 1 || rule.Includes[0] != "庆余年" {
		t.Fatalf("rule = %+v, want enabled fallback for channel 9", rule)
	}
	if len(rule.IgnoredLinkPatterns) != 2 || rule.IgnoredLinkPatterns[1] != "*.t.me" {
		t.Fatalf("ignored link patterns = %+v, want global patterns", rule.IgnoredLinkPatterns)
	}
}

func TestSettingsRuleStoreKeepsChannelRulePriorityAndMergesGlobalIgnoredLinks(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "tg-search.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	settings := repository.NewSettingsRepository(conn)
	if err := settings.Set(ctx, GlobalListenRulesSettingKey, `{
		"includes":["全局"],
		"excludes":[],
		"message_types":["link"],
		"link_types":["cloud_drive"],
		"ignored_link_patterns":["t.me","*.t.me"]
	}`); err != nil {
		t.Fatalf("save global rules: %v", err)
	}
	store := NewSettingsRuleStore(fakeRules{byChannel: map[int64]model.WatchRule{
		9: {ChannelID: 9, Enabled: true, Includes: []string{"频道"}},
	}}, settings)

	rule, err := store.FindByChannelID(ctx, 9)
	if err != nil {
		t.Fatalf("FindByChannelID returned error: %v", err)
	}
	if len(rule.Includes) != 1 || rule.Includes[0] != "频道" {
		t.Fatalf("rule = %+v, want channel rule priority", rule)
	}
	if len(rule.IgnoredLinkPatterns) != 2 || rule.IgnoredLinkPatterns[1] != "*.t.me" {
		t.Fatalf("ignored link patterns = %+v, want global patterns merged", rule.IgnoredLinkPatterns)
	}
}

func TestSettingsRuleStoreDefaultsIgnoredLinksForLegacyRules(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "tg-search.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	settings := repository.NewSettingsRepository(conn)
	if err := settings.Set(ctx, GlobalListenRulesSettingKey, `{
		"includes":["全局"],
		"excludes":[],
		"message_types":["link"],
		"link_types":["cloud_drive"]
	}`); err != nil {
		t.Fatalf("save global rules: %v", err)
	}
	store := NewSettingsRuleStore(fakeRules{byChannel: map[int64]model.WatchRule{}}, settings)

	rule, err := store.FindByChannelID(ctx, 9)
	if err != nil {
		t.Fatalf("FindByChannelID returned error: %v", err)
	}
	defaults := DefaultIgnoredLinkPatterns()
	if len(rule.IgnoredLinkPatterns) != len(defaults) || rule.IgnoredLinkPatterns[0] != defaults[0] {
		t.Fatalf("ignored link patterns = %+v, want defaults %+v", rule.IgnoredLinkPatterns, defaults)
	}
}
