package task

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"tg-search/internal/model"
)

type Handler func(context.Context, model.Task, ProgressSink) error

type WorkerOptions struct {
	Service      *Service
	Repository   *Repository
	Handlers     map[string]Handler
	Events       *EventBroker
	PollInterval time.Duration
	BatchSize    int
}

type Worker struct {
	service      *Service
	repo         *Repository
	handlers     map[string]Handler
	events       *EventBroker
	pollInterval time.Duration
	batchSize    int

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewWorker(opts WorkerOptions) *Worker {
	if opts.PollInterval <= 0 {
		opts.PollInterval = time.Second
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 20
	}
	return &Worker{
		service:      opts.Service,
		repo:         opts.Repository,
		handlers:     opts.Handlers,
		events:       opts.Events,
		pollInterval: opts.PollInterval,
		batchSize:    opts.BatchSize,
	}
}

func (w *Worker) Start(ctx context.Context) {
	w.mu.Lock()
	if w.cancel != nil {
		w.mu.Unlock()
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.wg.Add(1)
	w.mu.Unlock()

	go w.loop(runCtx)
}

func (w *Worker) Stop(ctx context.Context) error {
	w.mu.Lock()
	cancel := w.cancel
	w.cancel = nil
	w.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *Worker) ProcessOnce(ctx context.Context) (int, error) {
	if w.service == nil || w.repo == nil {
		return 0, errors.New("task worker is not configured")
	}
	items, err := w.repo.List(ctx, ListFilter{Status: model.TaskStatusQueued, Limit: w.batchSize})
	if err != nil {
		return 0, err
	}
	var firstErr error
	processed := 0
	for _, item := range items {
		if err := w.processTask(ctx, item); err != nil && firstErr == nil {
			firstErr = err
		}
		processed++
	}
	return processed, firstErr
}

func (w *Worker) loop(ctx context.Context) {
	defer w.wg.Done()
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()
	for {
		_, _ = w.ProcessOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) processTask(ctx context.Context, item model.Task) error {
	handler := w.handlers[item.Type]
	if handler == nil {
		return w.failUnsupported(ctx, item)
	}
	if err := w.service.Start(ctx, item.ID); err != nil {
		return err
	}
	w.publishSnapshot(ctx, item.ID)

	err := handler(ctx, item, w.progressSink(item.ID))
	if errors.Is(err, context.Canceled) {
		current, findErr := w.repo.FindByID(ctx, item.ID)
		if findErr == nil && current.Status == model.TaskStatusCanceling {
			_ = w.service.MarkCanceled(ctx, item.ID, "canceled")
			w.publishSnapshot(ctx, item.ID)
		}
		return err
	}
	if err != nil {
		_ = w.service.Fail(ctx, item.ID, "worker_error", err.Error())
		w.publishSnapshot(ctx, item.ID)
		return err
	}

	current, err := w.repo.FindByID(ctx, item.ID)
	if err != nil {
		return err
	}
	switch current.Status {
	case model.TaskStatusRunning:
		err = w.service.Succeed(ctx, item.ID, "completed")
	case model.TaskStatusCanceling:
		err = w.service.MarkCanceled(ctx, item.ID, "canceled")
	}
	w.publishSnapshot(ctx, item.ID)
	return err
}

func (w *Worker) failUnsupported(ctx context.Context, item model.Task) error {
	if err := w.service.Start(ctx, item.ID); err != nil {
		return err
	}
	err := fmt.Errorf("unsupported task type %q", item.Type)
	_ = w.service.Fail(ctx, item.ID, "unsupported_task", err.Error())
	w.publishSnapshot(ctx, item.ID)
	return err
}

func (w *Worker) progressSink(taskID int64) ProgressSink {
	base := NewProgressSink(w.service, taskID)
	if w.events == nil {
		return base
	}
	return &eventingProgressSink{
		ProgressSink: base,
		floodWait:    base,
		publish:      func(ctx context.Context) { w.publishSnapshot(ctx, taskID) },
	}
}

func (w *Worker) publishSnapshot(ctx context.Context, taskID int64) {
	if w.events == nil {
		return
	}
	item, err := w.repo.FindByID(ctx, taskID)
	if err != nil {
		return
	}
	w.events.Publish(Event{Type: EventTaskUpdated, Payload: item})
}

type eventingProgressSink struct {
	ProgressSink
	floodWait FloodWaitSink
	publish   func(context.Context)
}

func (s *eventingProgressSink) Progress(ctx context.Context, progress int64, total int64, message string) error {
	if err := s.ProgressSink.Progress(ctx, progress, total, message); err != nil {
		return err
	}
	s.publish(ctx)
	return nil
}

func (s *eventingProgressSink) FloodWait(ctx context.Context, nextRunAt time.Time, message string) error {
	if err := s.floodWait.FloodWait(ctx, nextRunAt, message); err != nil {
		return err
	}
	s.publish(ctx)
	return nil
}
