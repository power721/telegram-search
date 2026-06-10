package adminauth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"tg-search/internal/model"
	"tg-search/internal/repository"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

const DefaultSessionTTL = 24 * time.Hour

type Service struct {
	users      *repository.UserRepository
	mu         sync.Mutex
	sessions   map[string]sessionEntry
	sessionTTL time.Duration
	now        func() time.Time
}

type sessionEntry struct {
	user      model.User
	expiresAt time.Time
}

func NewService(users *repository.UserRepository) *Service {
	return &Service{
		users:      users,
		sessions:   map[string]sessionEntry{},
		sessionTTL: DefaultSessionTTL,
		now:        func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) CreateAdmin(ctx context.Context, username string, password string) (int64, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return 0, fmt.Errorf("username is required")
	}
	if len(password) < 8 {
		return 0, fmt.Errorf("password must be at least 8 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, fmt.Errorf("hash password: %w", err)
	}
	return s.users.Create(ctx, model.User{
		Username:     username,
		PasswordHash: string(hash),
		Role:         model.UserRoleAdmin,
	})
}

func (s *Service) UpdateCredentials(ctx context.Context, userID int64, username string, currentPassword string, newPassword string) (model.User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return model.User{}, fmt.Errorf("username is required")
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return model.User{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return model.User{}, ErrInvalidCredentials
	}
	passwordHash := user.PasswordHash
	if newPassword != "" {
		if len(newPassword) < 8 {
			return model.User{}, fmt.Errorf("password must be at least 8 characters")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			return model.User{}, fmt.Errorf("hash password: %w", err)
		}
		passwordHash = string(hash)
	}
	return s.users.UpdateCredentials(ctx, userID, username, passwordHash)
}

func (s *Service) Authenticate(ctx context.Context, username string, password string) (model.User, error) {
	user, err := s.users.FindByUsername(ctx, strings.TrimSpace(username))
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return model.User{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return model.User{}, ErrInvalidCredentials
	}
	return user, nil
}

func (s *Service) CreateSession(user model.User) (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepExpiredLocked()
	s.sessions[token] = sessionEntry{user: user, expiresAt: s.sessionExpiresAt()}
	return token, nil
}

func (s *Service) UpdateSession(token string, user model.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.sessions[token]
	if !ok {
		return
	}
	if s.expired(entry) {
		delete(s.sessions, token)
		return
	}
	entry.user = user
	s.sessions[token] = entry
}

func (s *Service) UserForSession(token string) (model.User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.sessions[token]
	if !ok {
		return model.User{}, false
	}
	if s.expired(entry) {
		delete(s.sessions, token)
		return model.User{}, false
	}
	return entry.user, true
}

func (s *Service) DeleteSession(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

func (s *Service) sessionExpiresAt() time.Time {
	ttl := s.sessionTTL
	if ttl <= 0 {
		ttl = DefaultSessionTTL
	}
	return s.now().Add(ttl)
}

func (s *Service) expired(entry sessionEntry) bool {
	return !entry.expiresAt.IsZero() && !s.now().Before(entry.expiresAt)
}

func (s *Service) sweepExpiredLocked() {
	for token, entry := range s.sessions {
		if s.expired(entry) {
			delete(s.sessions, token)
		}
	}
}
