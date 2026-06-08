package task

import (
	"context"
	"errors"
	"sync"

	"tg-search/internal/model"
)

type WorkerFunc func(context.Context) error

type Runner struct {
	service *Service

	mu      sync.Mutex
	cancels map[int64]context.CancelFunc
}

func NewRunner(service *Service) *Runner {
	return &Runner{
		service: service,
		cancels: make(map[int64]context.CancelFunc),
	}
}

func (r *Runner) Run(ctx context.Context, item model.Task, worker WorkerFunc) error {
	if err := r.service.Start(ctx, item.ID); err != nil {
		return err
	}

	runCtx, cancel := context.WithCancel(ctx)
	r.mu.Lock()
	r.cancels[item.ID] = cancel
	r.mu.Unlock()
	defer func() {
		cancel()
		r.mu.Lock()
		delete(r.cancels, item.ID)
		r.mu.Unlock()
	}()

	err := worker(runCtx)
	if errors.Is(err, context.Canceled) {
		current, findErr := r.service.repo.FindByID(ctx, item.ID)
		if findErr == nil && current.Status == model.TaskStatusCanceling {
			_ = r.service.MarkCanceled(ctx, item.ID, "canceled")
		}
		return err
	}
	if err != nil {
		_ = r.service.Fail(ctx, item.ID, "worker_error", err.Error())
		return err
	}
	return r.service.Succeed(ctx, item.ID, "completed")
}

func (r *Runner) Cancel(ctx context.Context, id int64) error {
	if err := r.service.Cancel(ctx, id); err != nil {
		return err
	}
	r.mu.Lock()
	cancel := r.cancels[id]
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}
