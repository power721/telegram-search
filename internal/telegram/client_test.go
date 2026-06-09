package telegram

import (
	"context"
	"strings"
	"testing"
)

func TestInMemoryCodeStore(t *testing.T) {
	store := NewCodeStore()

	store.Save("+10000000000", "hash")
	hash, ok := store.Take("+10000000000")
	if !ok {
		t.Fatal("Take returned ok=false")
	}
	if hash != "hash" {
		t.Fatalf("hash = %q, want hash", hash)
	}
	if _, ok := store.Take("+10000000000"); ok {
		t.Fatal("Take returned ok=true after consuming code hash")
	}
}

func TestNopClientReturnsClearError(t *testing.T) {
	client := NopClient{}

	_, err := client.SendCode(context.Background(), "+10000000000", "/tmp/account.session")
	if err == nil {
		t.Fatal("SendCode returned nil error")
	}
}

func TestGotdClientRequiresCredentialsBeforeNetworkClient(t *testing.T) {
	client := NewGotdClient(StaticCredentialsProvider{}, nil)

	_, err := client.SendCode(context.Background(), "+10000000000", "/tmp/account.session")
	if err == nil || !strings.Contains(err.Error(), "telegram api settings are not configured") {
		t.Fatalf("SendCode error = %v, want missing credentials error", err)
	}
}
