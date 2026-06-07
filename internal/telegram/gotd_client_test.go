package telegram

import (
	"context"
	"testing"

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
