# Telegram Client Reliability Design

Date: 2026-06-09

## Goal

Improve Telegram API reliability by adding non-credential runtime settings and applying them consistently to gotd clients used by login, QR login, history sync, remote search, media proxy, and realtime listeners.

## Scope

This change adds runtime configuration only. It does not move Telegram API ID or API hash into `config.yaml`; credentials remain managed by the admin setup/settings flow.

In scope:

- Add `telegram.proxy`, `telegram.rate_limit`, `telegram.reconnect_timeout`, and `telegram.dial_timeout` config fields.
- Apply gotd FloodWait middleware by default.
- Apply gotd rate limit middleware when enabled.
- Apply proxy dialer, reconnect backoff, and dial timeout to all gotd client constructors.
- Keep existing device identity defaults unless explicitly changed in code later.

Out of scope:

- Redis or cache infrastructure.
- Media proxy chunk prefetch.
- Database schema changes.
- API route changes.
- Frontend settings UI for these runtime fields.

## Architecture

Create a small helper in `internal/telegram` that builds `gotdtelegram.Options` from a package-level `RuntimeConfig` plus per-client fields such as session storage, update handler, logger, and `NoUpdates`. `GotdClient`, QR login, and `update.GotdListener` will call the helper instead of constructing duplicate option structs.

`internal/config` owns YAML parsing and defaults. `cmd/tg-search/main.go` passes `cfg.Telegram` into both `telegram.NewGotdClient` and `update.NewGotdListener`.

## Error Handling

Invalid proxy URLs should fail before network client use when client options are built. Missing credentials should continue to fail before option construction in flows that currently validate credentials first.

## Testing

Tests should cover:

- Config defaults and YAML overrides for the new `telegram` section.
- Rejection of credential-like fields such as `telegram.api_id` and `telegram.api_hash`.
- gotd option construction includes FloodWait middleware by default.
- rate limit middleware is only added when enabled.
- invalid proxy configuration returns a clear error.

