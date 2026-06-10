package medialimit

import (
	"context"
	"sync"
)

type Limiter struct {
	mu  sync.RWMutex
	sem chan struct{}
}

func New(concurrency int) *Limiter {
	if concurrency <= 0 {
		concurrency = 1
	}
	return &Limiter{sem: make(chan struct{}, concurrency)}
}

func (l *Limiter) Update(concurrency int) {
	if l == nil {
		return
	}
	if concurrency <= 0 {
		concurrency = 1
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sem = make(chan struct{}, concurrency)
}

func (l *Limiter) Run(ctx context.Context, fn func() error) error {
	if l == nil {
		return fn()
	}
	sem := l.current()
	select {
	case sem <- struct{}{}:
		defer func() { <-sem }()
	case <-ctx.Done():
		return ctx.Err()
	}
	return fn()
}

func (l *Limiter) current() chan struct{} {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.sem
}
