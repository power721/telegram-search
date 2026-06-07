# Phase 5b Runtime Reliability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build Phase 5b runtime reliability: history sync worker pools, duplicate-channel protection, retry/backoff, FloodWait handling, and a cleanup scheduler skeleton.

**Architecture:** Add a focused `internal/retry` package that classifies temporary, permanent, and FloodWait errors and exposes deterministic backoff behavior. Keep history sync storage logic in `internal/history`, add worker orchestration and channel locking there, reuse the retry classifier in `internal/update`, and add a small lifecycle-managed `internal/scheduler` package. Keep persistence unchanged and preserve the existing single-channel sync API.

**Tech Stack:** Go, SQLite, Gin, zap, standard-library goroutines/channels/context/testing.

---

## File Structure

- Create `internal/retry/policy.go`: typed retry wrappers, FloodWait parsing, retry policy, delay calculation, and context-aware sleep.
- Create `internal/retry/policy_test.go`: deterministic classifier and policy tests.
- Modify `internal/history/service.go`: add `Workers`, `RetryPolicy`, channel lock map, `SyncMany`, and retry-wrapped `SyncChannel`.
- Modify `internal/history/service_test.go`: add fake Telegram clients for retry, FloodWait, worker limit, and same-channel skip coverage.
- Modify `internal/api/router.go`: add `POST /api/channels/sync` before `POST /api/channels/:id/sync`.
- Modify `internal/api/handlers.go`: add batch sync request validation and response handling.
- Modify `internal/api/handlers_test.go`: add batch sync API validation and success tests.
- Modify `internal/update/service.go`: reuse retry classification and policy in the listener restart loop.
- Modify `internal/update/service_test.go`: add FloodWait listener retry test with deterministic sleep.
- Create `internal/scheduler/scheduler.go`: lifecycle-managed job runner.
- Create `internal/scheduler/cleanup.go`: cleanup job skeleton that only logs activity.
- Create `internal/scheduler/scheduler_test.go`: scheduler run/stop behavior and failed-job continuation.
- Modify `cmd/tg-provider/main.go`: wire `cfg.Sync.Workers`, retry defaults, and cleanup scheduler start/stop.

---

### Task 1: Retry Policy And FloodWait Classification

**Files:**
- Create: `internal/retry/policy_test.go`
- Create: `internal/retry/policy.go`

- [ ] **Step 1: Write the failing classifier tests**

Create `internal/retry/policy_test.go`:

```go
package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestClassifyFloodWaitFromTypedAndTextErrors(t *testing.T) {
	typed := Classify(FloodWait(45, errors.New("rpc flood")))
	if typed.Kind != KindFloodWait || typed.Wait != 45*time.Second {
		t.Fatalf("typed classification = %+v, want flood wait 45s", typed)
	}

	text := Classify(errors.New("rpc error: FLOOD_WAIT_60"))
	if text.Kind != KindFloodWait || text.Wait != time.Minute {
		t.Fatalf("text classification = %+v, want flood wait 1m", text)
	}
}

func TestPolicyRetriesTemporaryWithExponentialBackoffAndCap(t *testing.T) {
	var slept []time.Duration
	policy := Policy{
		BaseDelay: 10 * time.Millisecond,
		MaxDelay:  25 * time.Millisecond,
		MaxTries:  4,
		Sleep: func(ctx context.Context, d time.Duration) error {
			slept = append(slept, d)
			return nil
		},
	}
	attempts := 0
	err := policy.Run(context.Background(), func() error {
		attempts++
		if attempts < 4 {
			return Temporary(errors.New("network"))
		}
		return nil
	}, nil)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if attempts != 4 {
		t.Fatalf("attempts = %d, want 4", attempts)
	}
	want := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 25 * time.Millisecond}
	if len(slept) != len(want) {
		t.Fatalf("slept = %v, want %v", slept, want)
	}
	for i := range want {
		if slept[i] != want[i] {
			t.Fatalf("slept = %v, want %v", slept, want)
		}
	}
}

func TestPolicyStopsOnPermanentError(t *testing.T) {
	policy := Policy{
		BaseDelay: time.Millisecond,
		MaxDelay:  time.Millisecond,
		MaxTries:  3,
		Sleep: func(context.Context, time.Duration) error {
			t.Fatal("Sleep called for permanent error")
			return nil
		},
	}
	attempts := 0
	err := policy.Run(context.Background(), func() error {
		attempts++
		return Permanent(errors.New("bad request"))
	}, nil)
	if err == nil {
		t.Fatal("Run returned nil error, want permanent failure")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}
```

- [ ] **Step 2: Run the retry tests to verify they fail**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/retry
```

Expected: FAIL because package `tg-provider/internal/retry` does not exist.

- [ ] **Step 3: Implement the retry package**

Create `internal/retry/policy.go`:

```go
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
```

- [ ] **Step 4: Run the retry tests to verify they pass**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/retry
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/retry/policy.go internal/retry/policy_test.go
git commit -m "feat: add retry policy"
```

---

### Task 2: History Retry And FloodWait Account State

**Files:**
- Modify: `internal/history/service.go`
- Modify: `internal/history/service_test.go`

