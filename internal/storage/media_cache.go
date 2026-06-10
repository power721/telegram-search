package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"tg-search/internal/config"
	"tg-search/internal/model"
)

const (
	DefaultMediaCacheTTL = 7 * 24 * time.Hour
	mediaCacheTrimRatio  = 0.9
)

type MediaCache struct {
	root     string
	maxBytes int64
	ttl      time.Duration
	now      func() time.Time
}

type MediaCacheOptions struct {
	Root     string
	MaxBytes int64
	TTL      time.Duration
}

type MediaCacheEntry struct {
	Data []byte
	Size int64
}

type MediaCacheCleanupResult struct {
	ExpiredFiles int
	TrimmedFiles int
	BytesRemoved int64
	BytesBefore  int64
	BytesAfter   int64
}

func NewMediaCache(cfg config.Config) *MediaCache {
	return NewMediaCacheWithOptions(MediaCacheOptions{
		Root:     filepath.Join(cfg.Storage.Path, "thumbnails", "image-proxy"),
		MaxBytes: int64(cfg.Storage.MaxMediaCache),
		TTL:      DefaultMediaCacheTTL,
	})
}

func NewMediaCacheWithOptions(opts MediaCacheOptions) *MediaCache {
	return &MediaCache{
		root:     opts.Root,
		maxBytes: opts.MaxBytes,
		ttl:      opts.TTL,
		now:      time.Now,
	}
}

func MediaCacheKey(file model.FileResult) string {
	input := fmt.Sprintf("%d:%d:%d:%d:%d:%s",
		file.AccountID,
		file.ChannelID,
		file.TelegramMessageID,
		file.TelegramFileID,
		file.UpdatedAt.UnixNano(),
		file.MimeType,
	)
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

func (c *MediaCache) Get(ctx context.Context, key string) (MediaCacheEntry, bool, error) {
	if c == nil || c.root == "" || strings.TrimSpace(key) == "" {
		return MediaCacheEntry{}, false, nil
	}
	path := c.path(key)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return MediaCacheEntry{}, false, nil
	}
	if err != nil {
		return MediaCacheEntry{}, false, fmt.Errorf("read media cache %s: %w", path, err)
	}
	if err := ctx.Err(); err != nil {
		return MediaCacheEntry{}, false, err
	}
	now := c.timeNow()
	if err := os.Chtimes(path, now, now); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return MediaCacheEntry{}, false, fmt.Errorf("touch media cache %s: %w", path, err)
	}
	return MediaCacheEntry{Data: data, Size: int64(len(data))}, true, nil
}

func (c *MediaCache) Set(ctx context.Context, key string, data []byte) error {
	if c == nil || c.root == "" || strings.TrimSpace(key) == "" || len(data) == 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	path := c.path(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create media cache dir %s: %w", filepath.Dir(path), err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create media cache temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write media cache temp file %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close media cache temp file %s: %w", tmpName, err)
	}
	now := c.timeNow()
	if err := os.Chtimes(tmpName, now, now); err != nil {
		return fmt.Errorf("touch media cache temp file %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("commit media cache file %s: %w", path, err)
	}
	return nil
}

func (c *MediaCache) Cleanup(ctx context.Context) (MediaCacheCleanupResult, error) {
	var result MediaCacheCleanupResult
	if c == nil || c.root == "" {
		return result, nil
	}
	now := c.timeNow()
	files, err := c.cacheFiles(ctx)
	if err != nil {
		return result, err
	}
	for _, file := range files {
		result.BytesBefore += file.size
		if c.ttl > 0 && now.Sub(file.accessedAt) > c.ttl {
			if err := removeCacheFile(file.path); err != nil {
				return result, err
			}
			result.ExpiredFiles++
			result.BytesRemoved += file.size
			continue
		}
		result.BytesAfter += file.size
	}
	if c.maxBytes <= 0 || result.BytesAfter <= c.maxBytes {
		_ = c.removeEmptyDirs()
		return result, nil
	}
	files, err = c.cacheFiles(ctx)
	if err != nil {
		return result, err
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].accessedAt.Equal(files[j].accessedAt) {
			return files[i].path < files[j].path
		}
		return files[i].accessedAt.Before(files[j].accessedAt)
	})
	total := int64(0)
	for _, file := range files {
		total += file.size
	}
	target := int64(float64(c.maxBytes) * mediaCacheTrimRatio)
	if target < 0 {
		target = 0
	}
	for _, file := range files {
		if total <= target {
			break
		}
		if err := removeCacheFile(file.path); err != nil {
			return result, err
		}
		result.TrimmedFiles++
		result.BytesRemoved += file.size
		total -= file.size
	}
	result.BytesAfter = total
	_ = c.removeEmptyDirs()
	return result, nil
}

func (c *MediaCache) path(key string) string {
	clean := sanitizeCacheKey(key)
	if len(clean) < 4 {
		return filepath.Join(c.root, clean+".bin")
	}
	return filepath.Join(c.root, clean[:2], clean[2:4], clean+".bin")
}

func (c *MediaCache) timeNow() time.Time {
	if c.now != nil {
		return c.now().UTC()
	}
	return time.Now().UTC()
}

type cacheFile struct {
	path       string
	size       int64
	accessedAt time.Time
}

func (c *MediaCache) cacheFiles(ctx context.Context) ([]cacheFile, error) {
	var files []cacheFile
	err := filepath.WalkDir(c.root, func(path string, entry fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".tmp-") {
			info, statErr := entry.Info()
			if statErr != nil {
				return statErr
			}
			if c.ttl <= 0 || c.timeNow().Sub(info.ModTime()) > time.Hour {
				if err := removeCacheFile(path); err != nil {
					return err
				}
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		files = append(files, cacheFile{path: path, size: info.Size(), accessedAt: info.ModTime()})
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("walk media cache %s: %w", c.root, err)
	}
	return files, nil
}

func (c *MediaCache) removeEmptyDirs() error {
	return filepath.WalkDir(c.root, func(path string, entry fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil || path == c.root || !entry.IsDir() {
			return err
		}
		_ = os.Remove(path)
		return nil
	})
}

func removeCacheFile(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove media cache file %s: %w", path, err)
	}
	return nil
}

func sanitizeCacheKey(key string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(key) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "empty"
	}
	return b.String()
}
