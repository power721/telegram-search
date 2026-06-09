package medialimit

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestLimiterSerializesWhenConcurrencyOne(t *testing.T) {
	limiter := New(1)
	started := make(chan struct{})
	release := make(chan struct{})

	go func() {
		_ = limiter.Run(context.Background(), func() error {
			close(started)
			<-release
			return nil
		})
	}()
	<-started

	done := make(chan struct{})
	go func() {
		_ = limiter.Run(context.Background(), func() error {
			close(done)
			return nil
		})
	}()

	select {
	case <-done:
		t.Fatal("second call ran before first released")
	case <-time.After(10 * time.Millisecond):
	}

	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("second call did not run after release")
	}
}

func TestLimiterAllowsConfiguredConcurrency(t *testing.T) {
	limiter := New(2)
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = limiter.Run(context.Background(), func() error {
				started <- struct{}{}
				<-release
				return nil
			})
		}()
	}

	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("call did not start")
		}
	}
	close(release)
	wg.Wait()
}
