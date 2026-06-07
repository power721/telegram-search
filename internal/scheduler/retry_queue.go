package scheduler

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"tg-provider/internal/retry"
)

const (
	RetryJobQueued    = "queued"
	RetryJobRunning   = "running"
	RetryJobSucceeded = "succeeded"
	RetryJobFailed    = "failed"
)

type RetryQueueOptions struct {
	Policy retry.Policy
	Logger *zap.Logger
}

type RetryJob struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Attempts  int       `json:"attempts"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RetryQueue struct {
	policy retry.Policy
	logger *zap.Logger

	nextID uint64
	mu     sync.Mutex
	jobs   map[string]*retryJobState
	wg     sync.WaitGroup
}

type retryJobState struct {
	mu   sync.Mutex
	job  RetryJob
	done chan struct{}
}

func NewRetryQueue(opts RetryQueueOptions) *RetryQueue {
	if opts.Policy.MaxTries == 0 && opts.Policy.BaseDelay == 0 && opts.Policy.MaxDelay == 0 && opts.Policy.Sleep == nil {
		opts.Policy = retry.DefaultPolicy()
	}
	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}
	return &RetryQueue{
		policy: opts.Policy,
		logger: opts.Logger,
		jobs:   map[string]*retryJobState{},
	}
}

func (q *RetryQueue) Enqueue(ctx context.Context, name string, fn func(context.Context) error) RetryJob {
	now := time.Now().UTC()
	id := fmt.Sprintf("%d", atomic.AddUint64(&q.nextID, 1))
	state := &retryJobState{
		job: RetryJob{
			ID:        id,
			Name:      name,
			Status:    RetryJobQueued,
			CreatedAt: now,
			UpdatedAt: now,
		},
		done: make(chan struct{}),
	}

	q.mu.Lock()
	q.jobs[id] = state
	q.wg.Add(1)
	q.mu.Unlock()

	go q.run(ctx, state, fn)
	return state.snapshot()
}

func (q *RetryQueue) Snapshot(id string) (RetryJob, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	state, ok := q.jobs[id]
	if !ok {
		return RetryJob{}, false
	}
	return state.snapshot(), true
}

func (q *RetryQueue) Wait(ctx context.Context, id string) (RetryJob, error) {
	q.mu.Lock()
	state, ok := q.jobs[id]
	q.mu.Unlock()
	if !ok {
		return RetryJob{}, fmt.Errorf("retry job %s not found", id)
	}
	select {
	case <-state.done:
		return state.snapshot(), nil
	case <-ctx.Done():
		return RetryJob{}, ctx.Err()
	}
}

func (q *RetryQueue) Stop(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *RetryQueue) run(ctx context.Context, state *retryJobState, fn func(context.Context) error) {
	defer q.wg.Done()
	defer close(state.done)

	state.update(func(job *RetryJob) {
		job.Status = RetryJobRunning
	})

	var attempts int
	err := q.policy.Run(ctx, func() error {
		attempts++
		state.update(func(job *RetryJob) {
			job.Attempts = attempts
			job.UpdatedAt = time.Now().UTC()
		})
		return fn(ctx)
	}, nil)
	if err != nil {
		state.update(func(job *RetryJob) {
			job.Status = RetryJobFailed
			job.Attempts = attempts
			job.Error = err.Error()
			job.UpdatedAt = time.Now().UTC()
		})
		q.logger.Warn("retry queue job failed", zap.String("job_id", state.job.ID), zap.String("name", state.job.Name), zap.Error(err))
		return
	}
	state.update(func(job *RetryJob) {
		job.Status = RetryJobSucceeded
		job.Attempts = attempts
		job.UpdatedAt = time.Now().UTC()
	})
}

func (s *retryJobState) snapshot() RetryJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.job
}

func (s *retryJobState) update(fn func(*RetryJob)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.job)
}
