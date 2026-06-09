package telegramguard

import (
	"context"
	"sync"
	"time"
)

type Operation string

const (
	OperationFetchHistory Operation = "fetch_history"
	OperationSearch       Operation = "search"
)

type Options struct {
	Interval time.Duration
	Sleep    func(context.Context, time.Duration) error
	Now      func() time.Time
}

type Governor struct {
	interval time.Duration
	sleep    func(context.Context, time.Duration) error
	now      func() time.Time

	mu       sync.Mutex
	accounts map[int64]*accountState
}

type accountState struct {
	mu      sync.Mutex
	nextRun time.Time
}

func New(opts Options) *Governor {
	sleep := opts.Sleep
	if sleep == nil {
		sleep = sleepContext
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	interval := opts.Interval
	if interval < 0 {
		interval = 0
	}
	return &Governor{
		interval: interval,
		sleep:    sleep,
		now:      now,
		accounts: map[int64]*accountState{},
	}
}

func (g *Governor) Run(ctx context.Context, accountID int64, operation Operation, fn func() error) error {
	if g == nil {
		return fn()
	}
	state := g.account(accountID)
	state.mu.Lock()
	defer state.mu.Unlock()

	now := g.now()
	if wait := state.nextRun.Sub(now); wait > 0 {
		if err := g.sleep(ctx, wait); err != nil {
			return err
		}
	}

	err := fn()
	if g.interval > 0 {
		state.nextRun = g.now().Add(g.interval)
	}
	return err
}

func (g *Governor) account(accountID int64) *accountState {
	g.mu.Lock()
	defer g.mu.Unlock()
	state := g.accounts[accountID]
	if state == nil {
		state = &accountState{}
		g.accounts[accountID] = state
	}
	return state
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