- [ ] **Step 1: Write failing history retry tests**

Append these tests and helpers to `internal/history/service_test.go`:

```go
func TestSyncChannelRetriesTemporaryFetchError(t *testing.T) {
	ctx := context.Background()
	conn, accounts, channels, messages, links := setupHistoryTestStore(t)
	accountID, channelID := seedHistoryAccountAndChannel(t, ctx, accounts, channels)
	_ = accountID

	now := time.Now().UTC()
	fake := &retryingHistoryClient{
		failuresBeforeSuccess: 1,
		successBatch: []telegram.Message{
			{TelegramMessageID: 7, SenderID: 1, Text: "retry success", RawJSON: "{}", Date: now},
		},
	}
	service := NewService(Options{
		DB: conn, Accounts: accounts, Channels: channels, Messages: messages, Links: links,
		Telegram: fake, Sessions: session.NewManager(filepath.Join(t.TempDir(), "sessions")),
		Extractor: link.NewExtractor(), HistoryBatchSize: 10,
		RetryPolicy: retry.Policy{
			BaseDelay: time.Millisecond,
			MaxDelay:  time.Millisecond,
			MaxTries:  3,
			Sleep:     func(context.Context, time.Duration) error { return nil },
		},
	})

	result, err := service.SyncChannel(ctx, channelID)
	if err != nil {
		t.Fatalf("SyncChannel returned error: %v", err)
	}
	if result.Messages != 1 {
		t.Fatalf("messages = %d, want 1", result.Messages)
	}
	if fake.calls != 2 {
		t.Fatalf("fetch calls = %d, want 2", fake.calls)
	}
}

func TestSyncChannelMarksAccountFloodWaitBeforeRetry(t *testing.T) {
	ctx := context.Background()
	conn, accounts, channels, messages, links := setupHistoryTestStore(t)
	accountID, channelID := seedHistoryAccountAndChannel(t, ctx, accounts, channels)

	fake := &floodThenEmptyHistoryClient{}
	service := NewService(Options{
		DB: conn, Accounts: accounts, Channels: channels, Messages: messages, Links: links,
		Telegram: fake, Sessions: session.NewManager(filepath.Join(t.TempDir(), "sessions")),
		Extractor: link.NewExtractor(), HistoryBatchSize: 10,
		RetryPolicy: retry.Policy{
			BaseDelay: time.Millisecond,
			MaxDelay:  time.Millisecond,
			MaxTries:  2,
			Sleep:     func(context.Context, time.Duration) error { return nil },
		},
	})

	_, err := service.SyncChannel(ctx, channelID)
	if err != nil {
		t.Fatalf("SyncChannel returned error: %v", err)
	}
	account, err := accounts.FindByID(ctx, accountID)
	if err != nil {
		t.Fatalf("find account: %v", err)
	}
	if account.Status != model.AccountStatusFloodWait {
		t.Fatalf("status = %q, want FLOOD_WAIT", account.Status)
	}
	if fake.calls != 2 {
		t.Fatalf("fetch calls = %d, want 2", fake.calls)
	}
}

func setupHistoryTestStore(t *testing.T) (*sql.DB, *repository.AccountRepository, *repository.ChannelRepository, *repository.MessageRepository, *repository.LinkRepository) {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(context.Background(), conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	return conn, repository.NewAccountRepository(conn), repository.NewChannelRepository(conn), repository.NewMessageRepository(conn), repository.NewLinkRepository(conn)
}

func seedHistoryAccountAndChannel(t *testing.T, ctx context.Context, accounts *repository.AccountRepository, channels *repository.ChannelRepository) (int64, int64) {
	t.Helper()
	accountID, err := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	if err != nil {
		t.Fatalf("save account: %v", err)
	}
	channelID, err := channels.Save(ctx, model.Channel{
		AccountID: accountID, TelegramChannelID: 200, AccessHash: 300, Title: "VIP", Type: model.ChannelTypeChannel,
	})
	if err != nil {
		t.Fatalf("save channel: %v", err)
	}
	return accountID, channelID
}

type retryingHistoryClient struct {
	telegram.NopClient
	calls                 int
	failuresBeforeSuccess int
	successBatch          []telegram.Message
}

func (f *retryingHistoryClient) FetchHistory(ctx context.Context, account telegram.AccountSession, channel telegram.ChannelRef, offsetID int64, limit int) ([]telegram.Message, error) {
	f.calls++
	if f.calls <= f.failuresBeforeSuccess {
		return nil, retry.Temporary(errors.New("temporary history failure"))
	}
	if offsetID > 0 {
		return nil, nil
	}
	return f.successBatch, nil
}

type floodThenEmptyHistoryClient struct {
	telegram.NopClient
	calls int
}

func (f *floodThenEmptyHistoryClient) FetchHistory(ctx context.Context, account telegram.AccountSession, channel telegram.ChannelRef, offsetID int64, limit int) ([]telegram.Message, error) {
	f.calls++
	if f.calls == 1 {
		return nil, retry.FloodWait(60, errors.New("FLOOD_WAIT_60"))
	}
	return nil, nil
}
```

