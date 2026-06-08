package task

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"tg-search/internal/db"
	"tg-search/internal/model"
)

func TestTaskStateTransitions(t *testing.T) {
	ctx := context.Background()
	repo := openTaskRepository(t)
	service := NewService(repo)

	queued, err := service.Enqueue(ctx, model.TaskTypeHistorySync, map[string]any{"channel_id": 1})
	if err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}
	if queued.Status != model.TaskStatusQueued || queued.PayloadJSON == "" {
		t.Fatalf("queued task = %+v", queued)
	}
	if err := service.Start(ctx, queued.ID); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	running := mustFindTask(t, ctx, repo, queued.ID)
	if running.Status != model.TaskStatusRunning || running.StartedAt == nil {
		t.Fatalf("running task = %+v", running)
	}
	if err := service.Succeed(ctx, queued.ID, "done"); err != nil {
		t.Fatalf("Succeed returned error: %v", err)
	}
	succeeded := mustFindTask(t, ctx, repo, queued.ID)
	if succeeded.Status != model.TaskStatusSucceeded || succeeded.FinishedAt == nil || succeeded.Message != "done" {
		t.Fatalf("succeeded task = %+v", succeeded)
	}
	if err := service.Start(ctx, queued.ID); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("Start succeeded task error = %v, want ErrInvalidTransition", err)
	}

	failed, _ := service.Enqueue(ctx, model.TaskTypeHistorySync, map[string]any{"channel_id": 2})
	if err := service.Start(ctx, failed.ID); err != nil {
		t.Fatalf("Start failed task setup: %v", err)
	}
	if err := service.Fail(ctx, failed.ID, "temporary", "temporary failure"); err != nil {
		t.Fatalf("Fail returned error: %v", err)
	}
	if err := service.Retry(ctx, failed.ID); err != nil {
		t.Fatalf("Retry returned error: %v", err)
	}
	retried := mustFindTask(t, ctx, repo, failed.ID)
	if retried.Status != model.TaskStatusQueued || retried.RetryCount != 1 {
		t.Fatalf("retried task = %+v", retried)
	}

	paused, _ := service.Enqueue(ctx, model.TaskTypeHistorySync, map[string]any{"channel_id": 3})
	if err := service.Start(ctx, paused.ID); err != nil {
		t.Fatalf("Start pause task setup: %v", err)
	}
	if err := service.Pause(ctx, paused.ID); err != nil {
		t.Fatalf("Pause returned error: %v", err)
	}
	if err := service.Resume(ctx, paused.ID); err != nil {
		t.Fatalf("Resume returned error: %v", err)
	}
	resumed := mustFindTask(t, ctx, repo, paused.ID)
	if resumed.Status != model.TaskStatusRunning {
		t.Fatalf("resumed task = %+v", resumed)
	}

	flood, _ := service.Enqueue(ctx, model.TaskTypeHistorySync, map[string]any{"channel_id": 4})
	if err := service.Start(ctx, flood.ID); err != nil {
		t.Fatalf("Start flood task setup: %v", err)
	}
	nextRunAt := time.Date(2026, 6, 8, 13, 0, 0, 0, time.UTC)
	if err := service.SetFloodWait(ctx, flood.ID, nextRunAt, "FLOOD_WAIT_60"); err != nil {
		t.Fatalf("SetFloodWait returned error: %v", err)
	}
	flooded := mustFindTask(t, ctx, repo, flood.ID)
	if flooded.Status != model.TaskStatusFloodWait || flooded.NextRunAt == nil || !flooded.NextRunAt.Equal(nextRunAt) {
		t.Fatalf("flood wait task = %+v", flooded)
	}

	canceling, _ := service.Enqueue(ctx, model.TaskTypeHistorySync, map[string]any{"channel_id": 5})
	if err := service.Start(ctx, canceling.ID); err != nil {
		t.Fatalf("Start cancel task setup: %v", err)
	}
	if err := service.Cancel(ctx, canceling.ID); err != nil {
		t.Fatalf("Cancel returned error: %v", err)
	}
	canceled := mustFindTask(t, ctx, repo, canceling.ID)
	if canceled.Status != model.TaskStatusCanceling {
		t.Fatalf("canceling task = %+v", canceled)
	}
}

func TestTaskStateTransitionsRejectInvalid(t *testing.T) {
	ctx := context.Background()
	repo := openTaskRepository(t)
	service := NewService(repo)
	queued, err := service.Enqueue(ctx, model.TaskTypeHistorySync, map[string]any{"channel_id": 1})
	if err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}

	for name, fn := range map[string]func() error{
		"succeed queued": func() error { return service.Succeed(ctx, queued.ID, "done") },
		"fail queued":    func() error { return service.Fail(ctx, queued.ID, "x", "failed") },
		"pause queued":   func() error { return service.Pause(ctx, queued.ID) },
		"resume queued":  func() error { return service.Resume(ctx, queued.ID) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := fn(); !errors.Is(err, ErrInvalidTransition) {
				t.Fatalf("error = %v, want ErrInvalidTransition", err)
			}
		})
	}
}

func openTaskRepository(t *testing.T) *Repository {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(context.Background(), conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	return NewRepository(conn)
}

func mustFindTask(t *testing.T, ctx context.Context, repo *Repository, id int64) model.Task {
	t.Helper()
	item, err := repo.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("FindByID(%d): %v", id, err)
	}
	return item
}
