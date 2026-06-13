package ai

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"tg-search/internal/config"
	"tg-search/internal/db"
	"tg-search/internal/model"
	"tg-search/internal/repository"
	"tg-search/internal/resource"
	taskpkg "tg-search/internal/task"
)

func TestServiceEnhancesMultipleLinksInOneMessage(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	accounts := repository.NewAccountRepository(conn)
	channels := repository.NewChannelRepository(conn)
	messages := repository.NewMessageRepository(conn)
	links := repository.NewLinkRepository(conn)
	files := repository.NewFileRepository(conn)
	stats := repository.NewResourceStatsRepository(conn)
	settings := repository.NewSettingsRepository(conn)

	accountID, err := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	if err != nil {
		t.Fatalf("save account: %v", err)
	}
	channelID, err := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 100, Title: "Movies", Type: model.ChannelTypeChannel})
	if err != nil {
		t.Fatalf("save channel: %v", err)
	}
	stored, err := messages.SaveBatch(ctx, []model.Message{{
		AccountID:         accountID,
		ChannelID:         channelID,
		TelegramMessageID: 1,
		MessageType:       "text",
		MediaSummary:      "plain",
		Text:              "迷墙 https://pan.quark.cn/s/a\n另一部 https://www.alipan.com/s/b",
		RawJSON:           `{"id":1}`,
		Date:              time.Now().UTC(),
	}})
	if err != nil {
		t.Fatalf("save message: %v", err)
	}
	savedLinks, err := links.SaveBatch(ctx, stored[0].ID, []model.Link{
		{Type: "quark", URL: "https://pan.quark.cn/s/a", Category: "cloud_drive", MediaTitle: "Rule A", MediaEpisode: "E01", MediaQuality: "1080p"},
		{Type: "aliyun", URL: "https://www.alipan.com/s/b", Category: "cloud_drive", MediaTitle: "Rule B"},
	})
	if err != nil {
		t.Fatalf("save links: %v", err)
	}
	runtime := config.RuntimeSettingsFromConfig(config.Config{})
	runtime.AI.MediaMetadata = config.AIMediaMetadataSettings{
		Enabled: true,
		BaseURL: "https://api.example.com/v1",
		APIKey:  "secret",
		Model:   "media-model",
	}
	if err := settings.SaveRuntimeSettings(ctx, runtime); err != nil {
		t.Fatalf("save runtime settings: %v", err)
	}

	fake := &fakeEnhancer{
		response: EnhancementResponse{Items: []EnhancementItem{
			{
				LinkID: savedLinks[0].ID,
				Media:  MediaMetadata{Title: "AI A", Year: "2026"},
			},
			{
				URL:   savedLinks[1].URL,
				Media: MediaMetadata{Title: "AI B", Episode: "更新08集", Quality: "4K"},
			},
			{
				LinkID: 999999,
				Media:  MediaMetadata{Title: "Unknown"},
			},
		}},
	}
	service := NewService(ServiceOptions{
		Settings: settings,
		Defaults: config.Config{},
		Messages: messages,
		Links:    links,
		Resources: resource.NewService(
			links,
			files,
			stats,
		),
		NewEnhancer: func(config.AIMediaMetadataSettings) Enhancer {
			return fake
		},
	})
	payload, err := json.Marshal(taskpkg.AIMediaMetadataPayload{MessageID: stored[0].ID})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	err = service.RunMediaMetadataTask(ctx, model.Task{
		ID:          1,
		Type:        model.TaskTypeAIMediaMetadata,
		PayloadJSON: string(payload),
	}, noopProgress{})
	if err != nil {
		t.Fatalf("RunMediaMetadataTask: %v", err)
	}

	if len(fake.requests) != 1 {
		t.Fatalf("enhancer calls = %d, want 1", len(fake.requests))
	}
	if fake.requests[0].Message.Text != stored[0].Text || fake.requests[0].Message.RawJSON != `{"id":1}` || len(fake.requests[0].Links) != 2 {
		t.Fatalf("enhancement request = %+v", fake.requests[0])
	}
	updated, err := links.ListByMessage(ctx, stored[0].ID)
	if err != nil {
		t.Fatalf("list updated links: %v", err)
	}
	if updated[0].MediaTitle != "AI A" || updated[0].MediaYear != "2026" || updated[0].MediaEpisode != "E01" || updated[0].MediaQuality != "1080p" {
		t.Fatalf("first link metadata = %+v", updated[0])
	}
	if updated[1].MediaTitle != "AI B" || updated[1].MediaEpisode != "更新08集" || updated[1].MediaQuality != "4K" {
		t.Fatalf("second link metadata = %+v", updated[1])
	}
}

type fakeEnhancer struct {
	requests []EnhancementRequest
	response EnhancementResponse
	err      error
}

func (f *fakeEnhancer) Enhance(ctx context.Context, req EnhancementRequest) (EnhancementResponse, error) {
	f.requests = append(f.requests, req)
	return f.response, f.err
}

type noopProgress struct{}

func (noopProgress) Progress(ctx context.Context, progress int64, total int64, message string) error {
	return nil
}

func (noopProgress) Status(ctx context.Context) (string, error) {
	return model.TaskStatusRunning, nil
}