Add missing imports to `internal/history/service_test.go`:

```go
import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"tg-provider/internal/db"
	"tg-provider/internal/link"
	"tg-provider/internal/model"
	"tg-provider/internal/repository"
	"tg-provider/internal/retry"
	"tg-provider/internal/session"
	"tg-provider/internal/telegram"
)
```

- [ ] **Step 2: Run the history tests to verify they fail**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/history
```

Expected: FAIL because `Options.RetryPolicy` does not exist and `SyncChannel` does not retry.

- [ ] **Step 3: Add retry support to history service**

Modify `internal/history/service.go` imports:

```go
import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	dbpkg "tg-provider/internal/db"
	"tg-provider/internal/link"
	"tg-provider/internal/model"
	"tg-provider/internal/repository"
	"tg-provider/internal/retry"
	"tg-provider/internal/session"
	"tg-provider/internal/telegram"
)
```

Extend `Options` and `Service`:

```go
type Options struct {
	DB               *sql.DB
	Accounts         *repository.AccountRepository
	Channels         *repository.ChannelRepository
	Messages         *repository.MessageRepository
	Links            *repository.LinkRepository
	Telegram         telegram.Client
	Sessions         *session.Manager
	Extractor        *link.Extractor
	HistoryBatchSize int
	Workers          int
	RetryPolicy      retry.Policy
}

type Service struct {
	db               *sql.DB
	accounts         *repository.AccountRepository
	channels         *repository.ChannelRepository
	messages         *repository.MessageRepository
	links            *repository.LinkRepository
	telegram         telegram.Client
	sessions         *session.Manager
	extractor        *link.Extractor
	historyBatchSize int
	workers          int
	retryPolicy      retry.Policy
	mu               sync.Mutex
	runningChannels  map[int64]struct{}
}
```

In `NewService`, normalize workers and retry policy:

```go
	if opts.Workers <= 0 {
		opts.Workers = 1
	}
	if opts.RetryPolicy.MaxTries == 0 && opts.RetryPolicy.BaseDelay == 0 && opts.RetryPolicy.MaxDelay == 0 && opts.RetryPolicy.Sleep == nil {
		opts.RetryPolicy = retry.DefaultPolicy()
	}
	return &Service{
		db:               opts.DB,
		accounts:         opts.Accounts,
		channels:         opts.Channels,
		messages:         opts.Messages,
		links:            opts.Links,
		telegram:         opts.Telegram,
		sessions:         opts.Sessions,
		extractor:        opts.Extractor,
		historyBatchSize: opts.HistoryBatchSize,
		workers:          opts.Workers,
		retryPolicy:      opts.RetryPolicy,
		runningChannels:  map[int64]struct{}{},
	}
```

Replace `SyncChannel` with a wrapper and move the existing body into `syncChannelOnce`:

```go
var ErrChannelSyncInProgress = errors.New("channel sync already in progress")

func (s *Service) SyncChannel(ctx context.Context, channelID int64) (SyncResult, error) {
	if !s.tryAcquireChannel(channelID) {
		return SyncResult{}, ErrChannelSyncInProgress
	}
	defer s.releaseChannel(channelID)
	return s.syncChannelWithRetry(ctx, channelID)
}

func (s *Service) syncChannelWithRetry(ctx context.Context, channelID int64) (SyncResult, error) {
	var result SyncResult
	err := s.retryPolicy.Run(ctx, func() error {
		next, err := s.syncChannelOnce(ctx, channelID)
		if err == nil {
			result = next
		}
		return err
	}, func(ctx context.Context, attempt retry.Attempt) {
		if attempt.Classification.Kind == retry.KindFloodWait {
			s.markChannelAccountStatus(ctx, channelID, model.AccountStatusFloodWait)
		}
	})
	return result, err
}

func (s *Service) syncChannelOnce(ctx context.Context, channelID int64) (SyncResult, error) {
	// Move the current SyncChannel body here unchanged except for the function name.
}
```

Add lock and account-state helpers:

```go
func (s *Service) tryAcquireChannel(channelID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.runningChannels[channelID]; ok {
		return false
	}
	s.runningChannels[channelID] = struct{}{}
	return true
}

func (s *Service) releaseChannel(channelID int64) {
	s.mu.Lock()
	delete(s.runningChannels, channelID)
	s.mu.Unlock()
}

func (s *Service) markChannelAccountStatus(ctx context.Context, channelID int64, status string) {
	if s.accounts == nil || s.channels == nil {
		return
	}
	channel, err := s.channels.FindByID(ctx, channelID)
	if err != nil {
		return
	}
	_ = s.accounts.UpdateStatus(ctx, channel.AccountID, status)
}
```

- [ ] **Step 4: Run the history tests to verify they pass**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/history
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/history/service.go internal/history/service_test.go
git commit -m "feat: retry history sync failures"
```

---

### Task 3: History SyncMany Worker Pool And Channel Locks

**Files:**
- Modify: `internal/history/service.go`
- Modify: `internal/history/service_test.go`

- [ ] **Step 1: Write failing worker pool tests**

Append to `internal/history/service_test.go`:

```go
func TestSyncManyDeduplicatesChannelIDsAndRespectsWorkerLimit(t *testing.T) {
	ctx := context.Background()
	conn, accounts, channels, messages, links := setupHistoryTestStore(t)
	accountID, channel1 := seedHistoryAccountAndChannel(t, ctx, accounts, channels)
	channel2, err := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 201, AccessHash: 301, Title: "VIP 2", Type: model.ChannelTypeChannel})
	if err != nil {
		t.Fatalf("save channel2: %v", err)
	}
	channel3, err := channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 202, AccessHash: 302, Title: "VIP 3", Type: model.ChannelTypeChannel})
	if err != nil {
		t.Fatalf("save channel3: %v", err)
	}
	fake := &concurrentHistoryClient{delay: 5 * time.Millisecond}
	service := NewService(Options{
		DB: conn, Accounts: accounts, Channels: channels, Messages: messages, Links: links,
		Telegram: fake, Sessions: session.NewManager(filepath.Join(t.TempDir(), "sessions")),
		Extractor: link.NewExtractor(), HistoryBatchSize: 10, Workers: 2,
		RetryPolicy: retry.Policy{BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, MaxTries: 1, Sleep: func(context.Context, time.Duration) error { return nil }},
	})

	result := service.SyncMany(ctx, []int64{channel1, channel1, channel2, channel3})
	if result.Queued != 3 {
		t.Fatalf("queued = %d, want 3 unique channels", result.Queued)
	}
	if result.Skipped != 1 {
		t.Fatalf("skipped = %d, want 1 duplicate", result.Skipped)
	}
	if len(result.Failures) != 0 {
		t.Fatalf("failures = %+v, want none", result.Failures)
	}
	if fake.maxActive > 2 {
		t.Fatalf("max active = %d, want <= 2", fake.maxActive)
	}
}

func TestSyncManySkipsChannelAlreadySyncing(t *testing.T) {
	ctx := context.Background()
	conn, accounts, channels, messages, links := setupHistoryTestStore(t)
	_, channelID := seedHistoryAccountAndChannel(t, ctx, accounts, channels)
	fake := &blockingHistoryClient{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	service := NewService(Options{
		DB: conn, Accounts: accounts, Channels: channels, Messages: messages, Links: links,
		Telegram: fake, Sessions: session.NewManager(filepath.Join(t.TempDir(), "sessions")),
		Extractor: link.NewExtractor(), HistoryBatchSize: 10, Workers: 1,
		RetryPolicy: retry.Policy{BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, MaxTries: 1, Sleep: func(context.Context, time.Duration) error { return nil }},
	})

	done := make(chan error, 1)
	go func() {
		_, err := service.SyncChannel(ctx, channelID)
		done <- err
	}()
	select {
	case <-fake.started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first sync to start")
	}

	result := service.SyncMany(ctx, []int64{channelID})
	if result.Queued != 0 || result.Skipped != 1 {
		t.Fatalf("result = %+v, want queued=0 skipped=1", result)
	}

	close(fake.release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("first SyncChannel returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first sync")
	}
}

type concurrentHistoryClient struct {
	telegram.NopClient
	mu        sync.Mutex
	active    int
	maxActive int
	delay     time.Duration
}

func (f *concurrentHistoryClient) FetchHistory(ctx context.Context, account telegram.AccountSession, channel telegram.ChannelRef, offsetID int64, limit int) ([]telegram.Message, error) {
	f.mu.Lock()
	f.active++
	if f.active > f.maxActive {
		f.maxActive = f.active
	}
	f.mu.Unlock()
	time.Sleep(f.delay)
	f.mu.Lock()
	f.active--
	f.mu.Unlock()
	return nil, nil
}

type blockingHistoryClient struct {
	telegram.NopClient
	once    sync.Once
	started chan struct{}
	release chan struct{}
}

func (f *blockingHistoryClient) FetchHistory(ctx context.Context, account telegram.AccountSession, channel telegram.ChannelRef, offsetID int64, limit int) ([]telegram.Message, error) {
	f.once.Do(func() { close(f.started) })
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-f.release:
		return nil, nil
	}
}
```

Add `sync` to the `internal/history/service_test.go` imports.

- [ ] **Step 2: Run the history tests to verify they fail**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/history
```

Expected: FAIL because `SyncMany` and `SyncManyResult` do not exist.

- [ ] **Step 3: Implement SyncMany**

Add to `internal/history/service.go`:

```go
type SyncManyResult struct {
	Queued   int                  `json:"queued"`
	Skipped  int                  `json:"skipped"`
	Results  map[int64]SyncResult `json:"results"`
	Failures map[int64]string     `json:"failures"`
}

