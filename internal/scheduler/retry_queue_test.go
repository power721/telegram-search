package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"tg-search/internal/retry"
)

func TestRetryQueueRetriesTemporaryFailureAndRecordsSuccess(t *testing.T) {
	var attempts int
	queue := NewRetryQueue(RetryQueueOptions{
		Policy: retry.Policy{
			BaseDelay: time.Millisecond,
			MaxDelay:  time.Millisecond,
			MaxTries:  3,
			Sleep:     func(context.Context, time.Duration) error { return nil },
		},
		Logger: zap.NewNop(),
	})

	job := queue.Enqueue(context.Background(), "sync", func(context.Context) error {
		attempts++
		if attempts == 1 {
			return retry.Temporary(errors.New("temporary"))
		}
		return nil
	})

	done, err := queue.Wait(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if done.Status != RetryJobSucceeded {
		t.Fatalf("status = %q, want succeeded", done.Status)
	}
	if done.Attempts != 2 || attempts != 2 {
		t.Fatalf("attempts job=%d local=%d, want 2", done.Attempts, attempts)
	}
}

func TestRetryQueueRecordsPermanentFailureWithoutRetry(t *testing.T) {
	var attempts int
	queue := NewRetryQueue(RetryQueueOptions{
		Policy: retry.Policy{
			BaseDelay: time.Millisecond,
			MaxDelay:  time.Millisecond,
			MaxTries:  3,
			Sleep:     func(context.Context, time.Duration) error { return nil },
		},
		Logger: zap.NewNop(),
	})

	job := queue.Enqueue(context.Background(), "sync", func(context.Context) error {
		attempts++
		return retry.Permanent(errors.New("bad channel"))
	})

	done, err := queue.Wait(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if done.Status != RetryJobFailed {
		t.Fatalf("status = %q, want failed", done.Status)
	}
	if done.Attempts != 1 || attempts != 1 {
		t.Fatalf("attempts job=%d local=%d, want 1", done.Attempts, attempts)
	}
	if done.Error == "" {
		t.Fatal("error is empty, want failure reason")
	}
}

func TestRetryQueueStopWaitsForRunningJobs(t *testing.T) {
	queue := NewRetryQueue(RetryQueueOptions{Logger: zap.NewNop()})
	release := make(chan struct{})
	job := queue.Enqueue(context.Background(), "sync", func(context.Context) error {
		<-release
		return nil
	})

	stopped := make(chan error, 1)
	go func() {
		stopped <- queue.Stop(context.Background())
	}()
	time.Sleep(time.Millisecond)
	close(release)

	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("Stop returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Stop")
	}
	done, err := queue.Wait(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if done.Status != RetryJobSucceeded {
		t.Fatalf("status = %q, want succeeded", done.Status)
	}
}
