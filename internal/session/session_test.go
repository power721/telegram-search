package session

import (
	"path/filepath"
	"testing"
)

func TestPathForAccountIsDeterministic(t *testing.T) {
	manager := NewManager("/data/tg-provider/sessions")

	path := manager.PathForAccount(42)

	want := filepath.Join("/data/tg-provider/sessions", "account-42.session.json")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}
