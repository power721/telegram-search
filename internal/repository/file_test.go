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

func TestFileRepositoryStoresDuplicateTelegramFilesAcrossMessages(t *testing.T) {
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
	mirrorChannelID, err := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 2, Title: "Mirror", Type: model.ChannelTypeChannel})
	if err != nil {
		t.Fatalf("save mirror channel: %v", err)
	}
	stored, err := messages.SaveBatch(ctx, []model.Message{
		{
			AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1,
			Text: "archive one", RawJSON: "{}", Date: time.Now().UTC(),
		},
		{
			AccountID: accountID, ChannelID: mirrorChannelID, TelegramMessageID: 2,
			Text: "archive mirror", RawJSON: "{}", Date: time.Now().UTC(),
		},
	})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}

	first, err := files.SaveBatch(ctx, stored[0].ID, []model.File{{
		TelegramFileID: 42,
		FileName:       "release-pack.zip",
		Extension:      ".zip",
		MimeType:       "application/zip",
		SizeBytes:      1024,
		Category:       "archive",
	}})
	if err != nil {
		t.Fatalf("save first file: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("first saved files = %+v, want one stored file", first)
	}
	duplicate, err := files.SaveBatch(ctx, stored[1].ID, []model.File{{
		TelegramFileID: 42,
		FileName:       "renamed-release-pack.zip",
		Extension:      ".zip",
		MimeType:       "application/zip",
		SizeBytes:      1024,
		Category:       "archive",
	}})
	if err != nil {
		t.Fatalf("save duplicate file: %v", err)
	}
	if len(duplicate) != 1 || duplicate[0].MessageID != stored[1].ID || duplicate[0].TelegramFileID != 42 {
		t.Fatalf("duplicate saved files = %+v, want one stored file for second message", duplicate)
	}

	foundFirst, err := files.FindByMessageID(ctx, stored[0].ID)
	if err != nil {
		t.Fatalf("find first files: %v", err)
	}
	if len(foundFirst) != 1 {
		t.Fatalf("first message files len = %d, want 1", len(foundFirst))
	}
	foundDuplicate, err := files.FindByMessageID(ctx, stored[1].ID)
	if err != nil {
		t.Fatalf("find duplicate files: %v", err)
	}
	if len(foundDuplicate) != 1 {
		t.Fatalf("duplicate message files = %+v, want one", foundDuplicate)
	}
	if len(foundDuplicate) != 1 || foundDuplicate[0].TelegramFileID != 42 {
		t.Fatalf("duplicate message files = %+v, want telegram file 42", foundDuplicate)
	}
}

func TestFileRepositorySkipsDuplicateFilesByNameAndSizeWithoutTelegramID(t *testing.T) {
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
	stored, err := messages.SaveBatch(ctx, []model.Message{
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1, Text: "legacy one", RawJSON: "{}", Date: time.Now().UTC()},
		{AccountID: accountID, ChannelID: channelID, TelegramMessageID: 2, Text: "legacy two", RawJSON: "{}", Date: time.Now().UTC()},
	})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	if _, err := files.SaveBatch(ctx, stored[0].ID, []model.File{{
		FileName: "legacy.iso", Extension: ".iso", MimeType: "application/x-iso9660-image", SizeBytes: 4096,
	}}); err != nil {
		t.Fatalf("save first legacy file: %v", err)
	}
	duplicate, err := files.SaveBatch(ctx, stored[1].ID, []model.File{{
		FileName: "legacy.iso", Extension: ".iso", MimeType: "application/x-iso9660-image", SizeBytes: 4096,
	}})
	if err != nil {
		t.Fatalf("save duplicate legacy file: %v", err)
	}
	if len(duplicate) != 0 {
		t.Fatalf("duplicate legacy files = %+v, want skipped", duplicate)
	}
}
