package medialimit

import "context"

type Limiter struct {
	sem chan struct{}
}

func New(concurrency int) *Limiter {
	if concurrency <= 0 {
		concurrency = 1
	}
	return &Limiter{sem: make(chan struct{}, concurrency)}
}

func (l *Limiter) Run(ctx context.Context, fn func() error) error {
	if l == nil {
		return fn()
	}
	select {
	case l.sem <- struct{}{}:
		defer func() { <-l.sem }()
	case <-ctx.Done():
		return ctx.Err()
	}
	return fn()
}
