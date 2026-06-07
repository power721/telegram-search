# Phase 3 Multi-Account Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the Phase 3 multi-account lifecycle manager, explicit account state handling, account health recovery, account-aware status summaries, and isolation tests.

**Architecture:** Add `internal/account` as the owner of account lifecycle while keeping `internal/update` responsible for realtime listeners and queues. Repositories remain storage-focused; lifecycle transition policy lives in `internal/account`. Existing account filters in search, latest, links, and channels stay in place and receive stronger tests.

**Tech Stack:** Go, SQLite, gin, zap, gotd adapter interfaces, existing repository/search/channel/update packages.

---

## File Structure

- Create `internal/account/state.go`: known account status list and transition table.
- Create `internal/account/state_test.go`: state machine tests.
- Create `internal/account/manager.go`: account lifecycle manager, restart, health check loop, update runtime interface.
- Create `internal/account/manager_test.go`: lifecycle and health check tests with fake update runtime.
- Modify `internal/model/model.go`: add `AccountStates map[string]int64` to `StatusCounts`.
- Modify `internal/repository/status.go`: count accounts grouped by state.
- Modify `internal/repository/repository_test.go`: verify state counts and account-filtered repository searches.
- Modify `internal/api/handlers.go`: add `account_states` to `/api/status`.
- Modify `internal/api/handlers_test.go`: verify status state summary, search account filter, channel account filter.
- Create `internal/channel/service_test.go`: verify channel sync uses each account session and saves account-scoped channels.
- Modify `cmd/tg-provider/main.go`: replace direct update-service account startup with `account.Manager`.

## Task 1: Account State Machine

**Files:**
- Create: `internal/account/state_test.go`
- Create: `internal/account/state.go`

- [ ] **Step 1: Write failing state-machine tests**

Create `internal/account/state_test.go`:

```go
package account

import (
	"testing"

	"tg-provider/internal/model"
)

func TestKnownStatus(t *testing.T) {
	for _, status := range []string{
		model.AccountStatusNew,
		model.AccountStatusLoginRequired,
		model.AccountStatusSyncing,
		model.AccountStatusOnline,
		model.AccountStatusReconnecting,
		model.AccountStatusFloodWait,
		model.AccountStatusDisconnected,
	} {
		if !KnownStatus(status) {
			t.Fatalf("KnownStatus(%q) = false, want true", status)
		}
	}
	if KnownStatus("BROKEN") {
		t.Fatal("KnownStatus(BROKEN) = true, want false")
	}
}

func TestCanTransition(t *testing.T) {
	valid := []struct {
		from string
		to   string
	}{
		{model.AccountStatusNew, model.AccountStatusLoginRequired},
		{model.AccountStatusLoginRequired, model.AccountStatusOnline},
		{model.AccountStatusOnline, model.AccountStatusSyncing},
		{model.AccountStatusOnline, model.AccountStatusReconnecting},
		{model.AccountStatusSyncing, model.AccountStatusOnline},
		{model.AccountStatusReconnecting, model.AccountStatusDisconnected},
		{model.AccountStatusFloodWait, model.AccountStatusReconnecting},
		{model.AccountStatusDisconnected, model.AccountStatusReconnecting},
		{model.AccountStatusOnline, model.AccountStatusOnline},
	}
	for _, tc := range valid {
		if !CanTransition(tc.from, tc.to) {
			t.Fatalf("CanTransition(%q, %q) = false, want true", tc.from, tc.to)
		}
	}

	invalid := []struct {
		from string
		to   string
	}{
		{model.AccountStatusNew, model.AccountStatusOnline},
		{model.AccountStatusLoginRequired, model.AccountStatusSyncing},
		{model.AccountStatusDisconnected, model.AccountStatusOnline},
		{model.AccountStatusOnline, "BROKEN"},
		{"BROKEN", model.AccountStatusOnline},
	}
	for _, tc := range invalid {
		if CanTransition(tc.from, tc.to) {
			t.Fatalf("CanTransition(%q, %q) = true, want false", tc.from, tc.to)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/account
```

