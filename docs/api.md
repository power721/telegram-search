# tg-search API Documentation

This document describes the active REST API surface for the Phase 1E local index, search, and resource library baseline. All responses are JSON.

## Local Index Model

Message storage is split:

```text
telegram_messages          -> metadata
telegram_message_contents  -> text and raw_json
telegram_sync_cursors      -> per account/channel cursor state
```

`telegram_messages_fts` indexes only persisted local content from `telegram_message_contents.text`. Remote Telegram search results are display-only and are not inserted into `telegram_messages`, `telegram_message_contents`, `telegram_links`, `telegram_files`, or FTS.

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

### `POST /api/setup/telegram-api`

Stores Telegram API credentials for first-run setup. `app_hash` is write-only and is never returned.

Request:

```json
{
  "app_id": 123456,
  "app_hash": "your_app_hash"
}
```

Response `200`:

```json
{
  "configured": true,
  "app_id": 123456,
  "app_hash_set": true
}
```

## Telegram Settings

### `GET /api/settings/telegram-api`

Returns redacted Telegram API configuration state.

```json
{
  "configured": true,
  "app_id": 123456,
  "app_hash_set": true
}
```

### `PUT /api/settings/telegram-api`

Updates Telegram API credentials. The response is the same redacted shape as `GET /api/settings/telegram-api`.

## Telegram Login

Telegram onboarding creates or updates an account, stores the local session path, and starts metadata-only channel sync after successful login. It does not sync message history.

### `POST /api/telegram/login/send-code`

Request:

```json
{
  "phone": "+10000000000"
}
```

Response `200`:

```json
{
  "status": "LOGIN_REQUIRED"
}
```

### `POST /api/telegram/login/sign-in`

Request:

```json
{
  "phone": "+10000000000",
  "code": "12345"
}
```

Response `200`:

```json
{
  "status": "ONLINE",
  "account": {
    "id": 1,
    "phone": "+10000000000",
    "telegram_user_id": 42,
    "first_name": "Ada",
    "last_name": "Lovelace",
    "username": "ada",
    "status": "ONLINE",
    "session_path": "/data/tg-search/sessions/1.session",
    "last_online_at": "2026-06-08T02:00:00Z",
    "last_error": ""
  },
  "metadata_sync": {
    "status": "succeeded",
    "channel_count": 3
  }
}
```

If 2FA is required, response `202`:

```json
{
  "status": "LOGIN_REQUIRED",
  "password_required": true
}
```

### `POST /api/telegram/login/password`

Submits the 2FA password for a pending login.

Request:

```json
{
  "phone": "+10000000000",
  "password": "2fa-password"
}
```

Response `200` uses the same successful login shape as `/api/telegram/login/sign-in`.

If metadata sync fails after login, the account remains `ONLINE`, `account.last_error` is stored, and `metadata_sync.status` is `failed`.

## Accounts

### `GET /api/accounts`

Returns Telegram accounts.

```json
{
  "items": [
    {
      "id": 1,
      "phone": "+10000000000",
      "telegram_user_id": 42,
      "first_name": "Ada",
      "last_name": "Lovelace",
      "username": "ada",
      "status": "ONLINE",
      "session_path": "/data/tg-search/sessions/1.session",
      "last_online_at": "2026-06-08T02:00:00Z",
      "last_error": ""
    }
  ]
}
```

### `DELETE /api/accounts/:id`

Stops account runtime state, removes the local session file, and deletes the account row.

### `POST /api/accounts/:id/channels/sync-metadata`

Runs metadata-only channel sync for an account. The sync stores channel title, username, type, member count, description, avatar state, `sync_state="metadata_only"`, and `listen_state="disabled"`. It does not fetch message history.

### `GET /api/channels?account_id=1`

Returns channels for an account. Omit `account_id` to list all channels.

### `PATCH /api/channels/:id/control`

Updates per-channel control settings.

Sync Profiles:

```text
Quick  -> latest 100 messages
Normal -> latest 1000 messages
Deep   -> latest 10000 messages
Full   -> all available history
```

