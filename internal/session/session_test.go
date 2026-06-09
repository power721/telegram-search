package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPathForAccountIsDeterministic(t *testing.T) {
	manager := NewManager("/data/tg-search/sessions")

	path := manager.PathForAccount(42)

	want := filepath.Join("/data/tg-search/sessions", "account-42.session.json")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}

func TestRemoveForAccountDeletesSessionFile(t *testing.T) {
	dir := t.TempDir()
	manager := NewManager(dir)
	path := manager.PathForAccount(42)
	if err := os.WriteFile(path, []byte("session"), 0o600); err != nil {
		t.Fatalf("write session: %v", err)
	}

	if err := manager.RemoveForAccount(42); err != nil {
		t.Fatalf("RemoveForAccount returned error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("session file stat error = %v, want not exist", err)
	}
}

func TestRemoveForAccountIgnoresMissingSessionFile(t *testing.T) {
	manager := NewManager(t.TempDir())

	if err := manager.RemoveForAccount(42); err != nil {
		t.Fatalf("RemoveForAccount returned error: %v", err)
	}
}

func TestManagerMovesTemporaryQRSessionToAccount(t *testing.T) {
	dir := t.TempDir()
	manager := NewManager(dir)
	temp := manager.PathForTemporary("qr-login-abc")
	if err := os.WriteFile(temp, []byte(`{"auth":"temp"}`), 0o600); err != nil {
		t.Fatalf("write temp session: %v", err)
	}

	final, err := manager.MoveTemporaryToAccount(temp, 7)
	if err != nil {
		t.Fatalf("MoveTemporaryToAccount returned error: %v", err)
	}
	if final != manager.PathForAccount(7) {
		t.Fatalf("final path = %q, want account path", final)
	}
	if _, err := os.Stat(temp); !os.IsNotExist(err) {
		t.Fatalf("temp stat err = %v, want not exist", err)
	}
	data, err := os.ReadFile(final)
	if err != nil {
		t.Fatalf("read final session: %v", err)
	}
	if string(data) != `{"auth":"temp"}` {
		t.Fatalf("final data = %q", data)
	}
}

func TestManagerMoveTemporaryQRSessionReplacesExistingAccountSession(t *testing.T) {
	dir := t.TempDir()
	manager := NewManager(dir)
	final := manager.PathForAccount(7)
	if err := os.WriteFile(final, []byte(`{"auth":"old"}`), 0o600); err != nil {
		t.Fatalf("write old account session: %v", err)
	}
	temp := manager.PathForTemporary("qr-login-new")
	if err := os.WriteFile(temp, []byte(`{"auth":"new"}`), 0o600); err != nil {
		t.Fatalf("write temp session: %v", err)
	}

	if _, err := manager.MoveTemporaryToAccount(temp, 7); err != nil {
		t.Fatalf("MoveTemporaryToAccount returned error: %v", err)
	}

	data, err := os.ReadFile(final)
	if err != nil {
		t.Fatalf("read final session: %v", err)
	}
	if string(data) != `{"auth":"new"}` {
		t.Fatalf("final data = %q", data)
	}
}

func TestManagerMoveTemporaryQRSessionWaitsForDelayedSessionFile(t *testing.T) {
	dir := t.TempDir()
	manager := NewManager(dir)
	temp := manager.PathForTemporary("qr-login-delayed")
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = os.WriteFile(temp, []byte(`{"auth":"delayed"}`), 0o600)
	}()

	final, err := manager.MoveTemporaryToAccount(temp, 9)
	if err != nil {
		t.Fatalf("MoveTemporaryToAccount returned error: %v", err)
	}
	data, err := os.ReadFile(final)
	if err != nil {
		t.Fatalf("read final session: %v", err)
	}
	if string(data) != `{"auth":"delayed"}` {
		t.Fatalf("final data = %q", data)
	}
}
