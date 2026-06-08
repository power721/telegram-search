package repository

import (
	"context"
	"testing"
	"time"

	"tg-search/internal/model"
)

func TestFileRepositoryPersistsFileMetadata(t *testing.T) {
	ctx := context.Background()
	conn := openRepositoryTestDB(t)
	accounts := NewAccountRepository(conn)
	channels := NewChannelRepository(conn)
	messages := NewMessageRepository(conn)
	files := NewFileRepository(conn)

	accountID, err := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	if err != nil {
		t.Fatalf("save account: %v", err)
	}
	channelID, err := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	if err != nil {
		t.Fatalf("save channel: %v", err)
	}
	stored, err := messages.SaveBatch(ctx, []model.Message{{
		AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1,
		Text: "Ubuntu ISO", RawJSON: "{}", Date: time.Now().UTC(),
	}})
	if err != nil {
		t.Fatalf("save message: %v", err)
	}

	saved, err := files.SaveBatch(ctx, stored[0].ID, []model.File{{
		FileName:  "ubuntu-26.04.iso",
		Extension: ".iso",
		MimeType:  "application/x-iso9660-image",
		SizeBytes: 5_000_000_000,
		Category:  "software",
	}})
	if err != nil {
		t.Fatalf("save files: %v", err)
	}
	if len(saved) != 1 || saved[0].ID == 0 || saved[0].MessageID != stored[0].ID {
		t.Fatalf("saved files = %+v", saved)
	}

	updated, err := files.SaveBatch(ctx, stored[0].ID, []model.File{{
		FileName:  "ubuntu-26.04.iso",
		Extension: ".iso",
		MimeType:  "application/octet-stream",
		SizeBytes: 5_000_000_000,
		Category:  "software",
	}})
	if err != nil {
		t.Fatalf("update files: %v", err)
	}
	if len(updated) != 1 || updated[0].ID != saved[0].ID {
		t.Fatalf("updated files = %+v, want same id %d", updated, saved[0].ID)
	}

	found, err := files.FindByMessageID(ctx, stored[0].ID)
	if err != nil {
		t.Fatalf("find files: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("found len = %d, want 1", len(found))
	}
	if found[0].FileName != "ubuntu-26.04.iso" || found[0].Extension != ".iso" ||
		found[0].MimeType != "application/octet-stream" || found[0].SizeBytes != 5_000_000_000 ||
		found[0].Category != "software" {
		t.Fatalf("found file = %+v", found[0])
	}
}
