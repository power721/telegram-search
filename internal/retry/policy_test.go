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

func TestClassifyAuthKeyUnregisteredAsAuthFailure(t *testing.T) {
	classification := Classify(errors.New("callback: rpcDoRequest: rpc error code 401: AUTH_KEY_UNREGISTERED"))
	if classification.Kind != KindAuth {
		t.Fatalf("classification = %+v, want auth failure", classification)
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
