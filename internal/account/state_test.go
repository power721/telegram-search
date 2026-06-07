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
