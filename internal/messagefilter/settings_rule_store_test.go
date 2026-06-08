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
		"link_types":["cloud_drive"]
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
}

func TestSettingsRuleStoreKeepsChannelRulePriority(t *testing.T) {
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
}
