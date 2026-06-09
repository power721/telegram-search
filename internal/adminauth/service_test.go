package adminauth

import (
	"context"
	"path/filepath"
	"testing"

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
