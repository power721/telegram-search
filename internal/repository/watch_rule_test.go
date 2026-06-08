package repository

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"tg-search/internal/db"
	"tg-search/internal/model"
)

func TestWatchRuleRepositoryCRUDAndChannelLookup(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	accounts := NewAccountRepository(conn)
	channels := NewChannelRepository(conn)
	rules := NewWatchRuleRepository(conn)
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 200, Title: "VIP", Type: model.ChannelTypeChannel})

	id, err := rules.Create(ctx, model.WatchRule{
		ChannelID: channelID,
		Enabled:   true,
		Includes:  []string{" 庆余年 ", "", "1080p"},
		Excludes:  []string{"预告"},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	got, err := rules.FindByChannelID(ctx, channelID)
	if err != nil {
		t.Fatalf("FindByChannelID returned error: %v", err)
	}
	if got.ID != id || got.ChannelID != channelID || !got.Enabled {
		t.Fatalf("rule identity = %+v, want id=%d channel=%d enabled", got, id, channelID)
	}
	if !sameStrings(got.Includes, []string{"庆余年", "1080p"}) || !sameStrings(got.Excludes, []string{"预告"}) {
		t.Fatalf("terms = includes=%q excludes=%q", got.Includes, got.Excludes)
	}

	got.Enabled = false
	got.Includes = []string{"三体"}
	got.Excludes = []string{"花絮", " trailer "}
	if err := rules.Update(ctx, got); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	updated, err := rules.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if updated.Enabled || !sameStrings(updated.Includes, []string{"三体"}) || !sameStrings(updated.Excludes, []string{"花絮", "trailer"}) {
		t.Fatalf("updated rule = %+v", updated)
	}

	all, err := rules.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll returned error: %v", err)
	}
	if len(all) != 1 || all[0].ID != id {
		t.Fatalf("FindAll = %+v, want one rule id %d", all, id)
	}

	if _, err := rules.Create(ctx, model.WatchRule{ChannelID: channelID, Enabled: true}); !errors.Is(err, ErrDuplicateWatchRule) {
		t.Fatalf("duplicate Create err = %v, want ErrDuplicateWatchRule", err)
	}
	if err := rules.Delete(ctx, id); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := rules.FindByID(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("FindByID after delete err = %v, want ErrNotFound", err)
	}
}

func TestWatchRuleRepositoryCascadesWithChannelDelete(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	accounts := NewAccountRepository(conn)
	channels := NewChannelRepository(conn)
	rules := NewWatchRuleRepository(conn)
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 200, Title: "VIP", Type: model.ChannelTypeChannel})
	_, _ = rules.Create(ctx, model.WatchRule{ChannelID: channelID, Enabled: true})

	if _, err := conn.ExecContext(ctx, `DELETE FROM telegram_channels WHERE id = ?`, channelID); err != nil {
		t.Fatalf("delete channel: %v", err)
	}
	all, err := rules.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll returned error: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("rules after channel delete = %+v, want empty", all)
	}
}

func TestWatchRuleMessageAndLinkTypes(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	accounts := NewAccountRepository(conn)
	channels := NewChannelRepository(conn)
	rules := NewWatchRuleRepository(conn)
	accountID, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 201, Title: "VIP", Type: model.ChannelTypeChannel})

	id, err := rules.Create(ctx, model.WatchRule{
		ChannelID:    channelID,
		Enabled:      true,
		Includes:     []string{"电影", "课程"},
		Excludes:     []string{"广告"},
		MessageTypes: []string{" text ", "", "file"},
		LinkTypes:    []string{"cloud_drive", "magnet", "ed2k", "http"},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	got, err := rules.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if !sameStrings(got.MessageTypes, []string{"text", "file"}) {
		t.Fatalf("message_types = %q", got.MessageTypes)
	}
	if !sameStrings(got.LinkTypes, []string{"cloud_drive", "magnet", "ed2k", "http"}) {
		t.Fatalf("link_types = %q", got.LinkTypes)
	}

	got.MessageTypes = []string{"text"}
	got.LinkTypes = []string{"http"}
	if err := rules.Update(ctx, got); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	updated, err := rules.FindByChannelID(ctx, channelID)
	if err != nil {
		t.Fatalf("FindByChannelID returned error: %v", err)
	}
	if !sameStrings(updated.MessageTypes, []string{"text"}) || !sameStrings(updated.LinkTypes, []string{"http"}) {
		t.Fatalf("updated types = message:%q link:%q", updated.MessageTypes, updated.LinkTypes)
	}
}

func sameStrings(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
