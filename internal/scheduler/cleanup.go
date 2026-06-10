package scheduler

import (
	"context"

	"go.uber.org/zap"

	"tg-search/internal/storage"
)

type CleanupJob struct {
	Logger     *zap.Logger
	MediaCache *storage.MediaCache
}

func (j CleanupJob) Name() string {
	return "cleanup"
}

func (j CleanupJob) Run(ctx context.Context) error {
	logger := j.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	if j.MediaCache != nil {
		result, err := j.MediaCache.Cleanup(ctx)
		if err != nil {
			return err
		}
		logger.Info("cleanup job pruned media cache",
			zap.Int("expired_files", result.ExpiredFiles),
			zap.Int("trimmed_files", result.TrimmedFiles),
			zap.Int64("bytes_removed", result.BytesRemoved),
			zap.Int64("bytes_before", result.BytesBefore),
			zap.Int64("bytes_after", result.BytesAfter),
		)
		return nil
	}
	logger.Info("cleanup job checked temporary data")
	return nil
}
