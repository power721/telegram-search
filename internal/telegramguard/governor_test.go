package telegramguard

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestGovernorSerializesRequestsPerAccount(t *testing.T) {
	ctx := context.Background()
	g := New(Options{})
	start := make(chan struct{})
	release := make(chan struct{})
	entered := make(chan int, 2)

	go func() {
		_ = g.Run(ctx, 1, OperationFetchHistory, func() error {
			entered <- 1
			close(start)
			<-release
			return nil
		})
	}()
	<-start
	if got := <-entered; got != 1 {
		t.Fatalf("entered = %d, want 1", got)
	}

	done := make(chan struct{})
	go func() {
		_ = g.Run(ctx, 1, OperationFetchHistory, func() error {
			entered <- 2
			return nil
		})
		close(done)
	}()

	select {
	case got := <-entered:
		t.Fatalf("second request entered before first finished: %d", got)
	case <-time.After(10 * time.Millisecond):
	}

	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("second request did not complete")
	}
	if got := <-entered; got != 2 {
		t.Fatalf("entered = %d, want 2", got)
	}
}

func TestGovernorAllowsDifferentAccountsConcurrently(t *testing.T) {
	ctx := context.Background()
	g := New(Options{})
	started := make(chan int, 2)
	release := make(chan struct{})
	var wg sync.WaitGroup

	for _, accountID := range []int64{1, 2} {
		accountID := accountID
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = g.Run(ctx, accountID, OperationFetchHistory, func() error {
				started <- int(accountID)
				<-release
				return nil
			})
		}()
	}

	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("different-account request did not enter concurrently")
		}
	}
	close(release)
	wg.Wait()
}

func TestGovernorWaitsBetweenAccountRequests(t *testing.T) {
	var now time.Time
	var slept []time.Duration
	g := New(Options{
		Interval: 2 * time.Second,
		Now:      func() time.Time { return now },
		Sleep: func(ctx context.Context, d time.Duration) error {
			slept = append(slept, d)
			now = now.Add(d)
			return nil
		},
	})

	if err := g.Run(context.Background(), 1, OperationFetchHistory, func() error { return nil }); err != nil {
		t.Fatalf("first Run returned error: %v", err)
	}
	if err := g.Run(context.Background(), 1, OperationFetchHistory, func() error { return nil }); err != nil {
		t.Fatalf("second Run returned error: %v", err)
	}

	if len(slept) != 1 || slept[0] != 2*time.Second {
		t.Fatalf("slept = %v, want [2s]", slept)
	}
}
