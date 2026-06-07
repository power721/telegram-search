package update

import (
	"strings"
	"testing"
	"time"

	"github.com/gotd/td/tg"
)

func TestEventsFromGotdUpdates(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	edit := now.Add(time.Minute)
	updates := &tg.Updates{
		Updates: []tg.UpdateClass{
			&tg.UpdateNewChannelMessage{
				Message: &tg.Message{
					ID:      10,
					PeerID:  &tg.PeerChannel{ChannelID: 200},
					FromID:  &tg.PeerUser{UserID: 88},
					Message: "庆余年",
					Date:    int(now.Unix()),
				},
			},
			&tg.UpdateEditChannelMessage{
				Message: &tg.Message{
					ID:       10,
					PeerID:   &tg.PeerChannel{ChannelID: 200},
					FromID:   &tg.PeerUser{UserID: 88},
					Message:  "三体",
					Date:     int(now.Unix()),
					EditDate: int(edit.Unix()),
				},
			},
			&tg.UpdateDeleteChannelMessages{
				ChannelID: 200,
				Messages:  []int{10, 11},
			},
		},
	}

	events := EventsFromGotdUpdates(1, updates)

	if len(events) != 4 {
		t.Fatalf("len = %d, want 4: %+v", len(events), events)
	}
	if events[0].Type != EventNewMessage || events[0].TelegramChannelID != 200 || events[0].MessageID != 10 || events[0].Text != "庆余年" {
		t.Fatalf("new event = %+v", events[0])
	}
	if events[1].Type != EventEditMessage || events[1].EditDate == nil || !events[1].EditDate.Equal(edit) {
		t.Fatalf("edit event = %+v", events[1])
	}
	if events[2].Type != EventDeleteMessage || events[2].MessageID != 10 {
		t.Fatalf("delete event 1 = %+v", events[2])
	}
	if events[3].Type != EventDeleteMessage || events[3].MessageID != 11 {
		t.Fatalf("delete event 2 = %+v", events[3])
	}
}

func TestEventsFromGotdUpdatesIncludesHiddenAndButtonURLs(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	message := &tg.Message{
		ID:      20,
		PeerID:  &tg.PeerChannel{ChannelID: 200},
		Message: "🔗 链接: 115网盘",
		Date:    int(now.Unix()),
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
	updates := &tg.Updates{
		Updates: []tg.UpdateClass{
			&tg.UpdateNewChannelMessage{Message: message},
		},
	}

	events := EventsFromGotdUpdates(1, updates)

	if len(events) != 1 {
		t.Fatalf("len = %d, want 1: %+v", len(events), events)
	}
	for _, want := range []string{
		"🔗 链接: 115网盘",
		"https://115cdn.com/s/sws61os33xj?password=re39",
		"https://pan.quark.cn/s/hidden",
	} {
		if !strings.Contains(events[0].Text, want) {
			t.Fatalf("event text %q missing %q", events[0].Text, want)
		}
		if !strings.Contains(events[0].RawJSON, want) {
			t.Fatalf("raw json %q missing %q", events[0].RawJSON, want)
		}
	}
}
