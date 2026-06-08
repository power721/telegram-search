package update

import (
	"time"

	"tg-search/internal/model"
)

type EventType string

const (
	EventNewMessage    EventType = "new_message"
	EventEditMessage   EventType = "edit_message"
	EventDeleteMessage EventType = "delete_message"
)

type Event struct {
	Type              EventType
	AccountID         int64
	TelegramChannelID int64
	MessageID         int64
	SenderID          int64
	Text              string
	RawJSON           string
	Date              time.Time
	EditDate          *time.Time
	Files             []model.File
}
