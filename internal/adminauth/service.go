package adminauth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"tg-search/internal/model"
	"tg-search/internal/repository"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

const DefaultSessionTTL = 24 * time.Hour

type Service struct {
	users      *repository.UserRepository
	sessions   *repository.AdminSessionRepository
	sessionTTL time.Duration
	now        func() time.Time
}

func NewService(users *repository.UserRepository, sessions ...*repository.AdminSessionRepository) *Service {
	var sessionRepo *repository.AdminSessionRepository
	if len(sessions) > 0 {
		sessionRepo = sessions[0]
	}
	return &Service{
		users:      users,
		sessions:   sessionRepo,
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

func (s *Service) CreateSession(ctx context.Context, user model.User) (string, error) {
	if s.sessions == nil {
		return "", fmt.Errorf("admin session repository is required")
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
	now := s.now()
	if _, err := s.sessions.PruneExpired(ctx, now); err != nil {
		return "", err
	}
	if err := s.sessions.Create(ctx, model.AdminSession{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: s.sessionExpiresAt(),
		CreatedAt: now,
	}); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) UpdateSession(ctx context.Context, token string, user model.User) error {
	if s.sessions == nil {
		return fmt.Errorf("admin session repository is required")
	}
	if err := s.sessions.UpdateUser(ctx, token, user.ID, s.now()); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	return nil
}

func (s *Service) UserForSession(ctx context.Context, token string) (model.User, bool) {
	if s.sessions == nil {
		return model.User{}, false
	}
	now := s.now()
	user, err := s.sessions.FindUser(ctx, token, now)
	if errors.Is(err, sql.ErrNoRows) {
		_, _ = s.sessions.PruneExpired(ctx, now)
		return model.User{}, false
	}
	if err != nil {
		return model.User{}, false
	}
	return user, true
}

func (s *Service) DeleteSession(ctx context.Context, token string) error {
	if s.sessions == nil {
		return fmt.Errorf("admin session repository is required")
	}
	return s.sessions.Delete(ctx, token)
}

func (s *Service) PruneExpiredSessions(ctx context.Context) (int64, error) {
	if s.sessions == nil {
		return 0, fmt.Errorf("admin session repository is required")
	}
	return s.sessions.PruneExpired(ctx, s.now())
}

type SessionCleanupJob struct {
	Service *Service
}

func (j SessionCleanupJob) Name() string {
	return "admin_session_cleanup"
}

func (j SessionCleanupJob) Run(ctx context.Context) error {
	if j.Service == nil {
		return nil
	}
	_, err := j.Service.PruneExpiredSessions(ctx)
	return err
}

func (s *Service) sessionExpiresAt() time.Time {
	ttl := s.sessionTTL
	if ttl <= 0 {
		ttl = DefaultSessionTTL
	}
	return s.now().Add(ttl)
}
