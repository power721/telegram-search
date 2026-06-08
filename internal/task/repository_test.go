package task

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"tg-search/internal/db"
	"tg-search/internal/model"
)

func TestTaskRepository(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	repo := NewRepository(conn)
	created, err := repo.Create(ctx, model.Task{
		Type:        model.TaskTypeHistorySync,
		Total:       100,
		Message:     "queued history sync",
		PayloadJSON: `{"channel_id":1}`,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.ID == 0 || created.Status != model.TaskStatusQueued || created.Type != model.TaskTypeHistorySync {
		t.Fatalf("created task = %+v", created)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("created timestamps = %+v", created)
	}

	if err := repo.UpdateStatus(ctx, created.ID, model.TaskStatusRunning, StatusUpdate{
		Message:   "running",
		StartedAt: ptrTime(time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)),
	}); err != nil {
		t.Fatalf("UpdateStatus running returned error: %v", err)
	}
	if err := repo.AppendProgress(ctx, created.ID, 25, 100, "saved first batch"); err != nil {
		t.Fatalf("AppendProgress returned error: %v", err)
	}

	got, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if got.Status != model.TaskStatusRunning || got.Progress != 25 || got.Total != 100 || got.Message != "saved first batch" {
		t.Fatalf("updated task = %+v", got)
	}
	if got.StartedAt == nil || !got.StartedAt.Equal(time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("started_at = %v", got.StartedAt)
	}

	running, err := repo.List(ctx, ListFilter{Status: model.TaskStatusRunning, Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(running) != 1 || running[0].ID != created.ID {
		t.Fatalf("running tasks = %+v", running)
	}
}

func TestTaskRepositoryRestartQuery(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	repo := NewRepository(conn)
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	running := createTaskForRestart(t, ctx, repo, model.TaskStatusRunning, nil)
	canceling := createTaskForRestart(t, ctx, repo, model.TaskStatusCanceling, nil)
	paused := createTaskForRestart(t, ctx, repo, model.TaskStatusPaused, nil)
	reconnecting := createTaskForRestart(t, ctx, repo, model.TaskStatusReconnecting, nil)
	floodPast := createTaskForRestart(t, ctx, repo, model.TaskStatusFloodWait, ptrTime(now.Add(-time.Minute)))
	_ = createTaskForRestart(t, ctx, repo, model.TaskStatusFloodWait, ptrTime(now.Add(time.Hour)))
	_ = createTaskForRestart(t, ctx, repo, model.TaskStatusSucceeded, nil)
	_ = createTaskForRestart(t, ctx, repo, model.TaskStatusCanceled, nil)

	restartable, err := repo.ListRestartable(ctx, now)
	if err != nil {
		t.Fatalf("ListRestartable returned error: %v", err)
	}
	gotIDs := map[int64]bool{}
	for _, item := range restartable {
		gotIDs[item.ID] = true
	}
	for _, id := range []int64{running.ID, canceling.ID, paused.ID, reconnecting.ID, floodPast.ID} {
		if !gotIDs[id] {
			t.Fatalf("restartable ids = %+v, missing %d", gotIDs, id)
		}
	}
	if len(restartable) != 5 {
		t.Fatalf("restartable len = %d, want 5: %+v", len(restartable), restartable)
	}
}

func createTaskForRestart(t *testing.T, ctx context.Context, repo *Repository, status string, nextRunAt *time.Time) model.Task {
	t.Helper()
	created, err := repo.Create(ctx, model.Task{Type: model.TaskTypeHistorySync, PayloadJSON: `{}`})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := repo.UpdateStatus(ctx, created.ID, status, StatusUpdate{NextRunAt: nextRunAt}); err != nil {
		t.Fatalf("update task %d to %s: %v", created.ID, status, err)
	}
	got, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("find task: %v", err)
	}
	return got
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
