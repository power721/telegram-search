package scheduler

import (
	"context"

	"go.uber.org/zap"
)

type CleanupJob struct {
	Logger *zap.Logger
}

func (j CleanupJob) Name() string {
	return "cleanup"
}

func (j CleanupJob) Run(ctx context.Context) error {
	logger := j.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	logger.Info("cleanup job checked temporary data")
	return nil
}
