package redact

import "strings"

const Redacted = "[REDACTED]"

var sensitiveKeys = map[string]struct{}{
	"app_hash":        {},
	"api_hash":        {},
	"code":            {},
	"login_code":      {},
	"password":        {},
	"phone_code_hash": {},
	"session":         {},
	"session_data":    {},
}

func Secrets(values map[string]any) map[string]any {
	out := make(map[string]any, len(values))
	for key, value := range values {
		if _, ok := sensitiveKeys[strings.ToLower(key)]; ok {
			out[key] = Redacted
			continue
		}
		out[key] = value
	}
	return out
}

func String(input string, secrets ...string) string {
	out := input
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		out = strings.ReplaceAll(out, secret, Redacted)
	}
	return out
}
