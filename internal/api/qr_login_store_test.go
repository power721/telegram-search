package api

import (
	"context"
	"testing"
	"time"

	"tg-search/internal/telegram"
)

func TestQRLoginStoreAddFindCancel(t *testing.T) {
	store := NewQRLoginStore(time.Minute)
	session := &storeTestQRSession{
		token: telegram.QRLoginToken{URL: "tg://login?token=one", ExpiresAt: time.Now().UTC().Add(time.Minute)},
	}

	item := store.Add("/tmp/qr.session.json", session)
	if item.LoginID == "" {
		t.Fatal("login id is empty")
	}
	found, ok := store.Find(item.LoginID)
	if !ok || found.Session != session {
		t.Fatalf("Find returned %+v ok=%v, want stored session", found, ok)
	}
	if err := store.Cancel(context.Background(), item.LoginID); err != nil {
		t.Fatalf("Cancel returned error: %v", err)
	}
	if !session.cancelled {
		t.Fatal("session was not canceled")
	}
	if _, ok := store.Find(item.LoginID); ok {
		t.Fatal("Find returned canceled session")
	}
}

func TestQRLoginStoreExpiresOldSessions(t *testing.T) {
	store := NewQRLoginStore(time.Millisecond)
	session := &storeTestQRSession{
		token: telegram.QRLoginToken{URL: "tg://login?token=old", ExpiresAt: time.Now().UTC().Add(time.Minute)},
	}
	item := store.Add("/tmp/old.session.json", session)
	time.Sleep(2 * time.Millisecond)

	if _, ok := store.Find(item.LoginID); ok {
		t.Fatal("Find returned expired session")
	}
	if !session.cancelled {
		t.Fatal("expired session was not canceled")
	}
}

type storeTestQRSession struct {
	token     telegram.QRLoginToken
	cancelled bool
}

func (s *storeTestQRSession) Token() telegram.QRLoginToken {
	return s.token
}

func (s *storeTestQRSession) Poll(context.Context) (telegram.QRLoginPollResult, error) {
	return telegram.QRLoginPollResult{Status: telegram.QRLoginStatusPending, Token: s.token}, nil
}

func (s *storeTestQRSession) Cancel(context.Context) error {
	s.cancelled = true
	return nil
}