func (s *Service) SyncMany(ctx context.Context, channelIDs []int64) SyncManyResult {
	result := SyncManyResult{
		Results:  map[int64]SyncResult{},
		Failures: map[int64]string{},
	}
	unique := make([]int64, 0, len(channelIDs))
	seen := map[int64]struct{}{}
	for _, channelID := range channelIDs {
		if channelID <= 0 {
			result.Skipped++
			continue
		}
		if _, ok := seen[channelID]; ok {
			result.Skipped++
			continue
		}
		seen[channelID] = struct{}{}
		unique = append(unique, channelID)
	}
	if len(unique) == 0 {
		return result
	}

	workers := s.workers
	if workers <= 0 {
		workers = 1
	}
	if workers > len(unique) {
		workers = len(unique)
	}

	jobs := make(chan int64)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for channelID := range jobs {
				if !s.tryAcquireChannel(channelID) {
					mu.Lock()
					result.Skipped++
					mu.Unlock()
					continue
				}
				syncResult, err := s.syncChannelWithRetry(ctx, channelID)
				s.releaseChannel(channelID)
				mu.Lock()
				if err != nil {
					result.Failures[channelID] = err.Error()
				} else {
					result.Queued++
					result.Results[channelID] = syncResult
				}
				mu.Unlock()
			}
		}()
	}
	for _, channelID := range unique {
		select {
		case <-ctx.Done():
			mu.Lock()
			result.Failures[channelID] = ctx.Err().Error()
			mu.Unlock()
		case jobs <- channelID:
		}
	}
	close(jobs)
	wg.Wait()
	return result
}
```

- [ ] **Step 4: Run history tests**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/history
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/history/service.go internal/history/service_test.go
git commit -m "feat: add history sync worker pool"
```

---

### Task 4: Batch Channel Sync API

**Files:**
- Modify: `internal/api/router.go`
- Modify: `internal/api/handlers.go`
- Modify: `internal/api/handlers_test.go`

- [ ] **Step 1: Write failing batch sync API tests**

Append to `internal/api/handlers_test.go`:

```go
func TestBatchSyncAPIValidatesChannelIDs(t *testing.T) {
	router := NewRouter(testDeps(t))
	for _, body := range []string{
		`{}`,
		`{"channel_ids":[]}`,
		`{"channel_ids":[0]}`,
		`{"channel_ids":[-1]}`,
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/channels/sync", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d body=%s, want 400", body, w.Code, w.Body.String())
		}
	}
}

func TestBatchSyncAPIReturnsPerChannelResults(t *testing.T) {
	ctx := context.Background()
	deps, conn := testDepsWithDB(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 10, AccessHash: 20, Title: "VIP", Type: model.ChannelTypeChannel})
	deps.History = history.NewService(history.Options{
		DB: conn, Accounts: deps.Accounts, Channels: deps.Channels, Messages: deps.Messages, Links: deps.Links,
		Telegram: &apiHistoryClient{date: time.Now().UTC()},
		Sessions: session.NewManager(filepath.Join(t.TempDir(), "sessions")),
		Extractor: link.NewExtractor(), HistoryBatchSize: 10, Workers: 2,
		RetryPolicy: retry.Policy{BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, MaxTries: 1, Sleep: func(context.Context, time.Duration) error { return nil }},
	})
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/channels/sync", bytes.NewBufferString(`{"channel_ids":[`+strconv.FormatInt(channelID, 10)+`]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%s, want 202", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"queued":1`)) {
		t.Fatalf("response = %s, want queued 1", w.Body.String())
	}
}

type apiHistoryClient struct {
	telegram.NopClient
	date time.Time
}

func (f *apiHistoryClient) FetchHistory(ctx context.Context, account telegram.AccountSession, channel telegram.ChannelRef, offsetID int64, limit int) ([]telegram.Message, error) {
	if offsetID > 0 {
		return nil, nil
	}
	return []telegram.Message{{TelegramMessageID: 1, SenderID: 1, Text: "api sync", RawJSON: "{}", Date: f.date}}, nil
}
```

Add missing imports to `internal/api/handlers_test.go`:

```go
import (
	"database/sql"

	"tg-provider/internal/retry"
)
```

Add this helper beside `testDeps` in `internal/api/handlers_test.go`:

```go
func testDepsWithDB(t *testing.T) (Dependencies, *sql.DB) {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(context.Background(), conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	accounts := repository.NewAccountRepository(conn)
	channels := repository.NewChannelRepository(conn)
	messages := repository.NewMessageRepository(conn)
	links := repository.NewLinkRepository(conn)
	maintenance := repository.NewMaintenanceRepository(conn)
	status := repository.NewStatusRepository(conn)
	sessions := session.NewManager(filepath.Join(t.TempDir(), "sessions"))
	client := telegram.NopClient{}
	searchService := search.NewService(messages, links)
	historyService := history.NewService(history.Options{
		DB: conn, Accounts: accounts, Channels: channels, Messages: messages, Links: links,
		Telegram: client, Sessions: sessions, Extractor: link.NewExtractor(), HistoryBatchSize: 100,
	})
	channelService := channel.NewService(channels, client, sessions)
	return Dependencies{
		Accounts: accounts, Channels: channels, Messages: messages, Links: links, Maintenance: maintenance, Status: status,
		Search: searchService, History: historyService, ChannelSync: channelService,
		Telegram: client, Sessions: sessions, CodeStore: telegram.NewCodeStore(),
	}, conn
}
```

- [ ] **Step 2: Run API tests to verify they fail**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/api
```

Expected: FAIL because `/api/channels/sync` is not routed and `syncChannels` does not exist.

- [ ] **Step 3: Add the route**

Modify `internal/api/router.go` so the static route is registered before `/:id`:

```go
	api.GET("/channels", h.channels)
	api.POST("/channels/sync", h.syncChannels)
	api.GET("/channels/:id", h.channel)
	api.POST("/channels/:id/sync", h.syncChannel)
```

- [ ] **Step 4: Add the handler**

Add to `internal/api/handlers.go` near `syncChannel`:

```go
func (h handlers) syncChannels(c *gin.Context) {
	var req struct {
		ChannelIDs []int64 `json:"channel_ids"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if len(req.ChannelIDs) == 0 {
		errorText(c, http.StatusBadRequest, "channel_ids is required")
		return
	}
	for _, id := range req.ChannelIDs {
		if id <= 0 {
			errorText(c, http.StatusBadRequest, "channel_ids must contain positive integers")
			return
		}
	}
	result := h.deps.History.SyncMany(c.Request.Context(), req.ChannelIDs)
	c.JSON(http.StatusAccepted, result)
}
```

- [ ] **Step 5: Run API tests**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/api
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/router.go internal/api/handlers.go internal/api/handlers_test.go
git commit -m "feat: add batch channel sync api"
```

