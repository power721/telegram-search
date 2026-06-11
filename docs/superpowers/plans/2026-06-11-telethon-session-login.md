# Telethon Session Login Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Telethon StringSession login method to enable users to authenticate using existing Telethon session strings.

**Architecture:** Add new endpoint that converts Telethon StringSession to gotd session format using `session.TelethonSession()`, saves to file, connects to verify, and returns profile.

**Tech Stack:** Go, gotd/td v0.145.1, Gin

---

## File Structure

**New files:**
- `internal/telegram/telethon_login.go` - Telethon session conversion and login logic

**Modified files:**
- `internal/telegram/client.go` - Add interface method
- `internal/telegram/gotd_client.go` - Add NopClient stub
- `internal/api/handlers.go` - Add handler
- `internal/api/router.go` - Add route

---

### Task 1: Add Client Interface Method

**Files:**
- Modify: `internal/telegram/client.go:111-123`

- [ ] **Step 1: Add LoginWithTelethonSession to Client interface**

Add after line 115 (after `StartQRLogin`):

```go
LoginWithTelethonSession(ctx context.Context, sessionString string, sessionPath string) (Profile, error)
```

- [ ] **Step 2: Add NopClient stub**

In `internal/telegram/gotd_client.go`, add after `NopClient.StartQRLogin` (around line 186):

```go
func (NopClient) LoginWithTelethonSession(context.Context, string, string) (Profile, error) {
	return Profile{}, ErrUnavailable
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: Success (no errors)

- [ ] **Step 4: Commit**

```bash
git add internal/telegram/client.go internal/telegram/gotd_client.go
git commit -m "feat: add LoginWithTelethonSession interface method

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Implement Telethon Session Login

**Files:**
- Create: `internal/telegram/telethon_login.go`

- [ ] **Step 1: Create telethon_login.go with imports and function signature**

```go
package telegram

import (
	"context"
	"fmt"

	"github.com/gotd/td/session"
	gotdtelegram "github.com/gotd/td/telegram"
)

func (g *GotdClient) LoginWithTelethonSession(ctx context.Context, sessionString string, sessionPath string) (Profile, error) {
	return Profile, nil
}
```

- [ ] **Step 2: Add session conversion logic**

Replace function body:

```go
func (g *GotdClient) LoginWithTelethonSession(ctx context.Context, sessionString string, sessionPath string) (Profile, error) {
	data, err := session.TelethonSession(sessionString)
	if err != nil {
		return Profile{}, fmt.Errorf("invalid telethon session: %w", err)
	}

	loader := &session.Loader{Storage: &session.FileStorage{Path: sessionPath}}
	if err := loader.Save(ctx, data); err != nil {
		return Profile{}, fmt.Errorf("save session: %w", err)
	}

	return Profile{}, nil
}
```

- [ ] **Step 3: Add profile fetch logic**

Replace return statement with:

```go
	var profile Profile
	err = g.withClient(ctx, sessionPath, func(ctx context.Context, client *gotdtelegram.Client) error {
		user, err := client.Self(ctx)
		if err != nil {
			return err
		}
		profile = profileFromUser(user)
		return nil
	})
	if err != nil {
		return Profile{}, fmt.Errorf("fetch profile: %w", err)
	}
	return profile, nil
```

- [ ] **Step 4: Verify it compiles**

Run: `go build ./...`
Expected: Success

- [ ] **Step 5: Commit**

