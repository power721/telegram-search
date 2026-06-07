package update

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

func TestServiceProcessesQueuedEventsSequentiallyAndFlushesOnStop(t *testing.T) {
	processor := &recordingProcessor{}
	service := NewService(ServiceOptions{
		Processor: processor,
		QueueSize: 4,
	})

	ctx := context.Background()
	service.Start(ctx)
	for i := int64(1); i <= 3; i++ {
		if err := service.Enqueue(ctx, Event{Type: EventNewMessage, MessageID: i}); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}
	if err := service.Stop(context.Background()); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}

	got := processor.ids()
	want := []int64{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("processed ids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("processed ids = %v, want %v", got, want)
		}
	}
}

func TestServiceRetriesListenerAndMarksAccountReconnecting(t *testing.T) {
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

	listener := &flakyListener{started: make(chan struct{}, 2)}
	service := NewService(ServiceOptions{
		Accounts:      accounts,
		Processor:     &recordingProcessor{},
		Listener:      listener,
		RetryInterval: time.Millisecond,
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
	if updated.Status != model.AccountStatusReconnecting {
		t.Fatalf("status = %q, want RECONNECTING", updated.Status)
	}
	if err := service.Stop(context.Background()); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
}

type recordingProcessor struct {
	mu        sync.Mutex
	events    []Event
	processWG sync.WaitGroup
}

func (r *recordingProcessor) Process(ctx context.Context, event Event) error {
	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()
	return nil
}

func (r *recordingProcessor) ids() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]int64, 0, len(r.events))
	for _, event := range r.events {
		out = append(out, event.MessageID)
	}
	return out
}

type flakyListener struct {
	mu      sync.Mutex
	runs    int
	started chan struct{}
}

func (l *flakyListener) Run(ctx context.Context, account model.Account, emit func(Event) error) error {
	l.mu.Lock()
	l.runs++
	run := l.runs
	l.mu.Unlock()
	l.started <- struct{}{}
	if run == 1 {
		return errors.New("temporary disconnect")
	}
	<-ctx.Done()
	return ctx.Err()
}

func waitForStarts(t *testing.T, ch <-chan struct{}, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for listener start %d", i+1)
		}
	}
}
