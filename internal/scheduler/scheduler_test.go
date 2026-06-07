package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestSchedulerRunsJobAndStops(t *testing.T) {
	var runs int64
	job := jobFunc{
		name: "count",
		run: func(context.Context) error {
			atomic.AddInt64(&runs, 1)
			return nil
		},
	}
	s := New(Options{Interval: time.Millisecond, Jobs: []Job{job}, Logger: zap.NewNop()})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)
	waitForSchedulerRuns(t, &runs, 1)
	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	before := atomic.LoadInt64(&runs)
	time.Sleep(3 * time.Millisecond)
	after := atomic.LoadInt64(&runs)
	if after != before {
		t.Fatalf("runs after stop = %d, want %d", after, before)
	}
}

func TestSchedulerContinuesAfterJobError(t *testing.T) {
	var runs int64
	job := jobFunc{
		name: "flaky",
		run: func(context.Context) error {
			atomic.AddInt64(&runs, 1)
			return errors.New("cleanup failed")
		},
	}
	s := New(Options{Interval: time.Millisecond, Jobs: []Job{job}, Logger: zap.NewNop()})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)
	waitForSchedulerRuns(t, &runs, 2)
	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
}

func TestCleanupJobRuns(t *testing.T) {
	job := CleanupJob{Logger: zap.NewNop()}
	if job.Name() != "cleanup" {
		t.Fatalf("name = %q, want cleanup", job.Name())
	}
	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}

type jobFunc struct {
	name string
	run  func(context.Context) error
}

func (j jobFunc) Name() string {
	return j.name
}

func (j jobFunc) Run(ctx context.Context) error {
	return j.run(ctx)
}

func waitForSchedulerRuns(t *testing.T, runs *int64, want int64) {
	t.Helper()
	deadline := time.After(time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatalf("runs = %d, want at least %d", atomic.LoadInt64(runs), want)
		case <-ticker.C:
			if atomic.LoadInt64(runs) >= want {
				return
			}
		}
	}
}
