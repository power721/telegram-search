package model

import "time"

const (
	AccountStatusNew           = "NEW"
	AccountStatusLoginRequired = "LOGIN_REQUIRED"
	AccountStatusSyncing       = "SYNCING"
	AccountStatusOnline        = "ONLINE"
	AccountStatusReconnecting  = "RECONNECTING"
	AccountStatusFloodWait     = "FLOOD_WAIT"
	AccountStatusDisconnected  = "DISCONNECTED"
)

const (
	ChannelTypeChannel       = "channel"
	ChannelTypeSupergroup    = "supergroup"
	ChannelTypeSavedMessages = "saved_messages"
)

type Account struct {
	ID             int64     `json:"id"`
	Phone          string    `json:"phone"`
	TelegramUserID int64     `json:"telegram_user_id"`
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name"`
	Username       string    `json:"username"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Channel struct {
	ID                 int64      `json:"id"`
	AccountID          int64      `json:"account_id"`
	TelegramChannelID  int64      `json:"telegram_channel_id"`
	AccessHash         int64      `json:"access_hash"`
	Title              string     `json:"title"`
	Username           string     `json:"username"`
	Type               string     `json:"type"`
	LastMessageID      int64      `json:"last_message_id"`
	LastSyncTime       *time.Time `json:"last_sync_time,omitempty"`
	WebAccess          *bool      `json:"web_access,omitempty"`
	WebAccessCheckedAt *time.Time `json:"web_access_checked_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type WatchRule struct {
	ID        int64     `json:"id"`
	ChannelID int64     `json:"channel_id"`
	Enabled   bool      `json:"enabled"`
	Includes  []string  `json:"includes"`
	Excludes  []string  `json:"excludes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Message struct {
	ID                int64      `json:"id"`
	AccountID         int64      `json:"account_id"`
	ChannelID         int64      `json:"channel_id"`
	TelegramMessageID int64      `json:"telegram_message_id"`
	SenderID          int64      `json:"sender_id"`
	Text              string     `json:"text"`
	RawJSON           string     `json:"raw_json"`
	Date              time.Time  `json:"date"`
	EditDate          *time.Time `json:"edit_date,omitempty"`
	Deleted           bool       `json:"deleted"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`

	OriginalLinkInputs []Link `json:"-"`
}

type Link struct {
	ID        int64     `json:"id"`
	MessageID int64     `json:"message_id"`
	Type      string    `json:"type"`
	URL       string    `json:"url"`
	Password  string    `json:"password,omitempty"`
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type SearchResult struct {
	Message
	AccountPhone     string `json:"account_phone"`
	AccountUsername  string `json:"account_username"`
	AccountFirstName string `json:"account_first_name"`
	ChannelTitle     string `json:"channel_title"`
	ChannelUsername  string `json:"channel_username"`
	Links            []Link `json:"links"`
}

type LinkResult struct {
	Link
	MessageText       string    `json:"message_text"`
	MessageDate       time.Time `json:"message_date"`
	AccountID         int64     `json:"account_id"`
	ChannelID         int64     `json:"channel_id"`
	ChannelTitle      string    `json:"channel_title"`
	TelegramMessageID int64     `json:"telegram_message_id"`
}

type MergedLink struct {
	URL               string    `json:"url"`
	Password          string    `json:"password,omitempty"`
	Note              string    `json:"note,omitempty"`
	Datetime          time.Time `json:"datetime"`
	Source            string    `json:"source,omitempty"`
	ChannelID         int64     `json:"channel_id"`
	TelegramMessageID int64     `json:"telegram_message_id"`
}

type MergedLinks map[string][]MergedLink

type MergedLinksResponse struct {
	Total        int         `json:"total"`
	MergedByType MergedLinks `json:"merged_by_type"`
}

type StatusCounts struct {
	Accounts      int64            `json:"accounts"`
	Channels      int64            `json:"channels"`
	Messages      int64            `json:"messages"`
	Links         int64            `json:"links"`
	AccountStates map[string]int64 `json:"account_states"`
}
