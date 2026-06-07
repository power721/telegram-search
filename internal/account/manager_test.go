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

func TestManagerStopAccountStopsRuntimeAndMarksDisconnected(t *testing.T) {
	ctx := context.Background()
	fixture := newManagerFixture(t)
	accountID := fixture.saveAccount(t, model.AccountStatusOnline)
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

	if err := manager.StopAccount(ctx, accountID); err != nil {
		t.Fatalf("StopAccount returned error: %v", err)
	}

	account, err := fixture.accounts.FindByID(ctx, accountID)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if account.Status != model.AccountStatusDisconnected {
		t.Fatalf("status = %q, want DISCONNECTED", account.Status)
	}
	if got := runtime.stoppedIDs(); !sameIDs(got, []int64{accountID}) {
		t.Fatalf("stopped ids = %v, want [%d]", got, accountID)
	}
}

func TestManagerStopAccountIsIdempotent(t *testing.T) {
	ctx := context.Background()
	fixture := newManagerFixture(t)
	accountID := fixture.saveAccount(t, model.AccountStatusOnline)
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

	if err := manager.StopAccount(ctx, accountID); err != nil {
		t.Fatalf("first StopAccount returned error: %v", err)
	}
	if err := manager.StopAccount(ctx, accountID); err != nil {
		t.Fatalf("second StopAccount returned error: %v", err)
	}
	if got := runtime.stoppedIDs(); !sameIDs(got, []int64{accountID}) {
		t.Fatalf("stopped ids = %v, want one stop for [%d]", got, accountID)
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
	stopped   []int64
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

func (r *recordingUpdateRuntime) StopAccount(ctx context.Context, accountID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopped = append(r.stopped, accountID)
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

func (r *recordingUpdateRuntime) stoppedIDs() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]int64, len(r.stopped))
	copy(out, r.stopped)
	return out
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
