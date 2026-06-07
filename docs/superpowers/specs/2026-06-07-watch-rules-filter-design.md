# Watch Rules Filter Design

## Goal

Add database-backed watch rules so users can target specific Telegram channels and keep only resource messages that contain links and match configured include/exclude keywords.

The rules apply to both realtime updates and manual history sync, with different handling for the `enabled` flag:

- Realtime updates use only enabled rules.
- Manual history sync ignores `enabled` when a rule exists.

## Scope

Included:

- Persistent watch rule storage in SQLite.
- API endpoints to list, create, read, update, and delete watch rules.
- Shared filtering logic for realtime update processing and history sync.
- Link-required filtering when a rule applies.
- Include/exclude keyword filtering.
- Tests for repository behavior, API validation, realtime filtering, and history sync filtering.

Excluded:

- Automatic cleanup of already stored messages when a rule changes.
- UI for managing rules.
- Per-keyword matching modes such as regex, phrase boundaries, or case-sensitive matching.
- Background resync when watch rules are created or updated.

## Data Model

Add a `telegram_watch_rules` table:

```sql
CREATE TABLE telegram_watch_rules (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  channel_id INTEGER NOT NULL UNIQUE,
  enabled INTEGER NOT NULL DEFAULT 1,
  includes_json TEXT NOT NULL DEFAULT '[]',
  excludes_json TEXT NOT NULL DEFAULT '[]',
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  FOREIGN KEY(channel_id) REFERENCES telegram_channels(id) ON DELETE CASCADE
);
```

`channel_id` references the existing local `telegram_channels.id`. Each channel can have at most one rule. Include and exclude terms are stored as JSON arrays of strings.

The repository normalizes rule terms by trimming whitespace, dropping empty strings, and preserving user-provided order. Duplicate terms are allowed but have no extra effect.

## API

Add watch rule endpoints under `/api`:

- `GET /api/watch-rules`
- `POST /api/watch-rules`
- `GET /api/watch-rules/{id}`
- `PUT /api/watch-rules/{id}`
- `DELETE /api/watch-rules/{id}`

Create/update request:

```json
{
  "channel_id": 1,
  "enabled": true,
  "includes": ["庆余年", "1080p"],
  "excludes": ["预告", "花絮"]
}
```

`channel_id` is required and must reference an existing channel. `enabled` defaults to `true` on create when omitted. `includes` and `excludes` default to empty arrays.

Responses return the persisted rule with `id`, `channel_id`, `enabled`, `includes`, `excludes`, `created_at`, and `updated_at`.

## Filtering Semantics

When a watch rule applies:

1. Extract links with the existing link extractor.
2. Reject the message if no links are found.
3. If `includes` is non-empty, accept only when the message text contains at least one include term.
4. Reject when the message text contains any exclude term.
5. Matching is case-insensitive.

An empty `includes` list means link-only filtering. An empty `excludes` list means nothing is excluded.

## Realtime Data Flow

The gotd listener remains account-level and still receives updates for all known Telegram channels. Filtering happens in `update.Processor` after the local channel is resolved.

For new and edited message events:

- If the channel has no enabled rule, skip the message.
- If the channel has an enabled rule, apply link/include/exclude filtering.
- If the message passes, save it and replace its links using the existing transaction flow.
- If an edited message in an enabled-rule channel no longer passes, mark the stored message deleted so search results do not retain stale content.

The edit-event deletion above is event-driven cleanup for a message whose current Telegram content no longer matches. It is not a background sweep after rule changes.

Delete events keep the existing behavior: when the local channel is known, mark the message deleted. Delete events do not have text or links, so keyword filtering does not apply.

## History Sync Data Flow

Manual history sync continues to fetch channel history through the existing `history.Service`.

For each fetched message:

- If the channel has no watch rule, preserve existing behavior and sync the message normally.
- If the channel has a watch rule, ignore `enabled` and apply link/include/exclude filtering.
- Only passing messages are saved and have links persisted.

Rule changes do not trigger cleanup. If a message was already stored and later no longer matches a changed rule, it remains until explicitly deleted by a Telegram delete event, overwritten by a matching edit, or otherwise handled by future maintenance work.

## Error Handling

Malformed rule JSON in SQLite is treated as a repository error and returned to the caller or logged by the background processor. API validation rejects missing or invalid `channel_id` values and non-array include/exclude payloads.

Realtime processor errors are logged and do not crash the listener loop, matching current behavior. History sync reports repository/filtering errors as sync failures through the existing sync API response or retry queue.

## Testing

Repository tests cover create/update/delete, one-rule-per-channel behavior, cascade deletion, JSON term round trips, and term normalization.

API tests cover request validation, default values, CRUD responses, and duplicate channel rule handling.

Realtime processor tests cover:

- New messages from channels without enabled rules are skipped.
- New messages with enabled rules require links and include matches.
- Exclude matches reject messages.
- Edited messages that stop matching are marked deleted.
- Delete events still mark stored messages deleted.

History tests cover:

- Channels without rules still sync all fetched messages.
- Channels with rules filter messages even when `enabled=false`.
- Link extraction is reused for persisted links.
- Non-matching messages are not inserted during history sync.
