# API Key Settings Design

## Context

The application already has an `api_keys` table, first-run API key setup routes, and a settings page stub. API keys currently mark the setup step complete, but they are not enforced as credentials for API access. The new behavior makes API keys mandatory for business API access and adds settings-page management.

## Requirements

- The system must always have an active API key after first-run setup reaches the API key step.
- First-run setup automatically generates the API key. Users do not type a name and cannot skip the step.
- Business API requests must include a valid API key.
- The settings page must let an authenticated admin view the full API key at any time.
- The settings page must let an authenticated admin regenerate the API key.
- Regenerating creates a new key and invalidates old keys immediately.
- API keys cannot be deleted or disabled through the UI.
- The API should support both `Authorization: Bearer <key>` and `X-API-Key: <key>`.
- Health, readiness, setup, login, and API key management endpoints remain reachable without an API key where needed to bootstrap and operate the app.

## Recommended Approach

Use a single active API key model. The repository can retain historical rows by disabling old keys during regeneration, but only one key is active at a time.

Store two representations:

- `key_hash`: bcrypt hash used for request authentication.
- `key_ciphertext`: encrypted plaintext used only so the settings page can reveal the full key later.

The encryption key should be an app-local persistent secret stored in the database settings table. It is generated once on demand. This keeps the key viewable across restarts without requiring external configuration. This protects against casual database inspection less than a hardware-backed secret would, but it fits this local/self-hosted app and avoids making setup depend on another secret.

## API Behavior

Add API key management endpoints:

- `GET /api/settings/api-key`: returns metadata for the active key, including full plaintext key for admin UI display.
- `POST /api/settings/api-key/regenerate`: disables existing active keys, creates a new active key, and returns the new metadata plus full plaintext key.

Keep setup endpoint behavior but change semantics:

- `POST /api/setup/api-key`: idempotently ensures an active key exists, creating one automatically with a default name when missing. It returns the full key and marks the step complete.
- `POST /api/setup/api-key/skip`: remove the frontend use and make the backend return `404` or `405`, because skipping is no longer valid.

API key extraction order:

1. `Authorization: Bearer <key>`
2. `X-API-Key: <key>`

Authentication result:

- Missing key on protected business routes returns `401`.
- Invalid key returns `401`.
- Valid key allows the route and updates `last_used_at`.

Admin browser routes keep using the existing admin session cookie for bootstrap and management endpoints. The settings page can retrieve the full key with an admin session, then the frontend API client stores it in memory and sends `X-API-Key` on business API requests. After a page reload, the frontend can fetch the key from the settings endpoint again once the admin session is valid.

## Route Protection

Unauthenticated public routes:

- `GET /api/health`
- `GET /api/ready`
- `GET /api/setup/status`
- `POST /api/setup/admin` while no admin exists
- `POST /api/auth/login`
- `POST /api/auth/logout`
- `GET /api/auth/me`

Setup and API key management routes require an admin session after the admin user exists, but do not require an API key because they create or reveal the key:

- `POST /api/setup/api-key`
- `POST /api/setup/telegram-api`
- `POST /api/setup/listen-rules`
- `POST /api/setup/complete`
- `GET /api/settings/api-key`
- `POST /api/settings/api-key/regenerate`

All post-setup business routes require a valid API key. An admin session alone is not enough for business API access.

## Frontend

First-run API key step:

- On mount, call `POST /api/setup/api-key`.
- Show the generated key in a copyable code block.
- Store the generated key in memory so subsequent setup business calls can include `X-API-Key` where required.
- Provide a single continue button.
- Remove the name input and skip button.

Settings page:

- Replace the current API key stub text with an API Key panel.
- Load the active API key metadata and full key.
- Store the loaded key in memory for the API client.
- Show prefix, full key, created time, and last used time.
- Provide a regenerate button with confirmation copy.
- After regeneration, show the new full key immediately and replace the in-memory key.

API client:

- Keep using cookies for admin session calls.
- Add `X-API-Key` to requests whenever the in-memory key is available.
- Routes that need a business API call immediately after login should first load the active API key through the settings endpoint.

## Data Model

Add a migration to include encrypted plaintext:

- `api_keys.key_ciphertext TEXT NOT NULL DEFAULT ''`

Existing databases may have historical rows without ciphertext. When no active key has ciphertext, the settings endpoint should regenerate a new key automatically or return a clear conflict. Regeneration is preferred because it restores the mandatory invariant without manual database work.

## Testing

Backend tests:

- Setup API key endpoint creates a key automatically and cannot be skipped.
- Protected business route returns `401` without key.
- Protected business route succeeds with `Authorization: Bearer <key>`.
- Protected business route succeeds with `X-API-Key`.
- Admin session alone does not allow protected business route access.
- Invalid key returns `401`.
- Regeneration invalidates old key and accepts new key.
- Settings endpoint returns full key for authenticated admin.

Frontend tests:

- Setup API key view auto-generates and displays the key.
- Setup API key view no longer renders skip or name controls.
- API client sends `X-API-Key` when the key is loaded in memory.
- Settings page loads and displays the full API key.
- Settings page regenerate action replaces the displayed full key.

## Out of Scope

- Multiple named API keys.
- Per-key scopes or permissions.
- API key deletion.
- External KMS or environment-provided encryption secret.
