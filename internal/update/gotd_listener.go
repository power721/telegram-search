package update

import (
	"context"
	"encoding/json"
	"runtime"
	"time"

	gotdsession "github.com/gotd/td/session"
	gotdtelegram "github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"

	"tg-search/internal/build"
	"tg-search/internal/model"
	localsession "tg-search/internal/session"
	"tg-search/internal/telegram"
)

type GotdListener struct {
	credentials telegram.CredentialsProvider
	sessions    *localsession.Manager
	logger      *zap.Logger
}

func NewGotdListener(credentials telegram.CredentialsProvider, sessions *localsession.Manager, logger *zap.Logger) *GotdListener {
	if logger == nil {
		logger = zap.NewNop()
	}
	if credentials == nil {
		credentials = telegram.StaticCredentialsProvider{}
	}
	return &GotdListener{credentials: credentials, sessions: sessions, logger: logger}
}

func (l *GotdListener) Run(ctx context.Context, account model.Account, emit func(Event) error) error {
	sessionPath := ""
	if l.sessions != nil {
		sessionPath = l.sessions.PathForAccount(account.ID)
	}
	handler := gotdtelegram.UpdateHandlerFunc(func(ctx context.Context, updates tg.UpdatesClass) error {
		for _, event := range EventsFromGotdUpdates(account.ID, updates) {
			if err := emit(event); err != nil {
				return err
			}
		}
		return nil
	})
	manager := updates.New(updates.Config{
		Handler: handler,
		Logger:  l.logger.Named("updates"),
	})
	credentials, err := l.credentials.TelegramCredentials(ctx)
	if err != nil {
		return err
	}
	client := gotdtelegram.NewClient(credentials.APIID, credentials.APIHash, gotdtelegram.Options{
		SessionStorage: &gotdsession.FileStorage{Path: sessionPath},
		Logger:         l.logger,
		UpdateHandler:  manager,
		Device: gotdtelegram.DeviceConfig{
			DeviceModel:    "TG Search",
			SystemVersion:  runtime.GOOS,
			AppVersion:     build.Version,
			SystemLangCode: "zh-CN",
			LangCode:       "zh",
		},
	})
	return client.Run(ctx, func(ctx context.Context) error {
		self, err := client.Self(ctx)
		if err != nil {
			return err
		}
		return manager.Run(ctx, client.API(), self.ID, updates.AuthOptions{})
	})
}

func EventsFromGotdUpdates(accountID int64, updates tg.UpdatesClass) []Event {
	var out []Event
	for _, item := range updateItems(updates) {
		switch u := item.(type) {
		case *tg.UpdateNewChannelMessage:
			if event, ok := eventFromMessage(accountID, EventNewMessage, u.Message); ok {
				out = append(out, event)
			}
		case *tg.UpdateEditChannelMessage:
			if event, ok := eventFromMessage(accountID, EventEditMessage, u.Message); ok {
				out = append(out, event)
			}
		case *tg.UpdateDeleteChannelMessages:
			for _, id := range u.Messages {
				out = append(out, Event{
					Type:              EventDeleteMessage,
					AccountID:         accountID,
					TelegramChannelID: u.ChannelID,
					MessageID:         int64(id),
				})
			}
		case *tg.UpdateNewMessage:
			if event, ok := eventFromMessage(accountID, EventNewMessage, u.Message); ok {
				out = append(out, event)
			}
		case *tg.UpdateEditMessage:
			if event, ok := eventFromMessage(accountID, EventEditMessage, u.Message); ok {
				out = append(out, event)
			}
		}
	}
	return out
}

func updateItems(updates tg.UpdatesClass) []tg.UpdateClass {
	switch u := updates.(type) {
	case *tg.Updates:
		return u.Updates
	case *tg.UpdatesCombined:
		return u.Updates
	case *tg.UpdateShort:
		return []tg.UpdateClass{u.Update}
	default:
		return nil
	}
}

func eventFromMessage(accountID int64, typ EventType, item tg.MessageClass) (Event, bool) {
	message, ok := item.(*tg.Message)
	if !ok {
		return Event{}, false
	}
	channelID := peerChannelID(message.PeerID)
	if channelID == 0 {
		return Event{}, false
	}
	var editDate *time.Time
	if message.EditDate > 0 {
		t := time.Unix(int64(message.EditDate), 0).UTC()
		editDate = &t
	}
	indexedText, messageURLs := telegram.IndexedMessageText(message)
	messageType, mediaSummary := telegram.MessageMediaMetadata(message)
	rawJSON, _ := json.Marshal(map[string]any{
		"id":           message.ID,
		"date":         message.Date,
		"message":      message.Message,
		"message_urls": messageURLs,
	})
	return Event{
		Type:              typ,
		AccountID:         accountID,
		TelegramChannelID: channelID,
		MessageID:         int64(message.ID),
		SenderID:          peerID(message.FromID),
		MessageType:       messageType,
		MediaSummary:      mediaSummary,
		Text:              indexedText,
		RawJSON:           string(rawJSON),
		Date:              time.Unix(int64(message.Date), 0).UTC(),
		EditDate:          editDate,
		Files:             telegram.FilesFromMessage(message),
	}, true
}

func peerChannelID(peer tg.PeerClass) int64 {
	switch p := peer.(type) {
	case *tg.PeerChannel:
		return p.ChannelID
	default:
		return 0
	}
}

func peerID(peer tg.PeerClass) int64 {
	switch p := peer.(type) {
	case *tg.PeerUser:
		return p.UserID
	case *tg.PeerChat:
		return p.ChatID
	case *tg.PeerChannel:
		return p.ChannelID
	default:
		return 0
	}
}
