package task

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"tg-search/internal/model"
)

var ErrInvalidTransition = errors.New("invalid task status transition")
var ErrTaskNotDeletable = errors.New("task cannot be deleted while running or canceling")

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Enqueue(ctx context.Context, taskType string, payload any) (model.Task, error) {
	payloadJSON := "{}"
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return model.Task{}, fmt.Errorf("encode task payload: %w", err)
		}
		payloadJSON = string(encoded)
	}
	return s.repo.Create(ctx, model.Task{
		Type:        taskType,
		Status:      model.TaskStatusQueued,
		PayloadJSON: payloadJSON,
	})
}

func (s *Service) Start(ctx context.Context, id int64) error {
	now := time.Now().UTC()
	return s.transition(ctx, id, model.TaskStatusRunning, StatusUpdate{StartedAt: &now},
		model.TaskStatusQueued,
		model.TaskStatusPaused,
		model.TaskStatusReconnecting,
	)
}

func (s *Service) Succeed(ctx context.Context, id int64, message string) error {
	now := time.Now().UTC()
	return s.transition(ctx, id, model.TaskStatusSucceeded, StatusUpdate{
		Message:    message,
		FinishedAt: &now,
	}, model.TaskStatusRunning)
}

func (s *Service) Fail(ctx context.Context, id int64, code string, message string) error {
	now := time.Now().UTC()
	return s.transition(ctx, id, model.TaskStatusFailed, StatusUpdate{
		ErrorCode:    code,
		ErrorMessage: message,
		FinishedAt:   &now,
	}, model.TaskStatusRunning)
}

func (s *Service) Retry(ctx context.Context, id int64) error {
	current, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if !statusAllowed(current.Status, model.TaskStatusFailed, model.TaskStatusFloodWait, model.TaskStatusReconnecting) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, model.TaskStatusQueued)
	}
	retryCount := current.RetryCount + 1
	return s.repo.UpdateStatus(ctx, id, model.TaskStatusQueued, StatusUpdate{RetryCount: &retryCount})
}

type DeleteManyResult struct {
	Deleted     int     `json:"deleted"`
	RejectedIDs []int64 `json:"rejected_ids"`
	MissingIDs  []int64 `json:"missing_ids"`
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	current, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if !taskDeletable(current.Status) {
		return ErrTaskNotDeletable
	}
	return s.repo.Delete(ctx, id)
}

func (s *Service) DeleteMany(ctx context.Context, ids []int64) (DeleteManyResult, error) {
	result := DeleteManyResult{
		RejectedIDs: []int64{},
		MissingIDs:  []int64{},
	}
	seen := map[int64]struct{}{}
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		err := s.Delete(ctx, id)
		switch {
		case err == nil:
			result.Deleted++
		case errors.Is(err, ErrTaskNotDeletable):
			result.RejectedIDs = append(result.RejectedIDs, id)
		case errors.Is(err, sql.ErrNoRows):
			result.MissingIDs = append(result.MissingIDs, id)
		default:
			return result, err
		}
	}
	return result, nil
}

func (s *Service) Cancel(ctx context.Context, id int64) error {
	return s.transition(ctx, id, model.TaskStatusCanceling, StatusUpdate{},
		model.TaskStatusRunning,
		model.TaskStatusPaused,
		model.TaskStatusFloodWait,
		model.TaskStatusReconnecting,
	)
}

func (s *Service) Pause(ctx context.Context, id int64) error {
	return s.transition(ctx, id, model.TaskStatusPaused, StatusUpdate{}, model.TaskStatusRunning)
}

func (s *Service) Resume(ctx context.Context, id int64) error {
	return s.transition(ctx, id, model.TaskStatusRunning, StatusUpdate{}, model.TaskStatusPaused)
}

func (s *Service) SetFloodWait(ctx context.Context, id int64, nextRunAt time.Time, message string) error {
	next := nextRunAt.UTC()
	return s.transition(ctx, id, model.TaskStatusFloodWait, StatusUpdate{
		Message:   message,
		NextRunAt: &next,
	}, model.TaskStatusRunning)
}

func (s *Service) MarkCanceled(ctx context.Context, id int64, message string) error {
	now := time.Now().UTC()
	return s.transition(ctx, id, model.TaskStatusCanceled, StatusUpdate{
		Message:    message,
		FinishedAt: &now,
	}, model.TaskStatusCanceling)
}

func (s *Service) RestoreUnfinished(ctx context.Context, now time.Time) error {
	items, err := s.repo.ListRestartable(ctx, now.UTC())
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := s.repo.UpdateStatus(ctx, item.ID, model.TaskStatusQueued, StatusUpdate{}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) transition(ctx context.Context, id int64, nextStatus string, update StatusUpdate, allowedFrom ...string) error {
	current, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if !statusAllowed(current.Status, allowedFrom...) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, nextStatus)
	}
	return s.repo.UpdateStatus(ctx, id, nextStatus, update)
}

func statusAllowed(status string, allowed ...string) bool {
	for _, candidate := range allowed {
		if status == candidate {
			return true
		}
	}
	return false
}

func taskDeletable(status string) bool {
	return status != model.TaskStatusRunning && status != model.TaskStatusCanceling
}
