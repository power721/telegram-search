package task

import (
	"context"
	"time"

	"tg-search/internal/model"
)

type ProgressSink interface {
	Progress(ctx context.Context, progress int64, total int64, message string) error
	Status(ctx context.Context) (string, error)
}

type FloodWaitSink interface {
	FloodWait(ctx context.Context, nextRunAt time.Time, message string) error
}

type ServiceProgressSink struct {
	service *Service
	taskID  int64
}

func NewProgressSink(service *Service, taskID int64) *ServiceProgressSink {
	return &ServiceProgressSink{service: service, taskID: taskID}
}

func (s *ServiceProgressSink) Progress(ctx context.Context, progress int64, total int64, message string) error {
	return s.service.repo.AppendProgress(ctx, s.taskID, progress, total, message)
}

func (s *ServiceProgressSink) Status(ctx context.Context) (string, error) {
	item, err := s.service.repo.FindByID(ctx, s.taskID)
	if err != nil {
		return "", err
	}
	return item.Status, nil
}

func (s *ServiceProgressSink) FloodWait(ctx context.Context, nextRunAt time.Time, message string) error {
	return s.service.SetFloodWait(ctx, s.taskID, nextRunAt, message)
}

func IsCancelingStatus(status string) bool {
	return status == model.TaskStatusCanceling || status == model.TaskStatusCanceled
}
