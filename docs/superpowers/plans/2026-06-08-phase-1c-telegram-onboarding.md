# Phase 1C Telegram Onboarding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Telegram API configuration, phone/code/2FA login, account state, and metadata-only channel sync after successful login.

**Architecture:** Build on Phase 1A setup/settings/auth APIs and Phase 1B Vue shell. Keep onboarding conservative: login creates or updates a Telegram account and starts metadata sync for channels, groups, and Saved Messages, but never fetches message history. Sensitive values are write-only and never returned in API responses.

**Tech Stack:** Go 1.25, Gin, SQLite, existing repository/service/API test patterns, gotd/td adapter boundary, Vue 3, TypeScript, Pinia, Vue Router, Naive UI, Vitest.

---

## Prerequisite

Complete these plans first:

- [Phase 1A Foundation Plan](/home/harold/workspace/telegram-search/docs/superpowers/plans/2026-06-08-phase-1a-foundation.md)
- [Phase 1B Admin Shell Plan](/home/harold/workspace/telegram-search/docs/superpowers/plans/2026-06-08-phase-1b-admin-shell.md)

This plan assumes the project has already been renamed to `tg-search` and the fresh database baseline is allowed to change without compatibility migrations.

## Scope

In scope:

- Store Telegram API `app_id` and `app_hash` in settings.
- Expose Telegram API setup/settings endpoints with redacted responses.
- Replace old login routes with `/api/telegram/login/send-code`, `/api/telegram/login/sign-in`, and `/api/telegram/login/password`.
- Keep login codes, 2FA passwords, API hash, and session contents out of responses and logs.
- Add account response fields needed by the Accounts UI: `session_path`, `last_online_at`, `last_error`.
- Expand channel metadata to match the product baseline: `member_count`, `description`, `avatar_state`, `sync_state`, `listen_state`.
- Start metadata-only sync after successful Telegram login.
- Add setup wizard steps for Telegram API and Telegram Login.
- Add Accounts page list/status behavior.

Out of scope:

- Channel selection, Sync Profile, listen rules, and remote-search controls. These are Phase 1D.
- Message history sync, message contents split, sync cursors, FTS, Global Search, and Resources. These are Phase 1E.
- Persistent task runtime, SSE, retry/cancel, FloodWait resume, and restart recovery. These are Phase 1F.

## API Contract

Add or update these endpoints:

- `POST /api/setup/telegram-api`
- `GET /api/settings/telegram-api`
- `PUT /api/settings/telegram-api`
- `POST /api/telegram/login/send-code`
- `POST /api/telegram/login/sign-in`
- `POST /api/telegram/login/password`
- `GET /api/accounts`
- `DELETE /api/accounts/:id`
- `POST /api/accounts/:id/channels/sync-metadata`
- `GET /api/channels?account_id=123`

Telegram API responses return:

```json
{
  "configured": true,
  "app_id": 123456,
  "app_hash_set": true
}
```

They never return `app_hash`.

