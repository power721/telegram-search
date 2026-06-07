# Channel Web Access Detection Design

## Goal

Add an API that checks whether existing public Telegram channels or supergroups can be viewed through Telegram Web pages, saves the result on the channel row, and includes that result in channel list and detail responses.

The API accepts local `channel_ids` only. It does not accept raw usernames or `t.me` URLs.

## Scope

- Add persistent web access state to `telegram_channels`.
- Add a batch endpoint for checking one or more local channels.
- Return `web_access` from `GET /api/channels` and `GET /api/channels/{id}`.
- Cover repository, service, and API behavior with tests using a fake checker.

Out of scope:

- Background or scheduled rechecks.
- Checking private invite links.
- Accepting channels not already stored in SQLite.
- Storing long-lived HTTP error details in channel list responses.

## Data Model

Add migration `channel_web_access`:

```sql
ALTER TABLE telegram_channels ADD COLUMN web_access INTEGER;
ALTER TABLE telegram_channels ADD COLUMN web_access_checked_at DATETIME;
```

`web_access` is nullable:

- `NULL`: never checked.
- `1`: web page is accessible.
- `0`: web page is not accessible or cannot be checked.

`model.Channel` gains:

```go
WebAccess          *bool      `json:"web_access,omitempty"`
WebAccessCheckedAt *time.Time `json:"web_access_checked_at,omitempty"`
```

The repository scans and saves these fields for reads. Channel metadata upserts keep the existing detection result intact.

## API

Add:

```http
POST /api/channels/web-access/check
Content-Type: application/json

{"channel_ids":[1,2,3]}
```

Success response:

```json
{
  "items": [
    {
      "channel_id": 1,
      "web_access": true,
      "checked_at": "2026-06-07T12:00:00Z"
    }
  ]
}
```

Validation:

- `channel_ids` is required and must be non-empty.
- Every ID must be positive.
- The endpoint validates that every requested local channel exists before performing checks. If any ID is missing, it returns a standard error response and does not write partial results.

`GET /api/channels` and `GET /api/channels/{id}` continue returning channel objects directly. Checked channels include `web_access` and `web_access_checked_at`; unchecked channels omit both fields.

## Detection Behavior

Create a small channel web access service with an injectable checker interface:

```go
type Checker interface {
    Check(ctx context.Context, username string) (bool, error)
}
```

The default checker performs an HTTP request to:

```text
https://t.me/s/{username}?q=
```

Rules:

- Only local channels with a non-empty `username` are checked over HTTP.
- `saved_messages` and channels without `username` are stored as `web_access=false`.
- The response HTML must contain at least one `div.tgme_container div.tgme_widget_message_wrap` element to mean `true`.
- HTTP non-success status, no matching message element, timeout, DNS, and other network errors mean `false`.
- Batch checks run concurrently with a maximum concurrency of 5.
- Each result is saved with a fresh UTC `checked_at`.

The checker uses a bounded timeout so the batch API cannot hang indefinitely on a slow network.

## Components

- `internal/model`: add nullable web access fields to `Channel`.
- `internal/db`: add migration version for the two columns.
- `internal/repository`: scan new fields and add `UpdateWebAccess(ctx, channelID, access, checkedAt)`.
- `internal/channel`: add web access check service and default HTTP checker.
- `internal/api`: add route and handler for `POST /api/channels/web-access/check`.
- `docs/api.md`: document the new endpoint and response field.

## Error Handling

Input errors return standard `400` responses. Missing local channels return a standard not-found style response before any checks run. Internal repository errors return `500`.

HTTP probe failures are not API failures. They are stored as `web_access=false` because the endpoint's job is to persist the current web accessibility result.

## Tests

- Migration test verifies existing DBs receive the new columns.
- Repository test verifies `UpdateWebAccess` persists and channel reads return nullable state.
- Service test verifies:
  - public username calls the checker and stores the result;
  - missing username stores `false` without checker call;
  - duplicate IDs are handled deterministically.
- API test verifies:
  - valid batch request returns items and updates channel list response with `web_access`;
  - invalid body and non-positive IDs return `400`;
  - missing channel IDs return an error without partial updates.
