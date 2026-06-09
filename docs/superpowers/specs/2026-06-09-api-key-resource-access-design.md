# API Key Resource Access Design

## Context

API keys currently authenticate most business API routes. That is broader than the intended external access model. API keys should only be credentials for resource listing/detail endpoints and future image/video proxy endpoints. All other API routes should require an authenticated administrator session.

## Requirements

- API keys can access only the existing resource list/detail endpoints:
  - `GET /api/resources`
  - `GET /api/resources/grouped`
  - `GET /api/resources/:id`
- API keys cannot access global search, message/link/file/channel search, remote search, status, tasks, logs, accounts, channels, watch rules, Telegram login/control, settings, or maintenance routes.
- Administrator sessions can access all existing authenticated routes, including resource endpoints.
- Public/bootstrap routes remain unchanged: health, readiness, setup status, setup steps, and auth endpoints stay reachable according to their existing behavior.
- Future image and video proxy endpoints should use the same resource-access guard as the resource endpoints.

## Recommended Approach

Split the current business API protection into explicit route groups:

- `adminOnly`: guarded by an administrator session cookie only.
- `resourceAccess`: guarded by either an administrator session or a valid API key.

This keeps the public API key surface small and easy to audit. It also gives future media proxy endpoints a clear place to attach without re-opening management routes to API keys.

## API Behavior

`resourceAccess` accepts credentials in this order:

1. Administrator session cookie.
2. `Authorization: Bearer <api-key>`.
3. `X-API-Key: <api-key>`.
4. `api_key` query parameter.

Missing or invalid credentials return `401`.

`adminOnly` accepts only a valid administrator session cookie. API keys are ignored for this group. Missing or invalid administrator credentials return `401`.

## Route Protection

Resource-access routes:

- `GET /api/resources`
- `GET /api/resources/grouped`
- `GET /api/resources/:id`

Admin-only routes:

- Runtime status and storage usage.
- Tasks and task events.
- Logs and log downloads.
- Telegram login and account management.
- Channel sync/control/analyze routes.
- Listen/watch rule management.
- Search and remote search routes.
- Messages, links, maintenance, backup, and settings routes.

Public/setup/auth routes keep their existing placement and handler-level checks.

## Frontend

No broad frontend redesign is required for this change. The existing client may continue to include `X-API-Key` when one is stored, because the backend will no longer treat API keys as administrator credentials.

Administrator pages continue to rely on the session cookie. External consumers can call the resource endpoints with the API key.

## Testing

Backend tests should verify:

- API key succeeds on all three resource endpoints.
- Administrator session succeeds on resource endpoints.
- Missing credentials fail on resource endpoints.
- Invalid API key fails on resource endpoints.
- API key fails on representative admin-only routes such as `/api/status`, `/api/tasks`, and `/api/search/global`.
- Administrator session still succeeds on representative admin-only routes.

Frontend tests are not required unless route behavior changes force client-side adjustments.

## Out of Scope

- Multiple API key scopes.
- Per-key permissions.
- API key UI changes beyond existing settings behavior.
- Implementing image or video proxy endpoints in this change.
