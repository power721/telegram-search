# Telegram Account Logout And Delete Design

## Goal

Support Telegram account logout and deletion from the Accounts page.

## Behavior

Logout keeps the account and indexed data. It stops account runtime work, removes the local Telegram session file, and marks the account `LOGIN_REQUIRED` so the user can sign in again later.

Delete removes the account. It stops account runtime work, removes the local Telegram session file, and deletes the account row. Existing foreign keys cascade account-owned channels, messages, links, rules, and cursors.

## API

- `POST /api/accounts/:id/logout`
  - Returns the updated account.
  - `404` when the account does not exist.
  - Stops runtime before removing the session.
  - Updates status to `LOGIN_REQUIRED`.
- `DELETE /api/accounts/:id`
  - Existing endpoint.
  - Used by the UI for destructive account removal.

## UI

`AccountsView` adds an Actions column with Logout and Delete buttons. Delete uses a confirmation dialog. Both actions refresh the account list after success and surface request failures through the existing Telegram store error state.

## Testing

Backend API tests cover logout status update and session removal. Frontend store tests cover the new API actions. Accounts page tests cover rendering action buttons and calling logout/delete flows.
