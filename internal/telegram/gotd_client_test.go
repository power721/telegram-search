package telegram

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gotd/td/telegram/query/dialogs"
	"github.com/gotd/td/tg"
)

func TestListDialogChannelsCollectsAllDialogPages(t *testing.T) {
	ctx := context.Background()
	pages := [][]tg.DialogClass{
		{testDialog(1001), testDialog(1002)},
		{testDialog(2001)},
		nil,
	}
	calls := 0

	query := dialogs.QueryFunc(func(context.Context, dialogs.Request) (tg.MessagesDialogsClass, error) {
		if calls >= len(pages) {
			t.Fatalf("unexpected dialogs request %d", calls+1)
		}
		page := pages[calls]
		calls++
		return testDialogsResult(page, 3), nil
	})
	iter := dialogs.NewIterator(query, 2)
	channels, err := listDialogChannels(ctx, iter)
	if err != nil {
		t.Fatalf("listDialogChannels returned error: %v", err)
	}

	if calls != 3 {
		t.Fatalf("dialogs requests = %d, want 3 pages", calls)
	}
	if len(channels) != 3 {
		t.Fatalf("channels length = %d, want 3: %+v", len(channels), channels)
	}
	if channels[2].TelegramChannelID != 2001 {
		t.Fatalf("last channel id = %d, want second page channel 2001", channels[2].TelegramChannelID)
	}
	if channels[2].Username != "" {
		t.Fatalf("private channel username = %q, want empty", channels[2].Username)
	}
}

func TestApplyFullChannelMetadata(t *testing.T) {
	channel := Channel{
		TelegramChannelID: 1001,
		Title:             "Private Channel",
		MemberCount:       12,
		Description:       "dialog description",
	}
	full := &tg.ChannelFull{
		ID:    1001,
		About: "full description",
	}
	full.SetParticipantsCount(1234)

	got := applyFullChannelMetadata(channel, full)

	if got.Description != "full description" {
		t.Fatalf("description = %q, want full description", got.Description)
	}
	if got.MemberCount != 1234 {
		t.Fatalf("member_count = %d, want 1234", got.MemberCount)
	}
}

func testDialog(channelID int64) tg.DialogClass {
	return &tg.Dialog{Peer: &tg.PeerChannel{ChannelID: channelID}}
}

func testDialogsResult(items []tg.DialogClass, count int) tg.MessagesDialogsClass {
	messages := make([]tg.MessageClass, 0, len(items))
	chats := make([]tg.ChatClass, 0, len(items))
	for i, item := range items {
		channelID := item.GetPeer().(*tg.PeerChannel).ChannelID
		messages = append(messages, &tg.Message{
			ID:     i + 1,
			Date:   1000 - i,
			PeerID: item.GetPeer(),
		})
		chats = append(chats, &tg.Channel{
			ID:         channelID,
			AccessHash: channelID + 5000,
			Photo:      &tg.ChatPhoto{PhotoID: channelID + 9000},
			Title:      "Private Channel",
		})
	}
	return &tg.MessagesDialogsSlice{
		Dialogs:  items,
		Messages: messages,
		Chats:    chats,
		Count:    count,
	}
}

func TestConvertMessageIncludesHiddenAndButtonURLs(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	message := &tg.Message{
		ID:      10,
		Date:    int(now.Unix()),
		PeerID:  &tg.PeerChannel{ChannelID: 200},
		Message: "🔗 链接: 115网盘",
	}
	message.SetEntities([]tg.MessageEntityClass{
		&tg.MessageEntityTextURL{
			Offset: 4,
			Length: 5,
			URL:    "https://115cdn.com/s/sws61os33xj?password=re39",
		},
	})
	message.SetReplyMarkup(&tg.ReplyInlineMarkup{
		Rows: []tg.KeyboardButtonRow{{
			Buttons: []tg.KeyboardButtonClass{
				&tg.KeyboardButtonURL{Text: "打开", URL: "https://pan.quark.cn/s/hidden"},
			},
		}},
	})

	converted := convertMessage(message)

	for _, want := range []string{
		"🔗 链接: 115网盘",
		"https://115cdn.com/s/sws61os33xj?password=re39",
		"https://pan.quark.cn/s/hidden",
	} {
		if !strings.Contains(converted.Text, want) {
			t.Fatalf("converted text %q missing %q", converted.Text, want)
		}
		if !strings.Contains(converted.RawJSON, want) {
			t.Fatalf("raw json %q missing %q", converted.RawJSON, want)
		}
	}
}
