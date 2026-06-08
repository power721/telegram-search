package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"tg-search/internal/telegram"
)

type QRLoginStore struct {
	mu   sync.Mutex
	ttl  time.Duration
	now  func() time.Time
	byID map[string]QRLoginStoreItem
}

type QRLoginStoreItem struct {
	LoginID     string
	SessionPath string
	Session     telegram.QRLoginSession
	CreatedAt   time.Time
}

func NewQRLoginStore(ttl time.Duration) *QRLoginStore {
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	return &QRLoginStore{
		ttl:  ttl,
		now:  time.Now,
		byID: map[string]QRLoginStoreItem{},
	}
}

func (s *QRLoginStore) Add(sessionPath string, session telegram.QRLoginSession) QRLoginStoreItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := QRLoginStoreItem{
		LoginID:     newQRLoginID(),
		SessionPath: sessionPath,
		Session:     session,
		CreatedAt:   s.now().UTC(),
	}
	s.byID[item.LoginID] = item
	return item
}

func (s *QRLoginStore) Find(loginID string) (QRLoginStoreItem, bool) {
	s.mu.Lock()
	item, ok := s.byID[loginID]
	if !ok {
		s.mu.Unlock()
		return QRLoginStoreItem{}, false
	}
	if s.now().Sub(item.CreatedAt) <= s.ttl {
		s.mu.Unlock()
		return item, true
	}
	delete(s.byID, loginID)
	s.mu.Unlock()
	_ = item.Session.Cancel(context.Background())
	return QRLoginStoreItem{}, false
}

func (s *QRLoginStore) Remove(loginID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.byID, loginID)
}

func (s *QRLoginStore) Cancel(ctx context.Context, loginID string) error {
	s.mu.Lock()
	item, ok := s.byID[loginID]
	if ok {
		delete(s.byID, loginID)
	}
	s.mu.Unlock()
	if !ok {
		return nil
	}
	return item.Session.Cancel(ctx)
}

func newQRLoginID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(buf[:])
}
