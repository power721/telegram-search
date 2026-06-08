# Telegram Account Logout And Delete Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Telegram account logout and deletion controls to the Accounts page.

**Architecture:** Add a focused account logout API that composes existing account runtime, session manager, and repository behavior. Extend the Pinia Telegram store with `logoutAccount` and `deleteAccount`, then wire `AccountsView` actions to those store methods.

**Tech Stack:** Go, Gin, SQLite repositories, Vue 3, Pinia, Naive UI, Vitest.

---

### Task 1: Backend Logout API

**Files:**
- Modify: `internal/api/router.go`
- Modify: `internal/api/handlers.go`
- Test: `internal/api/handlers_test.go`

- [ ] **Step 1: Write the failing test**

Add an API test that creates an online account, writes its session file through `deps.Sessions.PathForAccount(accountID)`, calls `POST /api/accounts/{id}/logout`, and asserts a `200` response, `status: LOGIN_REQUIRED`, and no session file.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api -run TestLogoutAccount -count=1`

Expected: fail with `404` because the route does not exist.

- [ ] **Step 3: Implement minimal API**

Register `api.POST("/accounts/:id/logout", h.logoutAccount)`. The handler loads the account, calls `AccountRuntime.StopAccount` when configured, removes the local session via `Sessions.RemoveForAccount`, updates the account status to `LOGIN_REQUIRED`, reloads it, and returns JSON.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/api -run TestLogoutAccount -count=1`

Expected: pass.

### Task 2: Frontend Store Actions

**Files:**
- Modify: `web/src/stores/telegram.ts`
- Test: `web/src/stores/telegram.test.ts`

- [ ] **Step 1: Write failing store tests**

Add tests for `logoutAccount(1)` calling `POST /api/accounts/1/logout`, reloading accounts, and returning the updated account. Add a test for `deleteAccount(1)` calling `DELETE /api/accounts/1` and reloading accounts.

- [ ] **Step 2: Run tests to verify they fail**

Run: `npm test -- --run src/stores/telegram.test.ts`

Expected: fail because the store methods do not exist and `apiDelete` is not used.

- [ ] **Step 3: Implement store methods**

Import `apiDelete`. Add `logoutAccount(id)` and `deleteAccount(id)` actions that call the API and then `loadAccounts()`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `npm test -- --run src/stores/telegram.test.ts`

Expected: pass.

### Task 3: Accounts Page Actions

**Files:**
- Modify: `web/src/views/AccountsView.vue`
- Test: `web/src/views/AccountsView.test.ts`

- [ ] **Step 1: Write failing view tests**

Add tests that mount the view, assert Logout and Delete buttons are rendered, click Logout, and assert the store/API flow is called. Add a delete confirmation test that confirms the dialog and asserts the delete API call.

- [ ] **Step 2: Run tests to verify they fail**

Run: `npm test -- --run src/views/AccountsView.test.ts`

Expected: fail because action buttons are not rendered.

- [ ] **Step 3: Implement view actions**

Add an Actions column. Use Naive UI buttons for Logout and Delete. Use `useDialog()` for delete confirmation, and keep table layout stable with the empty-state colspan updated.

- [ ] **Step 4: Run tests to verify they pass**

Run: `npm test -- --run src/views/AccountsView.test.ts`

Expected: pass.

### Task 4: Final Verification

**Files:**
- No new files.

- [ ] **Step 1: Run backend verification**

Run: `go test ./internal/api ./internal/account ./internal/session`

Expected: pass.

- [ ] **Step 2: Run frontend verification**

Run: `npm test -- --run src/views/AccountsView.test.ts src/stores/telegram.test.ts`

Expected: pass.

- [ ] **Step 3: Inspect diff**

Run: `git diff --stat` and `git diff --check`

Expected: scoped changes, no whitespace errors.