Request:

```json
{
  "history_sync_enabled": true,
  "sync_profile": "Normal",
  "listen_enabled": false,
  "remote_search_allowed": true
}
```

Deep and Full changes check DB storage quota before saving. If DB usage is at or above `storage.max_db_size`, the API returns `409` with `storage_quota_exceeded`.

### `POST /api/channels/web-access/check`

Runs Telegram Web Access Detection for selected channels.

This check only determines whether a public username channel can be viewed through:

```text
https://t.me/s/{username}
```

The detector parses the page for `tgme_widget_message_wrap`. It does not mean Google/Bing indexing, PanSou indexing, Telegram public searchability, or complete content access.

Request:

```json
{
  "channel_ids": [1, 2]
}
```

### `POST /api/channels/:id/analyze`

Returns lightweight channel analysis from stored metadata and existing local counts only. It does not fetch Telegram history.

Response includes:

```json
{
  "channel": {},
  "control": {},
  "watch_rule": null,
  "indexed_counts": {
    "messages": 0,
    "links": 0,
    "files": 0
  }
}
```

## Listen Rules

### `POST /api/watch-rules`

Creates a listen rule.

```json
{
  "channel_id": 1,
  "enabled": true,
  "includes": ["电影", "课程"],
  "excludes": ["广告"],
  "message_types": ["text", "file"],
  "link_types": ["cloud_drive", "magnet", "ed2k", "http"]
}
```

`PUT /api/watch-rules/:id` uses the same payload. `GET /api/watch-rules` and `GET /api/watch-rules/:id` return these fields.

## Search

### `GET /api/search/global?q=ubuntu`

Returns grouped local search results.

```json
{
  "messages": { "items": [], "total": 0 },
  "links": { "items": [], "total": 0 },
  "files": { "items": [], "total": 0 },
  "channels": { "items": [], "total": 0 }
}
```

Scoped endpoints use the same filters (`q`, `account_id`, `channel_id`, `limit`, `offset`, date range where applicable):

```text
GET /api/search/messages
GET /api/search/links
GET /api/search/files
GET /api/search/channels
```

Legacy endpoints remain available:

```text
GET /api/search
GET /api/messages/latest
GET /api/links
GET /api/links/merged
```

## Remote Search

### `POST /api/search/remote`

Executes a display-only Telegram remote search for an unsynced channel.

Constraints:

- `query` must be non-empty.
- `channel_id` must reference an unsynced channel.
- `remote_search_allowed` must be `true`.
- Results are stored only in short-lived memory and do not write local index rows.

Request:

```json
{
  "channel_id": 1,
  "query": "ubuntu iso"
}
```

Response `202`:

```json
{
  "id": 1,
  "status": "queued",
  "source": "remote",
  "expires_at": "2026-06-08T10:30:00Z"
}
```

### `GET /api/search/remote/:task_id`

Returns temporary remote results:

```json
{
  "task": {},
  "items": [
    {
      "source": "remote",
      "channel_id": 1,
      "telegram_message_id": 99,
      "text": "ubuntu iso"
    }
  ]
}
```

## Resources

### `GET /api/resources`

Returns the Telegram Resource Library from local indexed links and files. Duplicate links are collapsed by URL, keeping the newest source message.

Filters:

```text
q
type
category
channel_id
account_id
extension
sort
limit
offset
```

Resource groups:

```text
cloud_drive
magnet
ed2k
http
files
```

Response:

```json
{
  "items": [],
  "total": 0,
  "grouped": {
    "cloud_drive": 0,
    "magnet": 0,
    "ed2k": 0,
    "http": 0,
    "files": 0
  }
}
```

### `GET /api/resources/grouped`

Returns grouped resource counts.

### `GET /api/resources/:id`

Returns one resource item from the current local resource library.

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

## Search And Resources

Message search, link, maintenance, and backup endpoints from the existing backend remain available while later phases reshape them around Global Search and the Telegram Resource Library.
