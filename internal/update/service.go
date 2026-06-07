package update

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"

	"tg-provider/internal/model"
	"tg-provider/internal/repository"
)

var ErrServiceStopped = errors.New("update service stopped")

type EventProcessor interface {
	Process(context.Context, Event) error
}

type Listener interface {
	Run(ctx context.Context, account model.Account, emit func(Event) error) error
}

type ListenerFunc func(ctx context.Context, account model.Account, emit func(Event) error) error

func (f ListenerFunc) Run(ctx context.Context, account model.Account, emit func(Event) error) error {
	return f(ctx, account, emit)
}

type NopListener struct{}

func (NopListener) Run(context.Context, model.Account, func(Event) error) error {
	return nil
}

type ServiceOptions struct {
	Accounts      *repository.AccountRepository
	Processor     EventProcessor
	Listener      Listener
	QueueSize     int
	RetryInterval time.Duration
	Logger        *zap.Logger
}

type Service struct {
	accounts      *repository.AccountRepository
	processor     EventProcessor
	listener      Listener
	retryInterval time.Duration
	logger        *zap.Logger

	mu         sync.Mutex
	startOnce  sync.Once
	cancel     context.CancelFunc
	ctx        context.Context
	events     chan Event
	stopped    bool
	workerWG   sync.WaitGroup
	listenerWG sync.WaitGroup
}

func NewService(opts ServiceOptions) *Service {
	if opts.Listener == nil {
		opts.Listener = NopListener{}
	}
	if opts.RetryInterval <= 0 {
		opts.RetryInterval = time.Second
	}
	if opts.QueueSize <= 0 {
		opts.QueueSize = 100
	}
	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}
	return &Service{
		accounts:      opts.Accounts,
		processor:     opts.Processor,
		listener:      opts.Listener,
		retryInterval: opts.RetryInterval,
		logger:        opts.Logger,
		events:        make(chan Event, opts.QueueSize),
	}
}

func (s *Service) Start(ctx context.Context) {
	s.startOnce.Do(func() {
		s.ctx, s.cancel = context.WithCancel(ctx)
		s.workerWG.Add(1)
		go s.worker()
	})
}

func (s *Service) StartAccount(ctx context.Context, account model.Account) error {
	s.Start(ctx)
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return ErrServiceStopped
	}
	s.listenerWG.Add(1)
	s.mu.Unlock()

	go s.runListener(account)
	return nil
}

func (s *Service) Enqueue(ctx context.Context, event Event) error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return ErrServiceStopped
	}
	events := s.events
	s.mu.Unlock()

	select {
	case events <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return nil
	}
	s.stopped = true
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()

	if err := wait(ctx, &s.listenerWG); err != nil {
		return err
	}
	close(s.events)
	return wait(ctx, &s.workerWG)
}

func (s *Service) worker() {
	defer s.workerWG.Done()
	for event := range s.events {
		if s.processor == nil {
			continue
		}
		if err := s.processor.Process(s.ctx, event); err != nil {
			s.logger.Error("process update event", zap.Error(err), zap.String("type", string(event.Type)))
		}
	}
}

func (s *Service) runListener(account model.Account) {
	defer s.listenerWG.Done()
	for {
		err := s.listener.Run(s.ctx, account, func(event Event) error {
			return s.Enqueue(s.ctx, event)
		})
		if s.ctx.Err() != nil {
			return
		}
		if err == nil {
			return
		}
		s.logger.Warn("telegram update listener stopped", zap.Int64("account_id", account.ID), zap.Error(err))
		if s.accounts != nil {
			if updateErr := s.accounts.UpdateStatus(s.ctx, account.ID, model.AccountStatusReconnecting); updateErr != nil {
				s.logger.Error("mark account reconnecting", zap.Int64("account_id", account.ID), zap.Error(updateErr))
			}
		}
		timer := time.NewTimer(s.retryInterval)
		select {
		case <-s.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func wait(ctx context.Context, wg *sync.WaitGroup) error {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
