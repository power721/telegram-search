package repository

import (
	"context"
	"path/filepath"
	"testing"

	"tg-search/internal/db"
	"tg-search/internal/model"
)

func TestUserRepositoryCreatesAndFindsAdmin(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "tg-search.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewUserRepository(conn)
	id, err := repo.Create(ctx, model.User{Username: "admin", PasswordHash: "hash", Role: model.UserRoleAdmin})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	user, err := repo.FindByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("find user: %v", err)
	}
	if user.ID != id || user.PasswordHash != "hash" || user.Role != model.UserRoleAdmin {
		t.Fatalf("user = %+v", user)
	}
	count, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
	updated, err := repo.UpdateCredentials(ctx, id, "root", "new-hash")
	if err != nil {
		t.Fatalf("update credentials: %v", err)
	}
	if updated.Username != "root" || updated.PasswordHash != "new-hash" || updated.ID != id {
		t.Fatalf("updated user = %+v", updated)
	}
}