Expected: FAIL because package `internal/account` or functions `KnownStatus` and `CanTransition` do not exist.

- [ ] **Step 3: Implement state machine**

Create `internal/account/state.go`:

```go
package account

import "tg-provider/internal/model"

var knownStatuses = map[string]struct{}{
	model.AccountStatusNew:           {},
	model.AccountStatusLoginRequired: {},
	model.AccountStatusSyncing:       {},
	model.AccountStatusOnline:        {},
	model.AccountStatusReconnecting:  {},
	model.AccountStatusFloodWait:     {},
	model.AccountStatusDisconnected:  {},
}

var allowedTransitions = map[string]map[string]struct{}{
	model.AccountStatusNew: {
		model.AccountStatusLoginRequired: {},
	},
	model.AccountStatusLoginRequired: {
		model.AccountStatusOnline:       {},
		model.AccountStatusDisconnected: {},
	},
	model.AccountStatusOnline: {
		model.AccountStatusSyncing:      {},
		model.AccountStatusReconnecting: {},
		model.AccountStatusFloodWait:    {},
		model.AccountStatusDisconnected: {},
	},
	model.AccountStatusSyncing: {
		model.AccountStatusOnline:       {},
		model.AccountStatusFloodWait:    {},
		model.AccountStatusDisconnected: {},
	},
	model.AccountStatusReconnecting: {
		model.AccountStatusOnline:       {},
		model.AccountStatusFloodWait:    {},
		model.AccountStatusDisconnected: {},
	},
	model.AccountStatusFloodWait: {
		model.AccountStatusReconnecting: {},
		model.AccountStatusDisconnected: {},
	},
	model.AccountStatusDisconnected: {
		model.AccountStatusReconnecting: {},
	},
}

func KnownStatus(status string) bool {
	_, ok := knownStatuses[status]
	return ok
}

func CanTransition(from string, to string) bool {
	if !KnownStatus(from) || !KnownStatus(to) {
		return false
	}
	if from == to {
		return true
	}
	targets, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	_, ok = targets[to]
	return ok
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/account
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/account/state.go internal/account/state_test.go
git commit -m "feat: add account state machine"
```

## Task 2: Account Manager Lifecycle

**Files:**
- Create: `internal/account/manager_test.go`
- Create: `internal/account/manager.go`

- [ ] **Step 1: Write failing lifecycle tests**

Create `internal/account/manager_test.go` with these tests and helper types:

```go
package account

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"tg-provider/internal/db"
	"tg-provider/internal/model"
	"tg-provider/internal/repository"
)

func TestManagerStartsOnlineAndReconnectingAccountsOnly(t *testing.T) {
	ctx := context.Background()
	fixture := newManagerFixture(t)
	onlineID := fixture.saveAccount(t, model.AccountStatusOnline)
	reconnectingID := fixture.saveAccount(t, model.AccountStatusReconnecting)
	fixture.saveAccount(t, model.AccountStatusLoginRequired)
	fixture.saveAccount(t, model.AccountStatusDisconnected)

	runtime := &recordingUpdateRuntime{}
	manager := NewManager(ManagerOptions{
		Accounts:       fixture.accounts,
		Updates:        runtime,
		HealthInterval: time.Hour,
	})

	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	defer manager.Stop(context.Background())

	got := runtime.startedIDs()
	want := []int64{onlineID, reconnectingID}
	if !sameIDs(got, want) {
		t.Fatalf("started ids = %v, want %v", got, want)
	}
	if runtime.startCalls() != 1 {
		t.Fatalf("update runtime Start calls = %d, want 1", runtime.startCalls())
	}
}

func TestManagerStartIsIdempotent(t *testing.T) {
	ctx := context.Background()
	fixture := newManagerFixture(t)
	onlineID := fixture.saveAccount(t, model.AccountStatusOnline)
	runtime := &recordingUpdateRuntime{}
	manager := NewManager(ManagerOptions{
		Accounts:       fixture.accounts,
		Updates:        runtime,
		HealthInterval: time.Hour,
	})

	if err := manager.Start(ctx); err != nil {
		t.Fatalf("first Start returned error: %v", err)
	}
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("second Start returned error: %v", err)
	}
	defer manager.Stop(context.Background())

	if got := runtime.startedIDs(); !sameIDs(got, []int64{onlineID}) {
		t.Fatalf("started ids = %v, want [%d]", got, onlineID)
	}
}

func TestManagerRestartTransitionsAndStartsAccount(t *testing.T) {
	ctx := context.Background()
	fixture := newManagerFixture(t)
	accountID := fixture.saveAccount(t, model.AccountStatusDisconnected)
	runtime := &recordingUpdateRuntime{}
	manager := NewManager(ManagerOptions{
		Accounts:       fixture.accounts,
		Updates:        runtime,
		HealthInterval: time.Hour,
	})
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	defer manager.Stop(context.Background())

	if err := manager.Restart(ctx, accountID); err != nil {
		t.Fatalf("Restart returned error: %v", err)
	}
	account, err := fixture.accounts.FindByID(ctx, accountID)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if account.Status != model.AccountStatusReconnecting {
		t.Fatalf("status = %q, want RECONNECTING", account.Status)
	}
	if got := runtime.startedIDs(); !sameIDs(got, []int64{accountID}) {
		t.Fatalf("started ids = %v, want [%d]", got, accountID)
	}
}

func TestManagerRejectsInvalidTransition(t *testing.T) {
	ctx := context.Background()
	fixture := newManagerFixture(t)
	accountID := fixture.saveAccount(t, model.AccountStatusNew)
	manager := NewManager(ManagerOptions{
		Accounts:       fixture.accounts,
		Updates:        &recordingUpdateRuntime{},
		HealthInterval: time.Hour,
	})

	account, err := fixture.accounts.FindByID(ctx, accountID)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if err := manager.TransitionStatus(ctx, account, model.AccountStatusOnline); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("TransitionStatus error = %v, want ErrInvalidTransition", err)
	}
	unchanged, err := fixture.accounts.FindByID(ctx, accountID)
	if err != nil {
		t.Fatalf("FindByID after transition returned error: %v", err)
	}
	if unchanged.Status != model.AccountStatusNew {
		t.Fatalf("status = %q, want NEW", unchanged.Status)
	}
}

func TestManagerHealthCheckRestartsDisconnectedAccount(t *testing.T) {
	ctx := context.Background()
	fixture := newManagerFixture(t)
	accountID := fixture.saveAccount(t, model.AccountStatusDisconnected)
	runtime := &recordingUpdateRuntime{}
	manager := NewManager(ManagerOptions{
		Accounts:       fixture.accounts,
		Updates:        runtime,
		HealthInterval: time.Hour,
	})
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	defer manager.Stop(context.Background())

	if err := manager.CheckHealth(ctx); err != nil {
		t.Fatalf("CheckHealth returned error: %v", err)
	}
	account, err := fixture.accounts.FindByID(ctx, accountID)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if account.Status != model.AccountStatusReconnecting {
		t.Fatalf("status = %q, want RECONNECTING", account.Status)
	}
	if got := runtime.startedIDs(); !sameIDs(got, []int64{accountID}) {
		t.Fatalf("started ids = %v, want [%d]", got, accountID)
	}
}

type managerFixture struct {
	accounts *repository.AccountRepository
}

func newManagerFixture(t *testing.T) managerFixture {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(context.Background(), conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	return managerFixture{accounts: repository.NewAccountRepository(conn)}
}

func (f managerFixture) saveAccount(t *testing.T, status string) int64 {
	t.Helper()
	id, err := f.accounts.Save(context.Background(), model.Account{
		Phone:  "+" + status + time.Now().Format("150405.000000000"),
		Status: status,
	})
	if err != nil {
		t.Fatalf("save account %s: %v", status, err)
	}
	return id
}

type recordingUpdateRuntime struct {
	mu        sync.Mutex
	starts    int
	stops     int
	started   []int64
	startErr  error
	stopError error
}

func (r *recordingUpdateRuntime) Start(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.starts++
}

func (r *recordingUpdateRuntime) StartAccount(ctx context.Context, account model.Account) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.startErr != nil {
		return r.startErr
	}
	r.started = append(r.started, account.ID)
	return nil
}

func (r *recordingUpdateRuntime) Stop(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stops++
	return r.stopError
}

func (r *recordingUpdateRuntime) startedIDs() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]int64, len(r.started))
	copy(out, r.started)
	return out
}

func (r *recordingUpdateRuntime) startCalls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.starts
}

func sameIDs(got []int64, want []int64) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[int64]int{}
	for _, id := range got {
		seen[id]++
	}
	for _, id := range want {
		seen[id]--
	}
	for _, count := range seen {
		if count != 0 {
			return false
		}
	}
	return true
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/account
```

