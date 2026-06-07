# API Response Contract

This document records the stable JSON shapes used by AList-TVBox and other HTTP clients.

## Error Response

All API errors use one envelope:

```json
{
  "error": {
    "code": "bad_request",
    "message": "phone is required"
  }
}
```

Rules:

- 4xx validation and not-found errors use `code: "bad_request"` unless a more specific code is added later.
- 5xx internal errors use `code: "internal_error"`.
- Sensitive request values such as `api_hash`, login codes, passwords, and session data must not be included in error messages or logs.

## Status Response

`GET /api/status`

```json
{
  "service": "ok",
  "accounts": 1,
  "channels": 2,
  "messages": 100,
  "links": 30,
  "account_states": {
    "ONLINE": 1
  }
}
```

Empty state summaries are returned as `{}`. Count fields are always numbers.

## Search Result

`GET /api/search?q=keyword`

The response is:

```json
{
  "items": [
    {
      "id": 1,
      "account_id": 1,
      "channel_id": 1,
      "telegram_message_id": 10,
      "sender_id": 7,
      "text": "message text",
      "raw_json": "{}",
      "date": "2026-01-01T00:00:00Z",
      "edit_date": "2026-01-01T00:01:00Z",
      "deleted": false,
      "created_at": "2026-01-01T00:00:00Z",
      "updated_at": "2026-01-01T00:00:00Z",
      "account_phone": "+10000000000",
      "account_username": "user",
      "account_first_name": "First",
      "channel_title": "Channel",
      "channel_username": "channel",
      "links": []
    }
  ]
}
```

Rules:

- `items` is always an array.
- `links` is an array. It may be empty.
- `edit_date` is omitted when Telegram does not provide an edit timestamp.
- Deleted messages are filtered from search and latest-message responses.

## Link Result

`GET /api/links`

Each item includes link fields plus message context:

- `id`
- `message_id`
- `type`
- `url`
- `password`, omitted when empty
- `note`, omitted when empty
- `created_at`
- `message_text`
- `message_date`
- `account_id`
- `channel_id`
- `channel_title`
- `telegram_message_id`

`GET /api/links/merged`

Returns links grouped by provider type:

- `total`: number of returned merged links after filtering, deduplication, and pagination
- `merged_by_type`: object keyed by link type

Each merged link includes:

- `url`
- `password`, omitted when empty
- `note`, omitted when empty
- `datetime`
- `source`
- `channel_id`
- `telegram_message_id`

## Channel Result

`GET /api/channels`

Each item includes:

- `id`
- `account_id`
- `telegram_channel_id`
- `access_hash`
- `title`
- `username`
- `type`
- `last_message_id`
- `last_sync_time`, omitted when never synced
- `created_at`
- `updated_at`

## Async Job Response

Manual sync APIs return immediately when the retry queue is enabled:

```json
{
  "job_id": "1",
  "status": "queued"
}
```

Job status values are `queued`, `running`, `succeeded`, and `failed`.
