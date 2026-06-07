package scheduler

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

type Job interface {
	Name() string
	Run(context.Context) error
}

type Options struct {
	Interval time.Duration
	Jobs     []Job
	Logger   *zap.Logger
}

type Scheduler struct {
	interval time.Duration
	jobs     []Job
	logger   *zap.Logger

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func New(opts Options) *Scheduler {
	if opts.Interval <= 0 {
		opts.Interval = time.Hour
	}
	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}
	return &Scheduler{interval: opts.Interval, jobs: opts.Jobs, logger: opts.Logger}
}

func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	for _, job := range s.jobs {
		job := job
		s.wg.Add(1)
		go s.runJob(runCtx, job)
	}
	s.mu.Unlock()
}

func (s *Scheduler) Stop(ctx context.Context) error {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Scheduler) runJob(ctx context.Context, job Job) {
	defer s.wg.Done()
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := job.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				s.logger.Error("scheduler job failed", zap.String("job", job.Name()), zap.Error(err))
				continue
			}
			s.logger.Debug("scheduler job completed", zap.String("job", job.Name()))
		}
	}
}
