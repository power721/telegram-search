package task

import (
	"context"
	"errors"
	"testing"
	"time"

	"tg-search/internal/model"
)

func TestRunnerCancelStopsRunningTask(t *testing.T) {
	ctx := context.Background()
	repo := openTaskRepository(t)
	service := NewService(repo)
	runner := NewRunner(service)

	item, err := service.Enqueue(ctx, model.TaskTypeHistorySync, map[string]any{"channel_id": 10})
	if err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}

	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- runner.Run(ctx, item, func(runCtx context.Context) error {
			close(started)
			<-runCtx.Done()
			return runCtx.Err()
		})
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("runner did not start worker")
	}

	if err := runner.Cancel(ctx, item.ID); err != nil {
		t.Fatalf("Cancel returned error: %v", err)
	}

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("runner did not stop after cancel")
	}

	canceled := mustFindTask(t, ctx, repo, item.ID)
	if canceled.Status != model.TaskStatusCanceled || canceled.FinishedAt == nil {
		t.Fatalf("canceled task = %+v", canceled)
	}
}