Expected: FAIL because `Manager`, `ManagerOptions`, `ErrInvalidTransition`, and lifecycle methods do not exist.

- [ ] **Step 3: Implement manager**

Create `internal/account/manager.go`:

```go
package account

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"tg-provider/internal/model"
	"tg-provider/internal/repository"
)

var ErrInvalidTransition = errors.New("invalid account status transition")

type UpdateRuntime interface {
	Start(context.Context)
	StartAccount(context.Context, model.Account) error
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/account
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/account/manager.go internal/account/manager_test.go
git commit -m "feat: add account lifecycle manager"
```

## Task 3: Account-Aware Status Summary

**Files:**
- Modify: `internal/model/model.go`
- Modify: `internal/repository/status.go`
- Modify: `internal/repository/repository_test.go`
- Modify: `internal/api/handlers.go`
- Modify: `internal/api/handlers_test.go`

- [ ] **Step 1: Write failing repository and API status tests**

In `internal/repository/repository_test.go`, extend `TestRepositoriesPersistSearchAndCountData` after the existing `counts` assertion with:

```go
	secondAccountID, err := accounts.Save(ctx, model.Account{Phone: "+10000000001", Status: model.AccountStatusReconnecting})
	if err != nil {
		t.Fatalf("save reconnecting account: %v", err)
	}
	if secondAccountID == 0 {
		t.Fatal("second account id = 0")
	}
	counts, err = status.Counts(ctx)
	if err != nil {
		t.Fatalf("counts with states: %v", err)
	}
	if counts.AccountStates[model.AccountStatusOnline] != 1 || counts.AccountStates[model.AccountStatusReconnecting] != 1 {
		t.Fatalf("account state counts = %+v, want ONLINE=1 RECONNECTING=1", counts.AccountStates)
	}
```

In `internal/api/handlers_test.go`, add:

