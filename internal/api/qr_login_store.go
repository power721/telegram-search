package api

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"tg-search/internal/telegram"
)

type QRLoginStore struct {
	byID map[string]QRLoginStoreItem
}

type QRLoginStoreItem struct {
	LoginID     string
	SessionPath string
	Session     telegram.QRLoginSession
}

func NewQRLoginStore(time.Duration) *QRLoginStore {
	return &QRLoginStore{byID: map[string]QRLoginStoreItem{}}
}

func (s *QRLoginStore) Add(sessionPath string, session telegram.QRLoginSession) QRLoginStoreItem {
	item := QRLoginStoreItem{LoginID: newQRLoginID(), SessionPath: sessionPath, Session: session}
	s.byID[item.LoginID] = item
	return item
}

func (s *QRLoginStore) Find(loginID string) (QRLoginStoreItem, bool) {
	item, ok := s.byID[loginID]
	return item, ok
}

func newQRLoginID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(buf[:])
}