---

### Task 5: Update Listener FloodWait Backoff

**Files:**
- Modify: `internal/update/service.go`
- Modify: `internal/update/service_test.go`

- [ ] **Step 1: Write failing update FloodWait test**

Append to `internal/update/service_test.go`:

```go
func TestServiceMarksFloodWaitAndRetriesListener(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "telegram.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	accounts := repository.NewAccountRepository(conn)
	accountID, err := accounts.Save(ctx, model.Account{Phone: "+10000000000", Status: model.AccountStatusOnline})
	if err != nil {
		t.Fatalf("save account: %v", err)
	}
	account, err := accounts.FindByID(ctx, accountID)
	if err != nil {
		t.Fatalf("find account: %v", err)
	}

	listener := &floodWaitListener{started: make(chan struct{}, 2)}
	var slept []time.Duration
	service := NewService(ServiceOptions{
		Accounts:  accounts,
		Processor: &recordingProcessor{},
		Listener:  listener,
		RetryPolicy: retry.Policy{
			BaseDelay: time.Millisecond,
			MaxDelay:  time.Millisecond,
			MaxTries:  3,
			Sleep: func(ctx context.Context, d time.Duration) error {
				slept = append(slept, d)
				return nil
			},
		},
	})
	service.Start(ctx)
	if err := service.StartAccount(ctx, account); err != nil {
		t.Fatalf("StartAccount returned error: %v", err)
	}

	waitForStarts(t, listener.started, 2)
	updated, err := accounts.FindByID(ctx, accountID)
	if err != nil {
		t.Fatalf("find updated account: %v", err)
	}
	if updated.Status != model.AccountStatusFloodWait {
		t.Fatalf("status = %q, want FLOOD_WAIT", updated.Status)
	}
	if len(slept) != 1 || slept[0] != time.Millisecond {
		t.Fatalf("slept = %v, want capped 1ms flood wait", slept)
	}
	if err := service.Stop(context.Background()); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
}

type floodWaitListener struct {
	mu      sync.Mutex
	runs    int
	started chan struct{}
}

func (l *floodWaitListener) Run(ctx context.Context, account model.Account, emit func(Event) error) error {
	l.mu.Lock()
	l.runs++
	run := l.runs
	l.mu.Unlock()
	l.started <- struct{}{}
	if run == 1 {
		return retry.FloodWait(60, errors.New("FLOOD_WAIT_60"))
	}
	<-ctx.Done()
	return ctx.Err()
}
```

Add `tg-provider/internal/retry` to the `internal/update/service_test.go` imports.

- [ ] **Step 2: Run update tests to verify they fail**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/update
```

Expected: FAIL because `ServiceOptions.RetryPolicy` does not exist and FloodWait does not set `FLOOD_WAIT`.

- [ ] **Step 3: Add retry policy to update service**

Modify `internal/update/service.go` imports:

```go
import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"

	"tg-provider/internal/model"
	"tg-provider/internal/repository"
	"tg-provider/internal/retry"
)
```

Extend `ServiceOptions` and `Service`:

```go
type ServiceOptions struct {
	Accounts      *repository.AccountRepository
	Processor     EventProcessor
	Listener      Listener
	QueueSize     int
	RetryInterval time.Duration
	RetryPolicy   retry.Policy
	Logger        *zap.Logger
}

type Service struct {
	accounts      *repository.AccountRepository
	processor     EventProcessor
	listener      Listener
	retryInterval time.Duration
	retryPolicy   retry.Policy
	logger        *zap.Logger
	// existing fields stay below
}
```

In `NewService`, set a policy that preserves `RetryInterval` behavior when callers do not provide a policy:

```go
	if opts.RetryPolicy.MaxTries == 0 && opts.RetryPolicy.BaseDelay == 0 && opts.RetryPolicy.MaxDelay == 0 && opts.RetryPolicy.Sleep == nil {
		opts.RetryPolicy = retry.Policy{
			BaseDelay: opts.RetryInterval,
			MaxDelay:  30 * time.Minute,
			MaxTries:  3,
		}
	}
	return &Service{
		accounts:      opts.Accounts,
		processor:     opts.Processor,
		listener:      opts.Listener,
		retryInterval: opts.RetryInterval,
		retryPolicy:   opts.RetryPolicy,
		logger:        opts.Logger,
		events:        make(chan Event, opts.QueueSize),
	}
