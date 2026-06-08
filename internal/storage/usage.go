package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"tg-search/internal/config"
	"tg-search/internal/model"
)

type UsageService struct {
	cfg config.Config
}

func NewUsageService(cfg config.Config) *UsageService {
	return &UsageService{cfg: cfg}
}

func (s *UsageService) Usage() (model.StorageUsage, error) {
	root := s.cfg.Storage.Path
	dbBytes, err := fileSize(filepath.Join(root, "tg-search.db"))
	if err != nil {
		return model.StorageUsage{}, err
	}
	indexBytes, err := dirSize(filepath.Join(root, "index"))
	if err != nil {
		return model.StorageUsage{}, err
	}
	mediaBytes, err := dirSize(filepath.Join(root, "thumbnails"))
	if err != nil {
		return model.StorageUsage{}, err
	}
	usage := model.StorageUsage{
		DBBytes:         dbBytes,
		IndexBytes:      indexBytes,
		MediaCacheBytes: mediaBytes,
		TotalBytes:      dbBytes + indexBytes + mediaBytes,
		MaxDBBytes:      int64(s.cfg.Storage.MaxDBSize),
		MaxMediaBytes:   int64(s.cfg.Storage.MaxMediaCache),
	}
	usage.DBOverQuota = usage.MaxDBBytes > 0 && usage.DBBytes > usage.MaxDBBytes
	usage.MediaOverQuota = usage.MaxMediaBytes > 0 && usage.MediaCacheBytes > usage.MaxMediaBytes
	return usage, nil
}

func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("stat %s: %w", path, err)
	}
	if info.IsDir() {
		return 0, fmt.Errorf("%s is a directory", path)
	}
	return info.Size(), nil
}

func dirSize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("walk %s: %w", root, err)
	}
	return total, nil
}
