package retry

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

type Kind string

const (
	KindTemporary Kind = "temporary"
	KindFloodWait Kind = "flood_wait"
	KindPermanent Kind = "permanent"
)

var floodWaitText = regexp.MustCompile(`(?i)FLOOD[_ ]?WAIT[_ ]?(\d+)`)

type Classification struct {
	Kind Kind
	Wait time.Duration
	Err  error
}

type Attempt struct {
	Number         int
	Classification Classification
	Delay          time.Duration
}

type Policy struct {
	BaseDelay time.Duration
	MaxDelay  time.Duration
	MaxTries  int
	Sleep     func(context.Context, time.Duration) error
}

type temporaryError struct {
	err error
}

func (e temporaryError) Error() string {
	return e.err.Error()
}

func (e temporaryError) Unwrap() error {
	return e.err
}

type permanentError struct {
	err error
}

func (e permanentError) Error() string {
	return e.err.Error()
}

func (e permanentError) Unwrap() error {
	return e.err
}

type FloodWaitError struct {
	Seconds int
	Err     error
}

func (e FloodWaitError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("flood wait %ds: %v", e.Seconds, e.Err)
	}
	return fmt.Sprintf("flood wait %ds", e.Seconds)
}

func (e FloodWaitError) Unwrap() error {
	return e.Err
}

func Temporary(err error) error {
	if err == nil {
		return nil
	}
	return temporaryError{err: err}
}

func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return permanentError{err: err}
}

func FloodWait(seconds int, err error) error {
	if seconds < 0 {
		seconds = 0
	}
	return FloodWaitError{Seconds: seconds, Err: err}
}

func DefaultPolicy() Policy {
	return Policy{
		BaseDelay: time.Second,
		MaxDelay:  30 * time.Minute,
		MaxTries:  3,
		Sleep:     sleepContext,
	}
}

func Classify(err error) Classification {
	if err == nil {
		return Classification{}
	}
	var flood FloodWaitError
	if errors.As(err, &flood) {
		return Classification{Kind: KindFloodWait, Wait: time.Duration(flood.Seconds) * time.Second, Err: err}
	}
	matches := floodWaitText.FindStringSubmatch(err.Error())
	if len(matches) == 2 {
		seconds, parseErr := strconv.Atoi(matches[1])
		if parseErr == nil {
			return Classification{Kind: KindFloodWait, Wait: time.Duration(seconds) * time.Second, Err: err}
		}
	}
	var permanent permanentError
	if errors.As(err, &permanent) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return Classification{Kind: KindPermanent, Err: err}
	}
	return Classification{Kind: KindTemporary, Err: err}
}

func (p Policy) Delay(attempt int, classification Classification) time.Duration {
	p = p.normalized()
	if classification.Kind == KindFloodWait && classification.Wait > 0 {
		if classification.Wait > p.MaxDelay {
			return p.MaxDelay
		}
		return classification.Wait
	}
	delay := p.BaseDelay
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= p.MaxDelay {
			return p.MaxDelay
		}
	}
	return delay
}

func (p Policy) Run(ctx context.Context, fn func() error, onRetry func(context.Context, Attempt)) error {
	p = p.normalized()
	var last error
	for attempt := 1; attempt <= p.MaxTries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		last = err
		classification := Classify(err)
		if classification.Kind == KindPermanent || attempt == p.MaxTries {
			return err
		}
		delay := p.Delay(attempt, classification)
		if onRetry != nil {
			onRetry(ctx, Attempt{Number: attempt, Classification: classification, Delay: delay})
		}
		if err := p.Sleep(ctx, delay); err != nil {
			return err
		}
	}
	return last
}

func (p Policy) normalized() Policy {
	if p.BaseDelay <= 0 {
		p.BaseDelay = time.Second
	}
	if p.MaxDelay <= 0 {
		p.MaxDelay = 30 * time.Minute
	}
	if p.MaxTries <= 0 {
		p.MaxTries = 3
	}
	if p.Sleep == nil {
		p.Sleep = sleepContext
	}
	return p
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
