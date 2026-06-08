# Setup Channel Onboarding Design

## Goal

Make Telegram setup responsive for accounts with hundreds of channels by moving slow channel metadata and web-access work into background tasks, reducing per-channel save calls, and making channel selection readable and fast.

## Architecture

Login updates the Telegram account and returns immediately. Channel metadata sync is enqueued after login, and that metadata sync schedules web-access detection after channels are stored. The setup channel page becomes a status-aware selection surface that loads current channel rows, shows public/web-access/blocked states, and avoids expensive per-row controls.

Channel selection saves selected channels through one batch control endpoint, uses the setup listen rules as one global filter configuration, and enqueues the first history sync through the existing channel sync endpoint. Per-channel watch-rule creation is removed from the setup flow.

## API Changes

- Add `PATCH /api/channels/control` with `channel_ids` and a shared `control` payload.
- Keep `POST /api/channels/sync` as the first-history-sync task entry point.
- Change successful Telegram login responses so `metadata_sync.status` is `queued` when async queueing is available.
- After async metadata sync completes, enqueue or run web-access detection for the synced channel IDs.

## Data Flow

1. Telegram login succeeds.
2. Account is marked `ONLINE`.
3. Metadata sync is queued with `context.WithoutCancel`.
4. The login page routes forward without waiting for Telegram channel listing.
5. The channel selection page loads channels from local storage and can be refreshed.
6. Saving selected channels sends one batch control request.
7. The page calls `/api/channels/sync` once for selected channels to create the first history sync task.

## Global Listen Rules

`/api/setup/listen-rules` remains the setup step for global listen rules. Runtime message filtering reads this global setting when no per-channel rule exists. Setup does not create duplicate per-channel watch rules.

## UI

The setup channel page uses a wider full-page layout, removes the history sync dropdown column, truncates long descriptions, displays compact status badges, and limits rendered channel rows through pagination-sized rendering to avoid scroll jank with 300+ channels.

## Testing

Backend tests cover async login metadata queueing, batch channel control, setup listen-rule fallback, and web-access checks after metadata sync. Frontend tests cover setup channel rendering, status labels, no per-channel watch-rule calls, and one batch save plus first history sync call.
