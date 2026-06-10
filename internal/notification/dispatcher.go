package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"

	"tg-search/internal/model"
	"tg-search/internal/repository"
)

const (
	defaultDispatchBatchSize = 50
	defaultDispatchMaxTries  = 5
	defaultDispatchTimeout   = 10 * time.Second
	defaultRetryDelay        = 30 * time.Second
)

type Dispatcher struct {
	deliveries *repository.NotificationDeliveryRepository
	webhooks   *repository.WebhookRepository
	client     *http.Client
	logger     *zap.Logger
	batchSize  int
	maxTries   int64
	retryDelay time.Duration
	now        func() time.Time
}

type DispatcherOptions struct {
	Deliveries *repository.NotificationDeliveryRepository
	Webhooks   *repository.WebhookRepository
	Client     *http.Client
	Logger     *zap.Logger
	BatchSize  int
	MaxTries   int64
	RetryDelay time.Duration
	Now        func() time.Time
}

type webhookEnvelope struct {
	EventType  string          `json:"event_type"`
	DeliveryID int64           `json:"delivery_id"`
	CreatedAt  time.Time       `json:"created_at"`
	Payload    json.RawMessage `json:"payload"`
}

func NewDispatcher(opts DispatcherOptions) *Dispatcher {
	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: defaultDispatchTimeout}
	}
	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = defaultDispatchBatchSize
	}
	if opts.MaxTries <= 0 {
		opts.MaxTries = defaultDispatchMaxTries
	}
	if opts.RetryDelay <= 0 {
		opts.RetryDelay = defaultRetryDelay
	}
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now().UTC() }
	}
	return &Dispatcher{
		deliveries: opts.Deliveries,
		webhooks:   opts.Webhooks,
		client:     opts.Client,
		logger:     opts.Logger,
		batchSize:  opts.BatchSize,
		maxTries:   opts.MaxTries,
		retryDelay: opts.RetryDelay,
		now:        opts.Now,
	}
}

func (d *Dispatcher) Name() string {
	return "notification-webhook-dispatch"
}

func (d *Dispatcher) Run(ctx context.Context) error {
	if d.deliveries == nil || d.webhooks == nil {
		return nil
	}
	items, err := d.deliveries.DueWebhookDeliveries(ctx, d.now(), d.batchSize)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := d.deliver(ctx, item); err != nil {
			d.logger.Warn("webhook delivery failed",
				zap.Int64("delivery_id", item.ID),
				zap.String("event_type", item.EventType),
				zap.Int64("target_id", item.TargetID),
				zap.Error(err),
			)
			continue
		}
	}
	return nil
}

func (d *Dispatcher) deliver(ctx context.Context, delivery model.NotificationDelivery) error {
	webhook, err := d.webhooks.FindByID(ctx, delivery.TargetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return d.markTerminalFailure(ctx, delivery, "webhook not found")
		}
		return err
	}
	if !webhook.Enabled {
		return d.markTerminalFailure(ctx, delivery, "webhook disabled")
	}
	body, err := encodeWebhookEnvelope(delivery)
	if err != nil {
		return d.markTerminalFailure(ctx, delivery, err.Error())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook.URL, bytes.NewReader(body))
	if err != nil {
		return d.markTerminalFailure(ctx, delivery, err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "tg-search-webhook/1")
	req.Header.Set("X-TG-Search-Event", delivery.EventType)
	req.Header.Set("X-TG-Search-Delivery", strconv.FormatInt(delivery.ID, 10))
	if webhook.Secret != "" {
		req.Header.Set("X-TG-Search-Signature", signWebhookBody(webhook.Secret, body))
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return d.markRetryableFailure(ctx, delivery, err.Error())
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return d.markRetryableFailure(ctx, delivery, fmt.Sprintf("webhook returned HTTP %d", resp.StatusCode))
	}
	if err := d.deliveries.MarkSucceeded(ctx, delivery.ID, d.now()); err != nil {
		return err
	}
	d.logger.Info("webhook delivery succeeded",
		zap.Int64("delivery_id", delivery.ID),
		zap.String("event_type", delivery.EventType),
		zap.Int64("webhook_id", webhook.ID),
	)
	return nil
}

func (d *Dispatcher) markRetryableFailure(ctx context.Context, delivery model.NotificationDelivery, message string) error {
	nextRetryCount := delivery.RetryCount + 1
	var nextRunAt *time.Time
	if nextRetryCount < d.maxTries {
		next := d.now().Add(d.retryDelay * time.Duration(nextRetryCount))
		nextRunAt = &next
	}
	if err := d.deliveries.MarkFailed(ctx, delivery.ID, message, nextRunAt); err != nil {
		return err
	}
	return fmt.Errorf("%s", message)
}

func (d *Dispatcher) markTerminalFailure(ctx context.Context, delivery model.NotificationDelivery, message string) error {
	if err := d.deliveries.MarkFailed(ctx, delivery.ID, message, nil); err != nil {
		return err
	}
	return fmt.Errorf("%s", message)
}

func encodeWebhookEnvelope(delivery model.NotificationDelivery) ([]byte, error) {
	payload := json.RawMessage(delivery.PayloadJSON)
	if !json.Valid(payload) {
		return nil, fmt.Errorf("invalid delivery payload JSON")
	}
	return json.Marshal(webhookEnvelope{
		EventType:  delivery.EventType,
		DeliveryID: delivery.ID,
		CreatedAt:  delivery.CreatedAt,
		Payload:    payload,
	})
}

func signWebhookBody(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
