# Telegram QR Login Design

## Goal

Support Telegram account login by scanning a QR code while keeping the existing phone verification-code login as a fallback.

## Scope

This change applies to the first-run Telegram login page and the same Telegram login API surface used for adding or reconnecting accounts. It does not remove or weaken the current phone, code, and two-step-password flow.

## Recommended Approach

Use a REST polling QR login flow.

The browser asks the backend to start a QR login session. The backend creates a temporary Telegram session file, exports a Telegram QR login token through gotd, and returns a `login_id`, `qr_url`, and `expires_at`. The browser renders the QR code from `qr_url` and polls status every two seconds. When the Telegram mobile app scans and confirms the QR code, the backend imports the accepted token, saves the account as `ONLINE`, moves the temporary session to the account session path, and returns the same successful login response shape used by the phone login flow.

## Alternatives Considered

Server-sent events would provide faster status updates, but the current login page and stores use request-response APIs. SSE adds lifecycle complexity without meaningful user benefit for this short-lived flow.

A single long request waiting for scan confirmation would reduce endpoint count, but it is harder to handle browser timeouts, QR expiration, cancellation, and token refresh. Polling keeps each request bounded and testable.

## Backend API

- `POST /api/telegram/login/qr/start`
  - Starts a QR login session.
  - Returns `login_id`, `qr_url`, `expires_at`, and `status: "pending"`.
  - Creates a temporary session file but does not create an account row.
- `GET /api/telegram/login/qr/:login_id`
  - Returns current QR login status.
  - When still pending, returns `status: "pending"` and the current `expires_at`.
  - When the token expires, returns a refreshed `qr_url` and `expires_at` when refresh succeeds.
  - When confirmed, returns `status: "online"`, `account`, and `metadata_sync`.
  - Returns `404` for unknown or already-cleaned sessions.
- `DELETE /api/telegram/login/qr/:login_id`
  - Cancels a pending QR login session and removes its temporary session file.

The existing endpoints remain unchanged:

- `POST /api/telegram/login/send-code`
- `POST /api/telegram/login/sign-in`
- `POST /api/telegram/login/password`

## Telegram Client

Extend `telegram.Client` with QR login capability backed by gotd `client.QR()`.

The implementation keeps a gotd client alive for each pending QR session because QR login needs an active session to export tokens and import the accepted token. It uses gotd QR token URLs directly so the frontend can render the QR code without the backend generating image bytes.

QR login success maps gotd authorization to the existing `telegram.Profile`. The profile must include the Telegram user ID, first name, last name, username, and phone when available. The phone field is required for the existing account uniqueness model, so the backend should fall back to a stable synthetic value such as `tg:<telegram_user_id>` only if Telegram does not expose a phone number in the authorization user.

## QR Session Store

Add an in-memory QR session store owned by the API dependencies.

Each session stores:

- `login_id`
- temporary session path
- current token URL
- expiration timestamp
- status
- cancellation function
- completed profile and account ID when available

Sessions are short-lived and are not persisted across process restarts. Restarting the service invalidates pending QR logins, which is acceptable because users can generate a new QR code.

The store removes sessions when they are canceled, completed, expired past a grace window, or after a status response has delivered the completed account.

## Account And Session Finalization

QR login does not create an account row before confirmation because the account phone and Telegram user ID are unknown.

After QR confirmation:

1. Save or update the account with `Status: ONLINE` and profile metadata.
2. Move the temporary session file to `session.Manager.PathForAccount(account.ID)`.
3. Update the saved account with the final session path if the account row was created before the move path was known.
4. Run the same post-login account profile and metadata sync logic used by phone login.
5. Return the standard login response with `account` and `metadata_sync`.

If account saving or session moving fails, remove the temporary session and return an error instead of leaving an orphaned online account.

## Frontend UI

`SetupTelegramLoginView` adds a segmented login mode control with two modes:

- QR login
- Verification code login

QR login is the default mode. The page shows a QR code, expiration state, refresh action, and cancel action. It starts polling after QR generation and stops polling when the user changes modes, cancels, leaves the page, or login succeeds.

The verification-code mode keeps the existing phone, code, and two-step-password controls. Successful login from either mode calls the same `finish()` flow and routes to `/setup/listen-rules`.

## Frontend Store And Types

Extend the Telegram store with QR actions:

- `startQRLogin()`
- `pollQRLogin(loginID)`
- `cancelQRLogin(loginID)`

Add response types for QR start and QR status. QR success can reuse `TelegramLoginResponse` fields by including `account` and `metadata_sync`.

## Error Handling

QR start returns an error when Telegram API credentials are unavailable or the Telegram client cannot export a token.

Polling returns pending status while the QR token is active. When token refresh fails, the response reports an error state and the frontend offers manual regeneration. Unknown `login_id` returns `404` and the frontend stops polling.

Cancellation is idempotent from the user's perspective. If the backend has already cleaned the session, the frontend treats it as canceled.

## Testing

Backend tests cover:

- QR start returns `login_id`, `qr_url`, and expiration without creating an account.
- QR poll returns pending status.
- QR poll finalizes a confirmed login, persists the account, moves the session, and returns metadata sync.
- QR cancellation removes temporary session state.
- Phone verification-code login endpoints still behave unchanged.

Frontend tests cover:

- The login page renders both QR and verification-code modes.
- QR mode starts a login and renders the QR payload.
- Polling stops on success and routes to listen rules.
- Switching to verification-code mode preserves the existing send-code and sign-in flow.

## Open Decisions Resolved

Both QR login and verification-code login are supported. QR login is additive and should not remove the current fallback path.