```bash
git add internal/telegram/telethon_login.go
git commit -m "feat: implement LoginWithTelethonSession

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Add API Handler

**Files:**
- Modify: `internal/api/handlers.go`

- [ ] **Step 1: Add telethonSessionLogin handler after password handler (after line 1147)**

```go
func (h handlers) telethonSessionLogin(c *gin.Context) {
	var req struct {
		SessionString string `json:"session_string"`
	}
	if !bindJSON(c, &req) {
		return
	}
	req.SessionString = strings.TrimSpace(req.SessionString)
	if req.SessionString == "" {
		errorText(c, http.StatusBadRequest, "session_string is required")
		return
	}

	sessionPath := ""
	if h.deps.Sessions != nil {
		sessionPath = h.deps.Sessions.PathForTemporary("telethon-" + fmt.Sprintf("%d", time.Now().UnixNano()))
		if err := os.MkdirAll(filepath.Dir(sessionPath), 0o700); err != nil {
			errorJSON(c, http.StatusInternalServerError, err)
			return
		}
	}

	profile, err := h.deps.Telegram.LoginWithTelethonSession(c.Request.Context(), req.SessionString, sessionPath)
	if err != nil {
		if h.deps.Sessions != nil && sessionPath != "" {
			_ = h.deps.Sessions.RemovePath(sessionPath)
		}
		errorJSON(c, http.StatusBadRequest, err)
		return
	}

	phone := profile.Phone
	if phone == "" {
		phone = fmt.Sprintf("tg:%d", profile.TelegramUserID)
	}

	accountID, err := h.deps.Accounts.Save(c.Request.Context(), model.Account{
		Phone:          phone,
		TelegramUserID: profile.TelegramUserID,
		Status:         model.AccountStatusLoginRequired,
	})
	if err != nil {
		if h.deps.Sessions != nil && sessionPath != "" {
			_ = h.deps.Sessions.RemovePath(sessionPath)
		}
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}

	if h.deps.Sessions != nil && sessionPath != "" {
		finalPath, err := h.deps.Sessions.MoveTemporaryToAccount(sessionPath, accountID)
		if err != nil {
			errorJSON(c, http.StatusInternalServerError, err)
			return
		}
		sessionPath = finalPath
	}

	account := model.Account{
		ID:             accountID,
		Phone:          phone,
		TelegramUserID: profile.TelegramUserID,
		FirstName:      profile.FirstName,
		LastName:       profile.LastName,
		Username:       profile.Username,
		Status:         model.AccountStatusOnline,
		SessionPath:    sessionPath,
	}
	now := time.Now().UTC()
	account.LastOnlineAt = &now

	if err := h.deps.Accounts.Update(c.Request.Context(), account); err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}

	h.respondWithOnlineAccount(c, account)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add internal/api/handlers.go
git commit -m "feat: add telethonSessionLogin handler

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Add API Route

**Files:**
- Modify: `internal/api/router.go`

- [ ] **Step 1: Add route after qr login route**

Find line with `telegramAPI.POST("/login/qr/start", h.startQRLogin)` and add after it:

```go
	telegramAPI.POST("/login/telethon-session", h.telethonSessionLogin)
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: Success

- [ ] **Step 3: Commit**

```bash
git add internal/api/router.go
git commit -m "feat: add telethon-session login route

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Manual Testing

**Files:**
- None (testing only)

- [ ] **Step 1: Start the application**

Run: `go run ./cmd/tg-search`
Expected: Server starts on configured port

- [ ] **Step 2: Test with invalid session string**

```bash
curl -X POST http://localhost:8080/api/telegram/login/telethon-session \
  -H "Content-Type: application/json" \
  -d '{"session_string": "invalid"}'
```

Expected: 400 Bad Request with error message

- [ ] **Step 3: Test with empty session string**

```bash
curl -X POST http://localhost:8080/api/telegram/login/telethon-session \
  -H "Content-Type: application/json" \
  -d '{"session_string": ""}'
```

Expected: 400 Bad Request "session_string is required"

- [ ] **Step 4: Test with valid session (if available)**

If you have a valid Telethon session string, test:

```bash
curl -X POST http://localhost:8080/api/telegram/login/telethon-session \
  -H "Content-Type: application/json" \
  -d '{"session_string": "YOUR_SESSION_STRING"}'
```

Expected: 200 OK with account profile

- [ ] **Step 5: Document completion**

Note: Manual testing complete. Ready for frontend integration.

---

## Self-Review Checklist

**Spec coverage:**
- ✓ Client interface method added
- ✓ GotdClient implementation using session.TelethonSession()
- ✓ API handler with validation
- ✓ Route added
- ✓ Phone number fallback (tg:{user_id})
- ✓ Error handling (empty, invalid, network)

**Placeholders:** None - all code is complete

**Type consistency:**
- Profile type used consistently
- sessionString and sessionPath parameter names consistent
- Account model fields match existing patterns
