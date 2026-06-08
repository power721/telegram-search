# tg-search API Documentation

This document describes the active REST API surface for the Phase 1A backend foundation. All responses are JSON.

## Error Response

```json
{
  "error": {
    "code": "bad_request",
    "message": "message"
  }
}
```

## Setup

### `GET /api/setup/status`

Returns first-run setup state.

```json
{
  "complete": false,
  "admin_configured": false,
  "api_key_configured": false,
  "telegram_configured": false
}
```

### `POST /api/setup/admin`

Creates the first admin user.

Request:

```json
{
  "username": "admin",
  "password": "secret123"
}
```

Response `201`:

```json
{
  "id": 1,
  "username": "admin",
  "role": "admin"
}
```

### `POST /api/setup/api-key`

Creates an optional API key. The plaintext key is returned only once.

Request:

```json
{
  "name": "cli"
}
```

Response `201`:

```json
{
  "id": 1,
  "name": "cli",
  "prefix": "0123abcd",
  "key": "0123abcd..."
}
```

### `POST /api/setup/complete`

Marks setup complete and returns setup status.

## Auth

### `POST /api/auth/login`

Creates an `HttpOnly` admin session cookie named `tg_search_session`.

Request:

```json
{
  "username": "admin",
  "password": "secret123"
}
```

### `GET /api/auth/me`

Returns the logged-in admin user. `password_hash` is never returned.

### `POST /api/auth/logout`

Deletes the in-memory session and clears the browser cookie.

## Storage

### `GET /api/storage/usage`

Returns local storage usage and quota state.

```json
{
  "db_bytes": 3200000000,
  "index_bytes": 1100000000,
  "media_cache_bytes": 0,
  "total_bytes": 4300000000,
  "max_db_bytes": 10000000000,
  "max_media_bytes": 20000000000,
  "db_over_quota": false,
  "media_over_quota": false
}
```

## Status

### `GET /api/status`

Returns basic runtime counts and Telegram account status summary.

```json
{
  "service": "ok",
  "accounts": 0,
  "channels": 0,
  "messages": 0,
  "links": 0,
  "account_states": {}
}
```

## Existing Telegram APIs

The existing backend Telegram login, channel, message search, link, maintenance, and backup endpoints remain present during Phase 1A. Later phases rename and reshape these APIs around the standalone `tg-search` admin console.
