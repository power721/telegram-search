package adminauth

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"tg-search/internal/db"
	"tg-search/internal/model"
	"tg-search/internal/repository"
)

func TestServiceCreatesAdminAndAuthenticates(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "tg-search.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	users := repository.NewUserRepository(conn)
	service := NewService(users)
	if _, err := service.CreateAdmin(ctx, "admin", "secret123"); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	user, err := service.Authenticate(ctx, "admin", "secret123")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if user.Username != "admin" || user.Role != model.UserRoleAdmin {
		t.Fatalf("user = %+v", user)
	}
	if _, err := service.Authenticate(ctx, "admin", "wrong"); err == nil {
		t.Fatal("Authenticate with wrong password returned nil error")
	}
	updated, err := service.UpdateCredentials(ctx, user.ID, "root", "secret123", "newsecret123")
	if err != nil {
		t.Fatalf("update credentials: %v", err)
	}
	if updated.Username != "root" || updated.PasswordHash == user.PasswordHash {
		t.Fatalf("updated user = %+v", updated)
	}
	if _, err := service.Authenticate(ctx, "admin", "secret123"); err == nil {
		t.Fatal("Authenticate with old credentials returned nil error")
	}
	if _, err := service.Authenticate(ctx, "root", "newsecret123"); err != nil {
		t.Fatalf("authenticate updated credentials: %v", err)
	}
	if _, err := service.UpdateCredentials(ctx, user.ID, "root", "wrong", "anothersecret"); err != ErrInvalidCredentials {
		t.Fatalf("update with wrong password error = %v, want ErrInvalidCredentials", err)
	}
}

func TestServiceExpiresSessionsServerSide(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	service := NewService(nil)
	service.sessionTTL = time.Minute
	service.now = func() time.Time { return now }

	token, err := service.CreateSession(model.User{ID: 1, Username: "admin", Role: model.UserRoleAdmin})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if user, ok := service.UserForSession(token); !ok || user.Username != "admin" {
		t.Fatalf("session lookup = %+v ok=%v, want admin", user, ok)
	}

	now = now.Add(time.Minute)
	if user, ok := service.UserForSession(token); ok {
		t.Fatalf("expired session lookup = %+v ok=true, want false", user)
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if len(service.sessions) != 0 {
		t.Fatalf("sessions after expired lookup = %d, want 0", len(service.sessions))
	}
}
