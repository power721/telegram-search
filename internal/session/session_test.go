package session

import (
	"os"
	"path/filepath"
	"testing"
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
