package storage

import (
	"os"
	"path/filepath"
	"testing"

	"tg-search/internal/config"
)

func TestUsageCountsDBIndexAndMediaCache(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "tg-search.db"), 10)
	writeFile(t, filepath.Join(root, "index", "fts.data"), 20)
	writeFile(t, filepath.Join(root, "thumbnails", "thumb.bin"), 30)
	service := NewUsageService(config.Config{
		Storage: config.StorageConfig{
			Path:          root,
			MaxDBSize:     config.Size(9),
			MaxMediaCache: config.Size(100),
		},
	})
	usage, err := service.Usage()
	if err != nil {
		t.Fatalf("Usage returned error: %v", err)
	}
	if usage.DBBytes != 10 || usage.IndexBytes != 20 || usage.MediaCacheBytes != 30 || usage.TotalBytes != 60 {
		t.Fatalf("usage = %+v", usage)
	}
	if !usage.DBOverQuota {
		t.Fatalf("DBOverQuota = false, want true")
	}
	if usage.MediaOverQuota {
		t.Fatalf("MediaOverQuota = true, want false")
	}
}

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