Successful login responses return:

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
    "last_online_at": "2026-06-08T10:00:00Z",
    "last_error": ""
  },
  "metadata_sync": {
    "status": "succeeded",
    "channel_count": 3
  }
}
```

If 2FA is required, `sign-in` returns HTTP `202`:

```json
{
  "status": "LOGIN_REQUIRED",
  "password_required": true
}
```

## File Structure

- Modify `internal/model/model.go`: add Telegram API settings response, account fields, channel metadata fields.
- Modify `internal/db/migrations.go`: update fresh schema baseline for account/channel metadata fields.
- Modify `internal/repository/settings.go`: typed helpers for Telegram API settings.
- Modify `internal/repository/account.go`: persist and scan new account fields.
- Modify `internal/repository/channel.go`: persist and scan new metadata fields.
- Modify `internal/telegram/client.go`: extend `telegram.Channel` metadata fields.
- Modify `internal/channel/service.go`: metadata-only sync using expanded channel metadata.
- Modify `internal/api/router.go`: add Telegram route group and remove old `/api/login/*` registration.
- Modify `internal/api/handlers.go`: Telegram API settings and login handlers.
- Modify `internal/api/handlers_test.go`: route, redaction, login, and metadata sync tests.
- Modify `internal/channel/service_test.go`: metadata sync tests.
- Modify `web/src/api/types.ts`: Telegram settings, login, account, and channel types.
- Create `web/src/stores/telegram.ts`: Telegram API and login flow state.
- Modify `web/src/stores/setup.ts`: include Telegram setup status.
- Modify `web/src/router/index.ts`: setup wizard routes for Telegram API/login.
- Create `web/src/views/SetupTelegramApiView.vue`.
- Create `web/src/views/SetupTelegramLoginView.vue`.
- Create `web/src/views/AccountsView.vue`.
- Add frontend tests beside changed stores/views.
- Modify `docs/api.md` and `README.md`.

## Task 1: Telegram API Settings

**Files:**

- Modify: `internal/model/model.go`
- Modify: `internal/repository/settings.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/handlers.go`
- Test: `internal/api/handlers_test.go`
- Test: `internal/repository/settings_test.go`

- [ ] **Step 1: Write settings repository tests**

Add tests that save `app_id=123456` and `app_hash="hash-secret"`, load the raw settings internally, and verify the public response has `app_hash_set=true` with no hash string.

Run:

```bash
go test ./internal/repository -run 'TestTelegramAPISettings' -v
```

Expected: FAIL because typed Telegram API settings helpers do not exist.

- [ ] **Step 2: Implement typed settings helpers**

Add these model types:

```go
type TelegramAPISettings struct {
	AppID   int    `json:"app_id"`
	AppHash string `json:"-"`
}

type TelegramAPISettingsResponse struct {
	Configured bool `json:"configured"`
	AppID      int  `json:"app_id"`
	AppHashSet bool `json:"app_hash_set"`
}
```

Add repository methods:

```go
func (r *SettingsRepository) SaveTelegramAPI(ctx context.Context, settings model.TelegramAPISettings) error
func (r *SettingsRepository) LoadTelegramAPI(ctx context.Context) (model.TelegramAPISettings, error)
func RedactTelegramAPI(settings model.TelegramAPISettings) model.TelegramAPISettingsResponse
```

Store the value under settings key `telegram_api`.

- [ ] **Step 3: Add API handler tests**

Add tests for:

- `POST /api/setup/telegram-api` accepts `{"app_id":123456,"app_hash":"hash-secret"}`.
- `GET /api/settings/telegram-api` returns `configured=true`, `app_hash_set=true`, and does not contain `hash-secret`.
- `PUT /api/settings/telegram-api` updates the value after setup.

Run:

```bash
go test ./internal/api -run 'TestTelegramAPISettings' -v
```

Expected: FAIL because routes do not exist.

- [ ] **Step 4: Implement routes and handlers**

Register:

```go
api.POST("/setup/telegram-api", h.saveSetupTelegramAPI)
api.GET("/settings/telegram-api", h.getTelegramAPISettings)
api.PUT("/settings/telegram-api", h.updateTelegramAPISettings)
```

Validation rules:

- `app_id` must be greater than zero.
- `app_hash` must be non-empty when saving a new setting.
- Responses must use `TelegramAPISettingsResponse`.

- [ ] **Step 5: Verify**

Run:

```bash
go test ./internal/repository ./internal/api
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/model/model.go internal/repository/settings.go internal/repository/settings_test.go internal/api/router.go internal/api/handlers.go internal/api/handlers_test.go
git commit -m "feat: add telegram api settings"
```

## Task 2: Account And Channel Metadata Baseline

**Files:**

- Modify: `internal/db/migrations.go`
- Modify: `internal/model/model.go`
- Modify: `internal/repository/account.go`
- Modify: `internal/repository/channel.go`
- Test: `internal/repository/account_test.go`
- Test: `internal/repository/channel_test.go`

- [ ] **Step 1: Write repository tests for expanded fields**

Add account test data with:

```go
model.Account{
	Phone: "+10000000000",
	Status: model.AccountStatusOnline,
	SessionPath: "/data/tg-search/sessions/1.session",
	LastOnlineAt: ptrTime(now),
	LastError: "",
}
```

Add channel test data with:

```go
model.Channel{
	AccountID: accountID,
	TelegramChannelID: 200,
	AccessHash: 300,
	Title: "VIP",
	Username: "vip",
	Type: model.ChannelTypeChannel,
	MemberCount: 1234,
	Description: "private resource channel",
	AvatarState: "unknown",
	SyncState: "metadata_only",
	ListenState: "disabled",
}
```

Run:

```bash
go test ./internal/repository -run 'Test.*Expanded.*Metadata' -v
```

Expected: FAIL because fields and columns are missing.

- [ ] **Step 2: Update fresh schema**

In `telegram_accounts`, add:

```sql
session_path TEXT NOT NULL DEFAULT '',
last_online_at DATETIME,
last_error TEXT NOT NULL DEFAULT ''
```

In `telegram_channels`, add:

```sql
member_count INTEGER NOT NULL DEFAULT 0,
description TEXT NOT NULL DEFAULT '',
avatar_state TEXT NOT NULL DEFAULT 'unknown',
sync_state TEXT NOT NULL DEFAULT 'metadata_only',
listen_state TEXT NOT NULL DEFAULT 'disabled',
web_access_error TEXT NOT NULL DEFAULT ''
```

- [ ] **Step 3: Update model and repositories**

Add Go fields with JSON names matching the schema. Update all `INSERT`, `UPDATE`, `SELECT`, and scan helpers so saved values round-trip.

- [ ] **Step 4: Verify repositories**

Run:

```bash
go test ./internal/db ./internal/repository
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/db/migrations.go internal/model/model.go internal/repository/account.go internal/repository/channel.go internal/repository/account_test.go internal/repository/channel_test.go
git commit -m "feat: expand telegram account and channel metadata"
```

## Task 3: Telegram Login API Rename And Redaction

**Files:**

- Modify: `internal/api/router.go`
- Modify: `internal/api/handlers.go`
- Modify: `internal/redact/redact.go`
- Test: `internal/api/handlers_test.go`

- [ ] **Step 1: Write route tests**

Update existing login tests to call:

```text
POST /api/telegram/login/send-code
POST /api/telegram/login/sign-in
POST /api/telegram/login/password
```

Add a negative test that `POST /api/login/send-code` returns `404`.

Run:

```bash
go test ./internal/api -run 'TestTelegramLoginRoutes' -v
```

Expected: FAIL because routes still use old paths.

- [ ] **Step 2: Register Telegram login routes**

In `NewRouter`, register:

```go
telegramAPI := api.Group("/telegram")
telegramAPI.POST("/login/send-code", h.sendCode)
telegramAPI.POST("/login/sign-in", h.signIn)
telegramAPI.POST("/login/password", h.password)
```

Remove old `/api/login/*` route registration.

- [ ] **Step 3: Redact sensitive request fields**

Ensure the redaction list includes:

```go
"app_hash"
"code"
"login_code"
"password"
"phone_code_hash"
"session"
```

- [ ] **Step 4: Verify**

Run:

```bash
go test ./internal/api ./internal/redact
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/router.go internal/api/handlers.go internal/api/handlers_test.go internal/redact/redact.go
git commit -m "feat: rename telegram login api"
```

## Task 4: Metadata Sync After Login

**Files:**

- Modify: `internal/telegram/client.go`
- Modify: `internal/channel/service.go`
- Modify: `internal/api/handlers.go`
- Test: `internal/channel/service_test.go`
- Test: `internal/api/handlers_test.go`

- [ ] **Step 1: Write channel service test**

Create a fake Telegram client returning a channel, supergroup, and Saved Messages. Verify `SyncAccount` saves all three with expanded metadata and does not call `FetchHistory`.

Run:

```bash
go test ./internal/channel -run 'TestSyncAccountStoresExpandedMetadataOnly' -v
```

Expected: FAIL until metadata fields are mapped and history is not touched.

- [ ] **Step 2: Extend Telegram channel adapter type**

Update `telegram.Channel`:

```go
type Channel struct {
	TelegramChannelID int64
	AccessHash int64
	Title string
	Username string
	Type string
	MemberCount int64
	Description string
	AvatarState string
}
```

- [ ] **Step 3: Map metadata into `model.Channel`**

Set defaults:

```go
SyncState: "metadata_only"
ListenState: "disabled"
AvatarState: firstNonEmpty(item.AvatarState, "unknown")
```

- [ ] **Step 4: Write API login sync test**

After successful `sign-in`, verify the response contains `metadata_sync.channel_count` and the repository contains channel rows. Verify message count remains zero.

Run:

```bash
go test ./internal/api -run 'TestTelegramSignInStartsMetadataSyncOnly' -v
```

Expected: FAIL until handler calls channel metadata sync.

- [ ] **Step 5: Trigger metadata sync after profile update**

In the successful `sign-in` and `password` paths:

- Update account status to `ONLINE`.
- Set `last_online_at` to current UTC time.
- Call `ChannelSync.SyncAccount(ctx, account)`.
- Return `metadata_sync.status="succeeded"` and `channel_count`.
- If metadata sync fails, keep account `ONLINE`, store `last_error`, and return HTTP `200` with `metadata_sync.status="failed"`.

- [ ] **Step 6: Verify**

Run:

```bash
go test ./internal/channel ./internal/api ./internal/repository
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/telegram/client.go internal/channel/service.go internal/channel/service_test.go internal/api/handlers.go internal/api/handlers_test.go
git commit -m "feat: sync telegram metadata after login"
```

## Task 5: Frontend Telegram Setup And Accounts

**Files:**

- Modify: `web/src/api/types.ts`
- Create: `web/src/stores/telegram.ts`
- Modify: `web/src/stores/setup.ts`
- Modify: `web/src/router/index.ts`
- Create: `web/src/views/SetupTelegramApiView.vue`
- Create: `web/src/views/SetupTelegramLoginView.vue`
- Create: `web/src/views/AccountsView.vue`
- Test: `web/src/stores/telegram.test.ts`
- Test: `web/src/views/SetupTelegramApiView.test.ts`
- Test: `web/src/views/SetupTelegramLoginView.test.ts`
- Test: `web/src/views/AccountsView.test.ts`

- [ ] **Step 1: Add frontend types**

Add interfaces:

```ts
export interface TelegramAPISettingsResponse {
  configured: boolean
  app_id: number
  app_hash_set: boolean
}

export interface TelegramAccount {
  id: number
  phone: string
  telegram_user_id: number
  first_name: string
  last_name: string
  username: string
  status: string
  last_online_at?: string
  last_error: string
}

export interface TelegramLoginResponse {
  status: string
  password_required?: boolean
  account?: TelegramAccount
  metadata_sync?: {
    status: string
    channel_count: number
  }
}
```

- [ ] **Step 2: Write store tests**

Test that `saveTelegramAPI`, `sendCode`, `signIn`, `submitPassword`, and `loadAccounts` call the exact API paths from this plan and keep `passwordRequired` state when HTTP `202` is returned.

Run:

```bash
npm run web:test -- telegram
```

Expected: FAIL until the store exists.

- [ ] **Step 3: Implement `telegram` Pinia store**

Store state:

```ts
settings: TelegramAPISettingsResponse | null
accounts: TelegramAccount[]
phone: string
passwordRequired: boolean
loading: boolean
error: string
```

Actions call the endpoints listed in the API Contract section.

- [ ] **Step 4: Add setup routes**

Add routes:

```ts
{ path: '/setup/telegram-api', name: 'setup-telegram-api', component: SetupTelegramApiView }
{ path: '/setup/telegram-login', name: 'setup-telegram-login', component: SetupTelegramLoginView }
{ path: '/accounts', name: 'accounts', component: AccountsView }
```

Setup flow order is Admin Account -> API Key -> Telegram API -> Telegram Login -> Home.

- [ ] **Step 5: Build setup views**

`SetupTelegramApiView.vue` contains fields for App ID and App Hash and submits to `POST /api/setup/telegram-api`.

`SetupTelegramLoginView.vue` contains phone, code, and 2FA password states. It displays metadata sync result after successful login.

`AccountsView.vue` lists account phone, username, status, last online time, and last error.

- [ ] **Step 6: Verify frontend**

Run:

```bash
npm run web:typecheck
npm run web:test
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add web/src/api/types.ts web/src/stores web/src/router/index.ts web/src/views
git commit -m "feat: add telegram onboarding ui"
```

## Task 6: Documentation And Final Verification

**Files:**

- Modify: `docs/api.md`
- Modify: `README.md`

- [ ] **Step 1: Update API docs**

Document the Telegram API settings endpoints, Telegram login endpoints, account response fields, and metadata sync behavior. Explicitly state: login syncs metadata only and does not sync message history.

- [ ] **Step 2: Update README setup flow**

Document first-run steps:

```text
Admin Account -> API Key -> Telegram API -> Telegram Login -> Home
```

- [ ] **Step 3: Run full phase verification**

Run:

```bash
go test ./...
npm run web:typecheck
npm run web:test
rg -n 'api_hash|hash-secret|login code|2FA password' docs README.md internal web
```

Expected:

- Go tests pass.
- Frontend typecheck and tests pass.
- `rg` shows only documentation text and redaction test fixtures, not handler responses that expose secrets.

- [ ] **Step 4: Commit**

```bash
git add docs/api.md README.md
git commit -m "docs: document telegram onboarding"
```

## Self-Review Checklist

- [ ] No old `/api/login/*` route remains registered.
- [ ] `app_hash`, login code, 2FA password, and session contents are never returned.
- [ ] Successful login starts metadata sync only.
- [ ] No message history sync API is called by onboarding.
- [ ] Channel rows default to `sync_state="metadata_only"` and `listen_state="disabled"`.
- [ ] Setup wizard includes Telegram API and Telegram Login steps.
