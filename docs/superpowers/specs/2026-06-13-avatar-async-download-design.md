# Avatar Async Download Design

## Background

Previous avatar implementation used on-demand proxy downloads that blocked web requests, causing queue issues. This design implements async background download with local file storage.

## Requirements

- Account login triggers immediate avatar download to local storage
- Channel sync triggers batch avatar downloads for all channels
- Manual sync button for re-downloading avatars
- Frontend displays local file or placeholder (first letter circle)
- Infinite cache (no TTL expiration)

## Architecture

### Storage Layer

**Path structure:** `storage.Path/avatars/{type}/{id}/{photoID}.jpg`
- `type`: `account` | `channel`
- Example: `data/tg-search/avatars/account/123/456789.jpg`
- PhotoID changes preserve old files (history retained, optional cleanup later)

### Download Queue

**Implementation:** Dedicated `avatarQueue` using existing `scheduler.RetryQueue`

**Task structure:**
```go
type AvatarDownloadTask struct {
    Type     string // "account" | "channel"
    ID       int64  // account.ID or channel.ID
    PhotoID  int64
    Priority int    // manual trigger = high priority
}
```

**Concurrency:** Reuse existing `AvatarLimiter` (20 concurrent downloads)

**Retry policy:** Exponential backoff (1s, 2s, 4s), max 3 retries

**Deduplication:** Skip if `(type, id, photoID)` file already exists

### Trigger Points

**Auto-trigger:**
1. Account login success → check `account.PhotoID > 0` → enqueue task
2. Channel sync complete → batch check `channel.PhotoID > 0` → enqueue tasks

**Manual trigger:**
- API: `POST /api/admin/accounts/:id/sync-avatar` → 202 Accepted
- Frontend: "Sync Avatar" button (visible only when `PhotoID > 0`)

**Channel sync exclusion:** No separate channel sync button; channel avatars auto-trigger after account syncs channel list.

## Data Flow

### Download Flow
1. Trigger source (login/sync/manual) → submit task to `avatarQueue`
2. Queue worker dequeues → call `telegram.Client.DownloadUserAvatar` or `DownloadChannelAvatar`
3. Success → atomic write to `avatars/{type}/{id}/{photoID}.jpg`
4. Failure → retry queue (max 3 attempts)

### Frontend Display Flow
1. Load account/channel list → check `PhotoID > 0`
2. Request: `GET /api/admin/accounts/:id/avatar` or `/api/admin/channels/:id/avatar`
3. Backend logic:
   - Check local file `avatars/{type}/{id}/{photoID}.jpg` exists
   - **Exists** → return file with `Cache-Control: public, max-age=31536000, immutable` + ETag
   - **Not exists** → return 404
4. Frontend:
   - 200/304: display avatar image
   - 404: display first-letter placeholder

### API Endpoints

**Read avatar (modify existing):**
- `GET /api/admin/accounts/:id/avatar` → check local file first, fallback to on-demand download
- `GET /api/admin/channels/:id/avatar` → check local file first, fallback to on-demand download

**Manual sync (new):**
- `POST /api/admin/accounts/:id/sync-avatar` → enqueue task, return 202 Accepted

## Error Handling

### Download Failures

| Scenario | Action |
|----------|--------|
| Network timeout | Retry queue with exponential backoff |
| Telegram API error (auth/permission) | Log error, stop retry |
| Disk full | Log error, stop retry |
| File write failure | Retry once |

### Frontend Degradation
- Local file missing → display first-letter placeholder
- No error UI needed

### Queue Lifecycle
- Process restart → in-flight tasks lost (acceptable; retrigger on next login/manual sync)
- Duplicate task → skip if file exists

## Component Changes

### Backend

**New packages:**
- `internal/avatar/service.go` — avatar download service
- `internal/avatar/queue.go` — task queue wrapper
- `internal/avatar/storage.go` — file path helpers

**Modified packages:**
- `internal/telegram/gotd_client.go` — account login hook, channel sync hook
- `internal/api/account_avatar.go` — prioritize local file
- `internal/api/channel_avatar.go` — prioritize local file
- `internal/api/handlers.go` — add manual sync endpoint
- `cmd/tg-search/main.go` — init avatarQueue

### Frontend

**Modified files:**
- `web/src/views/AccountsView.vue` — add "Sync Avatar" button, lazy-load avatar images
- `web/src/views/ChannelsView.vue` — lazy-load avatar images (no sync button)
- `web/src/api/admin.ts` — add `syncAccountAvatar()` API call

## Testing

### Unit Tests
- Avatar task enqueue/dequeue logic
- File path generation (`avatars/{type}/{id}/{photoID}.jpg`)
- PhotoID change deduplication
- Retry policy behavior

### Integration Tests
- Account login → avatar download triggered
- Channel sync → batch avatar downloads triggered
- Manual sync API → 202 response, task enqueued
- GET avatar API → local file returned, 304 with ETag

### Manual Verification
1. Login account → verify `avatars/account/{id}/{photoID}.jpg` created
2. Sync channels → verify multiple `avatars/channel/{id}/{photoID}.jpg` files
3. Click "Sync Avatar" button → verify file re-downloaded
4. Refresh page → verify 304 Not Modified responses
5. PhotoID changes → verify new file created, old file retained

## Migration

**Data migration:** None required (avatars download on-demand)

**Config changes:** None required (reuse existing `storage.Path`)

**Rollback plan:** Remove avatar download hooks, frontend falls back to placeholder display