```

Replace the fixed timer block in `runListener` with classification-based delay:

```go
		classification := retry.Classify(err)
		if classification.Kind == retry.KindPermanent {
			s.logger.Error("telegram update listener permanent failure", zap.Int64("account_id", account.ID), zap.Error(err))
			return
		}
		status := model.AccountStatusReconnecting
		if classification.Kind == retry.KindFloodWait {
			status = model.AccountStatusFloodWait
		}
		if s.accounts != nil {
			if updateErr := s.accounts.UpdateStatus(s.listenerCtx, account.ID, status); updateErr != nil {
				s.logger.Error("mark account after listener failure", zap.Int64("account_id", account.ID), zap.String("status", status), zap.Error(updateErr))
			}
		}
		delay := s.retryPolicy.Delay(1, classification)
		if s.retryPolicy.Sleep == nil {
			s.retryPolicy.Sleep = func(ctx context.Context, d time.Duration) error {
				timer := time.NewTimer(d)
				defer timer.Stop()
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-timer.C:
					return nil
				}
			}
		}
		if sleepErr := s.retryPolicy.Sleep(s.listenerCtx, delay); sleepErr != nil {
			return
		}
```

Keep the existing log line before classification:

```go
s.logger.Warn("telegram update listener stopped", zap.Int64("account_id", account.ID), zap.Error(err))
```

- [ ] **Step 4: Run update tests**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/update
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/update/service.go internal/update/service_test.go
git commit -m "feat: classify update listener retries"
```

---

### Task 6: Cleanup Scheduler Skeleton

**Files:**
- Create: `internal/scheduler/scheduler_test.go`
- Create: `internal/scheduler/scheduler.go`
- Create: `internal/scheduler/cleanup.go`

- [ ] **Step 1: Write failing scheduler tests**

Create `internal/scheduler/scheduler_test.go`:

```go
package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestSchedulerRunsJobAndStops(t *testing.T) {
	var runs int64
	job := jobFunc{
		name: "count",
		run: func(context.Context) error {
			atomic.AddInt64(&runs, 1)
			return nil
		},
	}
	s := New(Options{Interval: time.Millisecond, Jobs: []Job{job}, Logger: zap.NewNop()})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)
	waitForSchedulerRuns(t, &runs, 1)
	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	before := atomic.LoadInt64(&runs)
	time.Sleep(3 * time.Millisecond)
	after := atomic.LoadInt64(&runs)
	if after != before {
		t.Fatalf("runs after stop = %d, want %d", after, before)
	}
}

func TestSchedulerContinuesAfterJobError(t *testing.T) {
	var runs int64
	job := jobFunc{
		name: "flaky",
		run: func(context.Context) error {
			atomic.AddInt64(&runs, 1)
			return errors.New("cleanup failed")
		},
	}
	s := New(Options{Interval: time.Millisecond, Jobs: []Job{job}, Logger: zap.NewNop()})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)
	waitForSchedulerRuns(t, &runs, 2)
	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
}

func TestCleanupJobRuns(t *testing.T) {
	job := CleanupJob{Logger: zap.NewNop()}
	if job.Name() != "cleanup" {
		t.Fatalf("name = %q, want cleanup", job.Name())
	}
	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}

type jobFunc struct {
	name string
	run  func(context.Context) error
}

func (j jobFunc) Name() string {
	return j.name
}

func (j jobFunc) Run(ctx context.Context) error {
	return j.run(ctx)
}

func waitForSchedulerRuns(t *testing.T, runs *int64, want int64) {
	t.Helper()
	deadline := time.After(time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatalf("runs = %d, want at least %d", atomic.LoadInt64(runs), want)
		case <-ticker.C:
			if atomic.LoadInt64(runs) >= want {
				return
			}
		}
	}
}
```

- [ ] **Step 2: Run scheduler tests to verify they fail**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/scheduler
```

Expected: FAIL because package `tg-provider/internal/scheduler` does not exist.

- [ ] **Step 3: Implement scheduler**

Create `internal/scheduler/scheduler.go`:

```go
package scheduler

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

type Job interface {
	Name() string
	Run(context.Context) error
}

type Options struct {
	Interval time.Duration
	Jobs     []Job
	Logger   *zap.Logger
}

type Scheduler struct {
	interval time.Duration
	jobs     []Job
	logger   *zap.Logger

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func New(opts Options) *Scheduler {
	if opts.Interval <= 0 {
		opts.Interval = time.Hour
	}
	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}
	return &Scheduler{interval: opts.Interval, jobs: opts.Jobs, logger: opts.Logger}
}

func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	for _, job := range s.jobs {
		job := job
		s.wg.Add(1)
		go s.runJob(runCtx, job)
	}
	s.mu.Unlock()
}

