package redact

import "testing"

func TestSecretsRedactsKnownSensitiveKeys(t *testing.T) {
	input := map[string]any{
		"api_hash":        "hash",
		"password":        "pass",
		"code":            "12345",
		"phone_code_hash": "secret-code-hash",
		"session":         "session-data",
		"phone":           "+10000000000",
	}

	got := Secrets(input)

	for _, key := range []string{"api_hash", "password", "code", "phone_code_hash", "session"} {
		if got[key] != Redacted {
			t.Fatalf("%s = %q, want redacted", key, got[key])
		}
	}
	if got["phone"] != "+10000000000" {
		t.Fatalf("phone = %q, want unchanged", got["phone"])
	}
}

func TestStringReplacesSecretValues(t *testing.T) {
	got := String("api_hash=abc password=secret session-data", "abc", "secret", "session-data")

	if got != "api_hash=[REDACTED] password=[REDACTED] [REDACTED]" {
		t.Fatalf("got %q", got)
	}
}
