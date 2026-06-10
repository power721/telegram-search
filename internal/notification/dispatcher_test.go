package notification

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"tg-search/internal/db"
	"tg-search/internal/model"
	"tg-search/internal/repository"
)

func TestDispatcherDeliversWebhook(t *testing.T) {
	ctx := context.Background()
	conn := setupNotificationDB(t)
	webhooks := repository.NewWebhookRepository(conn)
	deliveries := repository.NewNotificationDeliveryRepository(conn)

	var gotEvent string
	var gotDelivery string
	var gotSignature string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEvent = r.Header.Get("X-TG-Search-Event")
		gotDelivery = r.Header.Get("X-TG-Search-Delivery")
		gotSignature = r.Header.Get("X-TG-Search-Signature")
		body, _ := io.ReadAll(r.Body)
		gotBody = body
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	hookID, err := webhooks.Create(ctx, model.Webhook{
		Name:    "hook",
		URL:     server.URL,
		Events:  []string{model.NotificationEventResourceCreated},
		Secret:  "secret",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create webhook: %v", err)
	}
	deliveryID, err := deliveries.Create(ctx, model.NotificationDelivery{
		EventType:   model.NotificationEventResourceCreated,
		TargetType:  model.NotificationTargetWebhook,
		TargetID:    hookID,
		PayloadJSON: `{"title":"哪吒3"}`,
	})
	if err != nil {
		t.Fatalf("create delivery: %v", err)
	}
	dispatcher := NewDispatcher(DispatcherOptions{
		Deliveries: deliveries,
		Webhooks:   webhooks,
		Now:        func() time.Time { return time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC) },
	})
	if err := dispatcher.Run(ctx); err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}

	if gotEvent != model.NotificationEventResourceCreated || gotDelivery != "1" {
		t.Fatalf("headers event=%q delivery=%q, want resource.created and 1", gotEvent, gotDelivery)
	}
	mac := hmac.New(sha256.New, []byte("secret"))
	_, _ = mac.Write(gotBody)
	wantSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if gotSignature != wantSignature {
		t.Fatalf("signature = %q, want %q", gotSignature, wantSignature)
	}
	var envelope struct {
		EventType  string          `json:"event_type"`
		DeliveryID int64           `json:"delivery_id"`
		Payload    json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(gotBody, &envelope); err != nil {
		t.Fatalf("decode webhook body: %v", err)
	}
	if envelope.EventType != model.NotificationEventResourceCreated || envelope.DeliveryID != deliveryID || string(envelope.Payload) != `{"title":"哪吒3"}` {
		t.Fatalf("envelope = %+v payload=%s, want event and payload", envelope, envelope.Payload)
	}
	stored, err := deliveries.FindByID(ctx, deliveryID)
	if err != nil {
		t.Fatalf("find delivery: %v", err)
	}
	if stored.Status != model.NotificationDeliverySucceeded || stored.DeliveredAt == nil {
		t.Fatalf("delivery = %+v, want succeeded with delivered_at", stored)
	}
}

func TestDispatcherRetriesWebhookFailure(t *testing.T) {
	ctx := context.Background()
	conn := setupNotificationDB(t)
	webhooks := repository.NewWebhookRepository(conn)
	deliveries := repository.NewNotificationDeliveryRepository(conn)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)
	hookID, err := webhooks.Create(ctx, model.Webhook{
		Name:    "hook",
		URL:     server.URL,
		Events:  []string{model.NotificationEventResourceCreated},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create webhook: %v", err)
	}
	deliveryID, err := deliveries.Create(ctx, model.NotificationDelivery{
		EventType:   model.NotificationEventResourceCreated,
		TargetType:  model.NotificationTargetWebhook,
		TargetID:    hookID,
		PayloadJSON: `{"title":"哪吒3"}`,
	})
	if err != nil {
		t.Fatalf("create delivery: %v", err)
	}
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	dispatcher := NewDispatcher(DispatcherOptions{
		Deliveries: deliveries,
		Webhooks:   webhooks,
		RetryDelay: time.Minute,
		Now:        func() time.Time { return now },
	})
	if err := dispatcher.Run(ctx); err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}
	stored, err := deliveries.FindByID(ctx, deliveryID)
	if err != nil {
		t.Fatalf("find delivery: %v", err)
	}
	if stored.Status != model.NotificationDeliveryFailed || stored.RetryCount != 1 || stored.NextRunAt == nil {
		t.Fatalf("delivery = %+v, want failed retry scheduled", stored)
	}
	if !stored.NextRunAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("next_run_at = %v, want %v", stored.NextRunAt, now.Add(time.Minute))
	}
}

func setupNotificationDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "tg-search.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(context.Background(), conn); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return conn
}