```go
func TestStatusIncludesAccountStateSummary(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	_, _ = deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	_, _ = deps.Accounts.Save(ctx, model.Account{Phone: "+10000000001", Status: model.AccountStatusReconnecting})
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		AccountStates map[string]int64 `json:"account_states"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.AccountStates[model.AccountStatusOnline] != 1 || body.AccountStates[model.AccountStatusReconnecting] != 1 {
		t.Fatalf("account_states = %+v, want ONLINE=1 RECONNECTING=1", body.AccountStates)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/repository ./internal/api
```

Expected: FAIL because `model.StatusCounts` has no `AccountStates` field and `/api/status` does not return `account_states`.

- [ ] **Step 3: Implement state counts**

Modify `internal/model/model.go`:

```go
type StatusCounts struct {
	Accounts      int64            `json:"accounts"`
	Channels      int64            `json:"channels"`
	Messages      int64            `json:"messages"`
	Links         int64            `json:"links"`
	AccountStates map[string]int64 `json:"account_states"`
}
```

Modify `internal/repository/status.go` by initializing and loading grouped states at the end of `Counts`:

```go
	counts.AccountStates = map[string]int64{}
	rows, err := r.db.QueryContext(ctx, `SELECT status, count(*) FROM telegram_accounts GROUP BY status`)
	if err != nil {
		return model.StatusCounts{}, fmt.Errorf("read account state counts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return model.StatusCounts{}, err
		}
		counts.AccountStates[status] = count
	}
	if err := rows.Err(); err != nil {
		return model.StatusCounts{}, err
	}
```

Modify `internal/api/handlers.go` status response:

```go
	c.JSON(http.StatusOK, gin.H{
		"service":        "ok",
		"accounts":       counts.Accounts,
		"channels":       counts.Channels,
		"messages":       counts.Messages,
		"links":          counts.Links,
		"account_states": counts.AccountStates,
	})
```

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/repository ./internal/api
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/model/model.go internal/repository/status.go internal/repository/repository_test.go internal/api/handlers.go internal/api/handlers_test.go
git commit -m "feat: add account state status summary"
```

## Task 4: Multi-Account Isolation Tests

**Files:**
- Modify: `internal/repository/repository_test.go`
- Modify: `internal/api/handlers_test.go`
- Create: `internal/channel/service_test.go`

- [ ] **Step 1: Write failing or strengthening isolation tests**

Add a new test to `internal/repository/repository_test.go`:

```go
func TestRepositoriesKeepAccountDataIsolated(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	accounts := NewAccountRepository(conn)
	channels := NewChannelRepository(conn)
	messages := NewMessageRepository(conn)
	links := NewLinkRepository(conn)

	account1, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Username: "one", Status: model.AccountStatusOnline})
	account2, _ := accounts.Save(ctx, model.Account{Phone: "+10000000001", Username: "two", Status: model.AccountStatusOnline})
	channel1, _ := channels.Save(ctx, model.Channel{AccountID: account1, TelegramChannelID: 100, Title: "A1", Type: model.ChannelTypeChannel})
	channel2, _ := channels.Save(ctx, model.Channel{AccountID: account2, TelegramChannelID: 100, Title: "A2", Type: model.ChannelTypeChannel})
	now := time.Now().UTC()
	stored1, _ := messages.SaveBatch(ctx, []model.Message{{AccountID: account1, ChannelID: channel1, TelegramMessageID: 1, Text: "shared keyword account one", RawJSON: "{}", Date: now}})
	stored2, _ := messages.SaveBatch(ctx, []model.Message{{AccountID: account2, ChannelID: channel2, TelegramMessageID: 1, Text: "shared keyword account two", RawJSON: "{}", Date: now}})
	_, _ = links.SaveBatch(ctx, stored1[0].ID, []model.Link{{Type: "url", URL: "https://example.com/one"}})
	_, _ = links.SaveBatch(ctx, stored2[0].ID, []model.Link{{Type: "url", URL: "https://example.com/two"}})

	results, err := messages.Search(ctx, SearchParams{Query: "shared", AccountID: account1, Limit: 10})
	if err != nil {
		t.Fatalf("search account1: %v", err)
	}
	if len(results) != 1 || results[0].AccountID != account1 || results[0].AccountUsername != "one" {
		t.Fatalf("account1 search results = %+v", results)
	}
	latest, err := messages.Latest(ctx, LatestParams{AccountID: account2, Limit: 10})
	if err != nil {
		t.Fatalf("latest account2: %v", err)
	}
	if len(latest) != 1 || latest[0].AccountID != account2 {
		t.Fatalf("account2 latest = %+v", latest)
	}
	linkResults, err := links.Search(ctx, LinkSearchParams{AccountID: account1, Limit: 10})
	if err != nil {
		t.Fatalf("links account1: %v", err)
	}
	if len(linkResults) != 1 || linkResults[0].AccountID != account1 || linkResults[0].URL != "https://example.com/one" {
		t.Fatalf("account1 links = %+v", linkResults)
	}
}
```

Add a new test to `internal/api/handlers_test.go`:

```go
func TestReadAPIsFilterByAccount(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	account1, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Username: "one", Status: model.AccountStatusOnline})
	account2, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000001", Username: "two", Status: model.AccountStatusOnline})
	channel1, _ := deps.Channels.Save(ctx, model.Channel{AccountID: account1, TelegramChannelID: 1, Title: "one-channel", Type: model.ChannelTypeChannel})
	channel2, _ := deps.Channels.Save(ctx, model.Channel{AccountID: account2, TelegramChannelID: 2, Title: "two-channel", Type: model.ChannelTypeChannel})
	now := time.Now().UTC()
	stored1, _ := deps.Messages.SaveBatch(ctx, []model.Message{{AccountID: account1, ChannelID: channel1, TelegramMessageID: 1, Text: "shared title one", RawJSON: "{}", Date: now}})
	stored2, _ := deps.Messages.SaveBatch(ctx, []model.Message{{AccountID: account2, ChannelID: channel2, TelegramMessageID: 2, Text: "shared title two", RawJSON: "{}", Date: now}})
	_, _ = deps.Links.SaveBatch(ctx, stored1[0].ID, []model.Link{{Type: "url", URL: "https://example.com/one"}})
	_, _ = deps.Links.SaveBatch(ctx, stored2[0].ID, []model.Link{{Type: "url", URL: "https://example.com/two"}})
	router := NewRouter(deps)

	for _, path := range []string{
		"/api/search?q=shared&account_id=" + strconv.FormatInt(account1, 10),
		"/api/messages/latest?account_id=" + strconv.FormatInt(account1, 10),
		"/api/links?account_id=" + strconv.FormatInt(account1, 10),
		"/api/channels?account_id=" + strconv.FormatInt(account1, 10),
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s status = %d body=%s", path, w.Code, w.Body.String())
		}
		if !bytes.Contains(w.Body.Bytes(), []byte("one")) {
			t.Fatalf("%s response missing account one data: %s", path, w.Body.String())
		}
		if bytes.Contains(w.Body.Bytes(), []byte("two")) || bytes.Contains(w.Body.Bytes(), []byte("https://example.com/two")) {
			t.Fatalf("%s response leaked account two data: %s", path, w.Body.String())
		}
	}
}
```

Because this API test uses `strconv`, ensure `internal/api/handlers_test.go` imports it.

Create `internal/channel/service_test.go`:

```go
package channel

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"tg-provider/internal/db"
	"tg-provider/internal/model"
	"tg-provider/internal/repository"
	"tg-provider/internal/session"
	"tg-provider/internal/telegram"
)

func TestServiceSyncAccountKeepsChannelsIsolated(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	accounts := repository.NewAccountRepository(conn)
	channels := repository.NewChannelRepository(conn)
	account1, _ := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	account2, _ := accounts.Save(ctx, model.Account{Phone: "+10000000001", Status: model.AccountStatusOnline})
	client := &recordingChannelClient{
		items: map[int64][]telegram.Channel{
			account1: {{TelegramChannelID: 100, Title: "One", Type: model.ChannelTypeChannel}},
			account2: {{TelegramChannelID: 100, Title: "Two", Type: model.ChannelTypeChannel}},
		},
	}
	service := NewService(channels, client, session.NewManager(filepath.Join(t.TempDir(), "sessions")))

	a1, _ := accounts.FindByID(ctx, account1)
	a2, _ := accounts.FindByID(ctx, account2)
	if _, err := service.SyncAccount(ctx, a1); err != nil {
		t.Fatalf("sync account1: %v", err)
	}
	if _, err := service.SyncAccount(ctx, a2); err != nil {
		t.Fatalf("sync account2: %v", err)
	}

	account1Channels, err := channels.FindByAccountID(ctx, account1)
	if err != nil {
		t.Fatalf("find account1 channels: %v", err)
	}
	account2Channels, err := channels.FindByAccountID(ctx, account2)
	if err != nil {
		t.Fatalf("find account2 channels: %v", err)
	}
	if len(account1Channels) != 1 || account1Channels[0].Title != "One" {
		t.Fatalf("account1 channels = %+v", account1Channels)
	}
	if len(account2Channels) != 1 || account2Channels[0].Title != "Two" {
		t.Fatalf("account2 channels = %+v", account2Channels)
	}
	if !client.sawSession(account1) || !client.sawSession(account2) {
		t.Fatalf("client sessions = %+v, want both account ids", client.sessions)
	}
}

type recordingChannelClient struct {
	telegram.NopClient
	mu       sync.Mutex
	items    map[int64][]telegram.Channel
	sessions []telegram.AccountSession
}

func (c *recordingChannelClient) ListChannels(ctx context.Context, session telegram.AccountSession) ([]telegram.Channel, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessions = append(c.sessions, session)
	return c.items[session.AccountID], nil
}

func (c *recordingChannelClient) sawSession(accountID int64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, session := range c.sessions {
		if session.AccountID == accountID && session.SessionPath != "" {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run tests**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/repository ./internal/api ./internal/channel
```

Expected: repository/search tests should pass if existing filters are correct; new API/channel tests may fail if imports or response assumptions need implementation adjustments.

- [ ] **Step 3: Make minimal implementation adjustments**

If `internal/api/handlers_test.go` lacks `strconv`, add it to the import block:

```go
import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)
```

If no production code changes are needed, keep this task as test-only coverage.

- [ ] **Step 4: Re-run tests**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/repository ./internal/api ./internal/channel
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repository/repository_test.go internal/api/handlers_test.go internal/channel/service_test.go
git commit -m "test: cover multi-account isolation"
```

## Task 5: Main Startup Wiring

**Files:**
- Modify: `cmd/tg-provider/main.go`

- [ ] **Step 1: Write compile-oriented failing change expectation**

Run current build before edits:

```bash
GOCACHE=/tmp/go-build-cache go build ./...
```

Expected: PASS before wiring changes. This establishes baseline.

- [ ] **Step 2: Replace direct update startup with account manager**

Modify `cmd/tg-provider/main.go` imports:

```go
	"tg-provider/internal/account"
```

After constructing `channelService`, construct and start the manager:

```go
	accountManager := account.NewManager(account.ManagerOptions{
		Accounts: accounts,
		Updates:  updateService,
		Logger:   logs.Telegram,
	})
	if err := accountManager.Start(ctx); err != nil {
		return err
	}
```

Remove this direct Phase 2 startup block:

```go
	updateService.Start(ctx)
	onlineAccounts, err := accounts.FindAll(ctx)
	if err != nil {
		return err
	}
	for _, account := range onlineAccounts {
		if account.Status != "ONLINE" {
			continue
		}
		if err := updateService.StartAccount(ctx, account); err != nil {
			return err
		}
	}
```

During shutdown, replace:

```go
	if err := updateService.Stop(shutdownCtx); err != nil {
		return err
	}
```

with:

```go
	if err := accountManager.Stop(shutdownCtx); err != nil {
		return err
	}
```

- [ ] **Step 3: Run build to verify wiring**

Run:

```bash
GOCACHE=/tmp/go-build-cache go build ./...
```

Expected: PASS.

- [ ] **Step 4: Run account and command tests**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/account ./cmd/tg-provider
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/tg-provider/main.go
git commit -m "feat: wire account manager startup"
```

## Task 6: Final Verification

**Files:**
- Verify all modified files.

- [ ] **Step 1: Run all tests**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./...
```

Expected: PASS for every package.

- [ ] **Step 2: Run build**

Run:

```bash
GOCACHE=/tmp/go-build-cache go build ./...
```

Expected: PASS with exit code 0.

- [ ] **Step 3: Check git status**

Run:

```bash
git status --short --branch
```

Expected: on `phase-3-multi-account` with no uncommitted changes.

- [ ] **Step 4: Review recent commits**

Run:

```bash
git log --oneline --decorate -8
```

Expected: shows Phase 3 design plus implementation commits on `phase-3-multi-account`.

## Self-Review

Spec coverage:

- Task 046 is covered by Task 2.
- Task 047 is covered by Task 1 and invalid transition tests in Task 2.
- Task 048 is covered by Task 2 startup tests and Task 5 main wiring.
- Task 049 is covered by Task 2 health check tests.
- Task 050 is covered by Task 4 channel sync and API channel filter tests.
- Task 051 is covered by Task 4 repository and API search filter tests.
- Task 052 is covered by Task 3.
- Task 053 is covered by Tasks 1-4.

Placeholder scan: no step depends on unspecified future work.

Type consistency: `ManagerOptions`, `UpdateRuntime`, `TransitionStatus`, `CheckHealth`, `StatusCounts.AccountStates`, and API response key `account_states` are defined before later tasks use them.
