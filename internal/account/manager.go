package account

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"tg-search/internal/model"
	"tg-search/internal/repository"
)

var ErrInvalidTransition = errors.New("invalid account status transition")

type UpdateRuntime interface {
	Start(context.Context)
	StartAccount(context.Context, model.Account) error
	StopAccount(context.Context, int64) error
	Stop(context.Context) error
}

type ManagerOptions struct {
	Accounts       *repository.AccountRepository
	Updates        UpdateRuntime
	HealthInterval time.Duration
	Logger         *zap.Logger
}

type Manager struct {
	accounts       *repository.AccountRepository
	updates        UpdateRuntime
	healthInterval time.Duration
	logger         *zap.Logger

	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	started bool
	active  map[int64]struct{}
	wg      sync.WaitGroup
}

func NewManager(opts ManagerOptions) *Manager {
	if opts.HealthInterval <= 0 {
		opts.HealthInterval = time.Minute
	}
	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}
	return &Manager{
		accounts:       opts.Accounts,
		updates:        opts.Updates,
		healthInterval: opts.HealthInterval,
		logger:         opts.Logger,
		active:         map[int64]struct{}{},
	}
}

func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return nil
	}
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.started = true
	if m.updates != nil {
		m.updates.Start(m.ctx)
	}
	m.mu.Unlock()

	accounts, err := m.List(ctx)
	if err != nil {
		return err
	}
	for _, account := range accounts {
		if shouldAutoStart(account.Status) {
			if err := m.startAccount(ctx, account); err != nil {
				return err
			}
		}
	}

	m.wg.Add(1)
	go m.healthLoop()
	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return nil
	}
	m.started = false
	if m.cancel != nil {
		m.cancel()
	}
	m.mu.Unlock()

	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	if m.updates != nil {
		if err := m.updates.Stop(ctx); err != nil {
			return err
		}
	}
	m.mu.Lock()
	m.active = map[int64]struct{}{}
	m.mu.Unlock()
	return nil
}

func (m *Manager) Restart(ctx context.Context, accountID int64) error {
	account, err := m.accounts.FindByID(ctx, accountID)
	if err != nil {
		return err
	}
	if err := m.TransitionStatus(ctx, account, model.AccountStatusReconnecting); err != nil {
		return err
	}
	account.Status = model.AccountStatusReconnecting
	return m.startAccount(ctx, account)
}

func (m *Manager) StopAccount(ctx context.Context, accountID int64) error {
	m.mu.Lock()
	_, active := m.active[accountID]
	if active {
		delete(m.active, accountID)
	}
	m.mu.Unlock()

	if active && m.updates != nil {
		if err := m.updates.StopAccount(ctx, accountID); err != nil {
			return err
		}
	}
	if m.accounts != nil {
		if err := m.accounts.UpdateStatus(ctx, accountID, model.AccountStatusDisconnected); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) List(ctx context.Context) ([]model.Account, error) {
	if m.accounts == nil {
		return nil, nil
	}
	return m.accounts.FindAll(ctx)
}

func (m *Manager) TransitionStatus(ctx context.Context, account model.Account, to string) error {
	if !CanTransition(account.Status, to) {
		err := fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, account.Status, to)
		m.logger.Warn("invalid account status transition", zap.Int64("account_id", account.ID), zap.String("from", account.Status), zap.String("to", to), zap.Error(err))
		return err
	}
	if m.accounts == nil {
		return nil
	}
	return m.accounts.UpdateStatus(ctx, account.ID, to)
}

func (m *Manager) CheckHealth(ctx context.Context) error {
	accounts, err := m.List(ctx)
	if err != nil {
		return err
	}
	for _, account := range accounts {
		switch account.Status {
		case model.AccountStatusReconnecting:
			if err := m.startAccount(ctx, account); err != nil {
				m.logger.Warn("restart reconnecting account", zap.Int64("account_id", account.ID), zap.Error(err))
			}
		case model.AccountStatusDisconnected:
			if err := m.Restart(ctx, account.ID); err != nil {
				m.logger.Warn("restart disconnected account", zap.Int64("account_id", account.ID), zap.Error(err))
			}
		}
	}
	return nil
}

func (m *Manager) healthLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(m.healthInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if err := m.CheckHealth(m.ctx); err != nil {
				m.logger.Warn("account health check", zap.Error(err))
			}
		}
	}
}

func (m *Manager) startAccount(ctx context.Context, account model.Account) error {
	m.mu.Lock()
	if _, ok := m.active[account.ID]; ok {
		m.mu.Unlock()
		return nil
	}
	m.active[account.ID] = struct{}{}
	m.mu.Unlock()

	if m.updates == nil {
		return nil
	}
	if err := m.updates.StartAccount(ctx, account); err != nil {
		m.mu.Lock()
		delete(m.active, account.ID)
		m.mu.Unlock()
		return err
	}
	return nil
}

func shouldAutoStart(status string) bool {
	return status == model.AccountStatusOnline || status == model.AccountStatusReconnecting
}
