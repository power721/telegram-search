package telegram

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrUnavailable = errors.New("telegram client is unavailable")
var ErrPasswordRequired = errors.New("telegram password required")

type SentCode struct {
	PhoneCodeHash string
}

type Profile struct {
	TelegramUserID int64
	FirstName      string
	LastName       string
	Username       string
}

type AccountSession struct {
	AccountID   int64
	Phone       string
	SessionPath string
}

type Channel struct {
	TelegramChannelID int64
	AccessHash        int64
	Title             string
	Username          string
	Type              string
	MemberCount       int64
	Description       string
	AvatarState       string
}

type ChannelRef struct {
	TelegramChannelID int64
	AccessHash        int64
	Type              string
}

type Message struct {
	TelegramMessageID int64
	SenderID          int64
	Text              string
	RawJSON           string
	Date              time.Time
	EditDate          *time.Time
}

type Client interface {
	SendCode(ctx context.Context, phone string, sessionPath string) (SentCode, error)
	SignIn(ctx context.Context, phone string, code string, phoneCodeHash string, sessionPath string) (Profile, error)
	Password(ctx context.Context, password string, sessionPath string) (Profile, error)
	ListChannels(ctx context.Context, session AccountSession) ([]Channel, error)
	FetchHistory(ctx context.Context, session AccountSession, channel ChannelRef, offsetID int64, limit int) ([]Message, error)
	SearchMessages(ctx context.Context, session AccountSession, channel ChannelRef, query string, limit int) ([]Message, error)
}

type CodeStore struct {
	mu     sync.Mutex
	hashes map[string]string
}

func NewCodeStore() *CodeStore {
	return &CodeStore{hashes: map[string]string{}}
}

func (s *CodeStore) Save(phone string, hash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hashes[phone] = hash
}

func (s *CodeStore) Take(phone string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	hash, ok := s.hashes[phone]
	if ok {
		delete(s.hashes, phone)
	}
	return hash, ok
}

type NopClient struct{}

func (NopClient) SendCode(context.Context, string, string) (SentCode, error) {
	return SentCode{}, ErrUnavailable
}

func (NopClient) SignIn(context.Context, string, string, string, string) (Profile, error) {
	return Profile{}, ErrUnavailable
}

func (NopClient) Password(context.Context, string, string) (Profile, error) {
	return Profile{}, ErrUnavailable
}

func (NopClient) ListChannels(context.Context, AccountSession) ([]Channel, error) {
	return nil, ErrUnavailable
}

func (NopClient) FetchHistory(context.Context, AccountSession, ChannelRef, int64, int) ([]Message, error) {
	return nil, ErrUnavailable
}

func (NopClient) SearchMessages(context.Context, AccountSession, ChannelRef, string, int) ([]Message, error) {
	return nil, ErrUnavailable
}
