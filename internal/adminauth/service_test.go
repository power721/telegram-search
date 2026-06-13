package adminauth

import (
	"context"
	"database/sql"
	"errors"
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
	service := NewService(users, repository.NewAdminSessionRepository(conn))
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
	userID, err := users.Create(ctx, model.User{Username: "admin", PasswordHash: "hash", Role: model.UserRoleAdmin})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	user, err := users.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("find user: %v", err)
	}
	sessions := repository.NewAdminSessionRepository(conn)
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	service := NewService(users, sessions)
	service.sessionTTL = time.Minute
	service.now = func() time.Time { return now }

	token, err := service.CreateSession(ctx, user)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if user, ok := service.UserForSession(ctx, token); !ok || user.Username != "admin" {
		t.Fatalf("session lookup = %+v ok=%v, want admin", user, ok)
	}

	now = now.Add(time.Minute)
	if user, ok := service.UserForSession(ctx, token); ok {
		t.Fatalf("expired session lookup = %+v ok=true, want false", user)
	}
	if _, err := sessions.FindUser(ctx, token, now); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expired session repository error = %v, want sql.ErrNoRows", err)
	}
}

func TestServicePersistsSessionsAcrossServiceInstances(t *testing.T) {
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
	userID, err := users.Create(ctx, model.User{Username: "admin", PasswordHash: "hash", Role: model.UserRoleAdmin})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	user, err := users.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("find user: %v", err)
	}

	first := NewService(users, repository.NewAdminSessionRepository(conn))
	token, err := first.CreateSession(ctx, user)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	second := NewService(users, repository.NewAdminSessionRepository(conn))
	got, ok := second.UserForSession(ctx, token)
	if !ok {
		t.Fatal("fresh service did not load persisted session")
	}
	if got.ID != user.ID || got.Username != user.Username || got.Role != user.Role {
		t.Fatalf("session user = %+v, want %+v", got, user)
	}
}
