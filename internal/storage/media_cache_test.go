package storage

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tg-search/internal/model"
)

func TestMediaCacheGetSet(t *testing.T) {
	cache := NewMediaCacheWithOptions(MediaCacheOptions{
		Root: t.TempDir(),
		TTL:  time.Hour,
	})
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	cache.now = func() time.Time { return now }

	if err := cache.Set(context.Background(), "poster", []byte("image-data")); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	entry, hit, err := cache.Get(context.Background(), "poster")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !hit {
		t.Fatal("Get hit = false, want true")
	}
	if !bytes.Equal(entry.Data, []byte("image-data")) || entry.Size != int64(len("image-data")) {
		t.Fatalf("entry = %+v, want cached data", entry)
	}
}

func TestMediaCacheCleanupRemovesExpiredFiles(t *testing.T) {
	cache := NewMediaCacheWithOptions(MediaCacheOptions{
		Root: t.TempDir(),
		TTL:  time.Hour,
	})
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	cache.now = func() time.Time { return now }
	if err := cache.Set(context.Background(), "old", []byte("old")); err != nil {
		t.Fatalf("set old: %v", err)
	}
	oldPath := cache.path("old")
	oldTime := now.Add(-2 * time.Hour)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatalf("touch old cache file: %v", err)
	}
	if err := cache.Set(context.Background(), "fresh", []byte("fresh")); err != nil {
		t.Fatalf("set fresh: %v", err)
	}

	result, err := cache.Cleanup(context.Background())
	if err != nil {
		t.Fatalf("Cleanup returned error: %v", err)
	}
	if result.ExpiredFiles != 1 || result.TrimmedFiles != 0 || result.BytesRemoved != 3 {
		t.Fatalf("cleanup result = %+v, want one expired file", result)
	}
	if _, err := os.Stat(oldPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old file stat err = %v, want not exist", err)
	}
	if _, err := os.Stat(cache.path("fresh")); err != nil {
		t.Fatalf("fresh file stat returned error: %v", err)
	}
}

func TestMediaCacheCleanupTrimsOldestFilesOverQuota(t *testing.T) {
	cache := NewMediaCacheWithOptions(MediaCacheOptions{
		Root:     t.TempDir(),
		MaxBytes: 10,
		TTL:      time.Hour,
	})
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	cache.now = func() time.Time { return now }
	for i, key := range []string{"oldest", "middle", "newest"} {
		if err := cache.Set(context.Background(), key, bytes.Repeat([]byte{byte('a' + i)}, 5)); err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
		accessed := now.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(cache.path(key), accessed, accessed); err != nil {
			t.Fatalf("touch %s: %v", key, err)
		}
	}

	result, err := cache.Cleanup(context.Background())
	if err != nil {
		t.Fatalf("Cleanup returned error: %v", err)
	}
	if result.TrimmedFiles != 2 || result.BytesAfter != 5 {
		t.Fatalf("cleanup result = %+v, want two trimmed files and 5 bytes remaining", result)
	}
	for _, key := range []string{"oldest", "middle"} {
		if _, err := os.Stat(cache.path(key)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s stat err = %v, want not exist", key, err)
		}
	}
	if _, err := os.Stat(cache.path("newest")); err != nil {
		t.Fatalf("newest file stat returned error: %v", err)
	}
}

func TestMediaCacheKeyChangesWhenFileChanges(t *testing.T) {
	base := testFileResult(123, time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC))
	updated := testFileResult(123, base.UpdatedAt.Add(time.Second))
	if MediaCacheKey(base) == MediaCacheKey(updated) {
		t.Fatal("MediaCacheKey did not change when file updated_at changed")
	}
}

func TestMediaCacheKeyIncludesMediaContext(t *testing.T) {
	base := testFileResult(123, time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC))
	other := base
	other.AccountID = 2
	if MediaCacheKey(base) == MediaCacheKey(other) {
		t.Fatal("MediaCacheKey did not change when account changed")
	}
	other = base
	other.ChannelID = 3
	if MediaCacheKey(base) == MediaCacheKey(other) {
		t.Fatal("MediaCacheKey did not change when channel changed")
	}
}

func testFileResult(fileID int64, updatedAt time.Time) modelFileResult {
	return model.FileResult{
		File:              model.File{TelegramFileID: fileID, UpdatedAt: updatedAt, MimeType: "image/jpeg"},
		AccountID:         1,
		ChannelID:         1,
		TelegramMessageID: 1,
	}
}

type modelFileResult = model.FileResult

func TestMediaCacheRemoveEmptyRootMissing(t *testing.T) {
	cache := NewMediaCacheWithOptions(MediaCacheOptions{
		Root: filepath.Join(t.TempDir(), "missing"),
		TTL:  time.Hour,
	})
	if _, err := cache.Cleanup(context.Background()); err != nil {
		t.Fatalf("Cleanup returned error for missing root: %v", err)
	}
}
