package task

import (
	"context"
	"errors"

	"go.uber.org/zap"

	"tg-search/internal/config"
	"tg-search/internal/model"
)

type CleanupJob struct {
	service *Service
	policy  config.TaskRetentionConfig
	logger  *zap.Logger
}

func NewCleanupJob(service *Service, policy config.TaskRetentionConfig, logger *zap.Logger) CleanupJob {
	return CleanupJob{
		service: service,
		policy:  policy,
		logger:  logger,
	}
}

func (j CleanupJob) Name() string {
	return "task_cleanup"
}

func (j CleanupJob) Run(ctx context.Context) error {
	statusPolicies := map[string]int{
		model.TaskStatusSucceeded:    j.policy.SucceededDays,
		model.TaskStatusFailed:       j.policy.FailedDays,
		model.TaskStatusCanceled:     j.policy.CanceledDays,
		model.TaskStatusPaused:       j.policy.PausedDays,
		model.TaskStatusFloodWait:    j.policy.FloodWaitDays,
		model.TaskStatusReconnecting: j.policy.ReconnectingDays,
	}

	totalDeleted := int64(0)
	hasError := false

	for status, days := range statusPolicies {
		if days <= 0 {
			continue // 0 means skip cleanup for this status
		}

		deleted, err := j.service.repo.DeleteOldTasks(ctx, status, days)
		if err != nil {
			j.logger.Error("task cleanup failed",
				zap.String("status", status),
				zap.Int("retention_days", days),
				zap.Error(err))
			hasError = true
			continue // Continue cleaning other statuses
		}

		if deleted > 0 {
			j.logger.Info("tasks cleaned up",
				zap.String("status", status),
				zap.Int("retention_days", days),
				zap.Int64("deleted", deleted))
			totalDeleted += deleted
		}
	}

	if totalDeleted > 0 {
		j.logger.Info("task cleanup completed", zap.Int64("total_deleted", totalDeleted))
	}

	if hasError {
		return errors.New("task cleanup completed with errors")
	}

	return nil
}