func (s *Scheduler) Stop(ctx context.Context) error {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Scheduler) runJob(ctx context.Context, job Job) {
	defer s.wg.Done()
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := job.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				s.logger.Error("scheduler job failed", zap.String("job", job.Name()), zap.Error(err))
				continue
			}
			s.logger.Debug("scheduler job completed", zap.String("job", job.Name()))
		}
	}
}
```

Create `internal/scheduler/cleanup.go`:

```go
package scheduler

import (
	"context"

	"go.uber.org/zap"
)

type CleanupJob struct {
	Logger *zap.Logger
}

func (j CleanupJob) Name() string {
	return "cleanup"
}

func (j CleanupJob) Run(ctx context.Context) error {
	logger := j.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	logger.Info("cleanup job checked temporary data")
	return nil
}
```

- [ ] **Step 4: Run scheduler tests**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/scheduler
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/scheduler/scheduler.go internal/scheduler/cleanup.go internal/scheduler/scheduler_test.go
git commit -m "feat: add cleanup scheduler skeleton"
```

---

### Task 7: Main Wiring And Final Verification

**Files:**
- Modify: `cmd/tg-provider/main.go`
- Modify: `internal/api/handlers_test.go` if Task 4 changed the test helper shape

- [ ] **Step 1: Wire retry, workers, and scheduler in main**

Modify imports in `cmd/tg-provider/main.go`:

```go
	"tg-provider/internal/retry"
	"tg-provider/internal/scheduler"
```

Create one retry policy after `tgClient`:

```go
	retryPolicy := retry.DefaultPolicy()
```

Pass it to update service:

```go
	updateService := updatepkg.NewService(updatepkg.ServiceOptions{
		Accounts:    accounts,
		Processor:   updateProcessor,
		Listener:    updatepkg.NewGotdListener(cfg.Telegram.APIID, cfg.Telegram.APIHash, sessions, logs.Telegram),
		RetryPolicy: retryPolicy,
		Logger:      logs.Telegram,
	})
```

Pass workers and retry policy to history service:

```go
	historyService := history.NewService(history.Options{
		DB: conn, Accounts: accounts, Channels: channels, Messages: messages, Links: links,
		Telegram: tgClient, Sessions: sessions, Extractor: link.NewExtractor(),
		HistoryBatchSize: cfg.Sync.HistoryBatchSize,
		Workers:          cfg.Sync.Workers,
		RetryPolicy:      retryPolicy,
	})
```

Start the cleanup scheduler after `accountManager.Start(ctx)` succeeds:

```go
	cleanupScheduler := scheduler.New(scheduler.Options{
		Interval: time.Hour,
		Jobs: []scheduler.Job{
			scheduler.CleanupJob{Logger: logs.App},
		},
		Logger: logs.App,
	})
	cleanupScheduler.Start(ctx)
```

Stop it during shutdown before stopping the account manager:

```go
	if err := cleanupScheduler.Stop(shutdownCtx); err != nil {
		return err
	}
	if err := accountManager.Stop(shutdownCtx); err != nil {
		return err
	}
```

- [ ] **Step 2: Run focused package tests**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/retry ./internal/history ./internal/api ./internal/update ./internal/scheduler ./cmd/tg-provider
```

Expected: PASS.

- [ ] **Step 3: Run full tests**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./...
```

Expected: PASS.

- [ ] **Step 4: Run full build**

Run:

```bash
GOCACHE=/tmp/go-build-cache go build ./...
```

Expected: exit code 0.

- [ ] **Step 5: Inspect final branch state**

Run:

```bash
git status --short --branch
git log --oneline --decorate -10
```

Expected: on `phase-5b-runtime-reliability` with no uncommitted changes after the final commit.

- [ ] **Step 6: Commit**

```bash
git add cmd/tg-provider/main.go internal/api/handlers_test.go
git commit -m "feat: wire runtime reliability services"
```

If `internal/api/handlers_test.go` was not changed in this task, omit it from `git add`.

---

## Self-Review

Spec coverage:

- History worker pool is covered by Task 3.
- Same-channel concurrency protection is covered by Tasks 2 and 3.
- Retry/backoff and FloodWait classification are covered by Task 1.
- History retry and FloodWait account state are covered by Task 2.
- Batch sync API is covered by Task 4.
- Update listener FloodWait behavior is covered by Task 5.
- Cleanup scheduler skeleton is covered by Task 6.
- Main wiring is covered by Task 7.

Type consistency:

- The plan uses `retry.Policy`, `retry.Attempt`, `retry.Classification`, and `retry.KindFloodWait` consistently.
- `history.Options` owns `Workers` and `RetryPolicy`; `update.ServiceOptions` owns `RetryPolicy`.
- `SyncManyResult` uses `map[int64]SyncResult` and `map[int64]string`, matching the spec.

Execution notes:

- Use `GOCACHE=/tmp/go-build-cache` for all Go test and build commands.
- Keep commits task-scoped.
- If a focused test fails because a previous task changed helper structure, adjust only the local test helper needed by that task and keep the public runtime behavior unchanged.
