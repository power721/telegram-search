# Telegram QR Login Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Telegram QR-code login while keeping the existing phone verification-code login flow.

**Architecture:** The API owns short-lived QR login sessions and exposes start, poll, and cancel endpoints. The Telegram client abstraction starts a gotd-backed QR session that keeps a Telegram client running until the login is confirmed or canceled. The Vue login page defaults to QR mode and keeps the existing phone-code mode as a fallback.

**Tech Stack:** Go, Gin, gotd v0.145.1, SQLite repositories, Vue 3, Pinia, Naive UI, Vitest, `qrcode` for browser QR rendering.

---

## File Structure

- Modify `internal/telegram/client.go`: add QR login domain types, `Profile.Phone`, and `Client.StartQRLogin`.
- Create `internal/telegram/qr_login.go`: gotd QR login session implementation.
- Create `internal/telegram/qr_login_test.go`: focused tests for QR session state using small fakes where possible.
- Modify `internal/session/session.go`: add path helpers for temporary QR session files and moving session files.
- Create `internal/api/qr_login_store.go`: in-memory QR session registry and cleanup behavior.
- Modify `internal/api/router.go`: add QR login endpoints and dependency.
- Modify `internal/api/handlers.go`: add QR start, poll, cancel handlers and refactor shared online-login response logic.
- Modify `internal/api/handlers_test.go`: add API tests and extend `fakeTelegram`.
- Modify `cmd/tg-search/main.go`: initialize QR login store dependency.
- Modify `web/package.json` and `web/package-lock.json`: add `qrcode` and `@types/qrcode`.
- Modify `web/src/api/types.ts`: add QR login response types.
- Modify `web/src/stores/telegram.ts`: add QR login store state/actions.
- Modify `web/src/stores/telegram.test.ts`: cover QR API paths and success state.
- Modify `web/src/views/SetupTelegramLoginView.vue`: add mode switch, QR rendering, polling, and cleanup.
- Modify `web/src/views/SetupTelegramLoginView.test.ts`: cover both QR and code modes.

### Task 1: Backend QR API Contract

**Files:**
- Modify: `internal/api/handlers_test.go`
- Modify: `internal/telegram/client.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/handlers.go`

- [ ] **Step 1: Write failing API tests for QR start and pending poll**

Add tests near `TestTelegramLoginRoutes`:

```go
func TestTelegramQRLoginStartAndPendingPoll(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	fake := &fakeTelegram{
		qrTokenURL: "tg://login?token=test-token",
		qrExpires: time.Now().UTC().Add(time.Minute),
	}
	deps.Telegram = fake
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/telegram/login/qr/start", nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("start status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var started struct {
		LoginID   string    `json:"login_id"`
		QRURL     string    `json:"qr_url"`
		ExpiresAt time.Time `json:"expires_at"`
		Status    string    `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &started); err != nil {
		t.Fatalf("invalid start JSON: %v body=%s", err, w.Body.String())
	}
	if started.LoginID == "" || started.QRURL != "tg://login?token=test-token" || started.Status != "pending" {
		t.Fatalf("start body = %+v, want login id, token url, pending", started)
	}
	accounts, err := deps.Accounts.FindAll(ctx)
	if err != nil {
		t.Fatalf("find accounts: %v", err)
	}
	if len(accounts) != 0 {
		t.Fatalf("accounts len = %d, want 0 before QR confirmation", len(accounts))
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/telegram/login/qr/"+started.LoginID, nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("poll status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var polled struct {
		LoginID   string    `json:"login_id"`
		QRURL     string    `json:"qr_url"`
		ExpiresAt time.Time `json:"expires_at"`
		Status    string    `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &polled); err != nil {
		t.Fatalf("invalid poll JSON: %v body=%s", err, w.Body.String())
	}
	if polled.LoginID != started.LoginID || polled.Status != "pending" || polled.QRURL != started.QRURL {
		t.Fatalf("poll body = %+v, want same pending QR session", polled)
	}
}
```

- [ ] **Step 2: Run the failing test**

Run: `go test ./internal/api -run TestTelegramQRLoginStartAndPendingPoll -count=1`

Expected: FAIL because `/api/telegram/login/qr/start` is not registered and QR login types are missing.

- [ ] **Step 3: Add minimal QR domain types and route stubs**

In `internal/telegram/client.go`, add:

```go
const (
	QRLoginStatusPending = "pending"
	QRLoginStatusOnline  = "online"
)

type QRLoginToken struct {
	URL       string
	ExpiresAt time.Time
}

type QRLoginPollResult struct {
	Status  string
	Token   QRLoginToken
	Profile Profile
}

type QRLoginSession interface {
	Token() QRLoginToken
	Poll(context.Context) (QRLoginPollResult, error)
	Cancel(context.Context) error
}
```

Extend `Client`:

```go
	StartQRLogin(ctx context.Context, sessionPath string) (QRLoginSession, error)
```

In `internal/api/router.go`, register:

```go
telegramAPI.POST("/login/qr/start", h.startQRLogin)
telegramAPI.GET("/login/qr/:login_id", h.pollQRLogin)
telegramAPI.DELETE("/login/qr/:login_id", h.cancelQRLogin)
```

In `internal/api/handlers.go`, add temporary handlers that call `h.deps.Telegram.StartQRLogin`, store the session in a new dependency added in Task 2, and return pending JSON.

- [ ] **Step 4: Extend test fake minimally**

In `internal/api/handlers_test.go`, extend `fakeTelegram`:

```go
	qrTokenURL string
	qrExpires  time.Time
	qrSession  *fakeQRLoginSession
```

Add:

```go
func (f *fakeTelegram) StartQRLogin(ctx context.Context, sessionPath string) (telegram.QRLoginSession, error) {
	session := &fakeQRLoginSession{
		token: telegram.QRLoginToken{URL: f.qrTokenURL, ExpiresAt: f.qrExpires},
	}
	f.qrSession = session
	return session, nil
}

type fakeQRLoginSession struct {
	token     telegram.QRLoginToken
	result    telegram.QRLoginPollResult
	cancelled bool
}

func (s *fakeQRLoginSession) Token() telegram.QRLoginToken {
	return s.token
}

func (s *fakeQRLoginSession) Poll(ctx context.Context) (telegram.QRLoginPollResult, error) {
	if s.result.Status == "" {
		return telegram.QRLoginPollResult{Status: telegram.QRLoginStatusPending, Token: s.token}, nil
	}
	return s.result, nil
}

func (s *fakeQRLoginSession) Cancel(ctx context.Context) error {
	s.cancelled = true
	return nil
}
```

- [ ] **Step 5: Run the test until green**

Run: `go test ./internal/api -run TestTelegramQRLoginStartAndPendingPoll -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/handlers.go internal/api/handlers_test.go internal/api/router.go internal/telegram/client.go
git commit -m "feat: add qr login api contract"
```

### Task 2: QR Session Store And Cleanup

**Files:**
- Create: `internal/api/qr_login_store.go`
- Create: `internal/api/qr_login_store_test.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/handlers.go`

- [ ] **Step 1: Write failing store tests**

Create `internal/api/qr_login_store_test.go`:

```go
package api

import (
	"context"
	"testing"
	"time"

	"tg-search/internal/telegram"
)

func TestQRLoginStoreAddFindCancel(t *testing.T) {
	store := NewQRLoginStore(time.Minute)
	session := &storeTestQRSession{
		token: telegram.QRLoginToken{URL: "tg://login?token=one", ExpiresAt: time.Now().UTC().Add(time.Minute)},
	}

	item := store.Add("/tmp/qr.session.json", session)
	if item.LoginID == "" {
		t.Fatal("login id is empty")
	}
	found, ok := store.Find(item.LoginID)
	if !ok || found.Session != session {
		t.Fatalf("Find returned %+v ok=%v, want stored session", found, ok)
	}
	if err := store.Cancel(context.Background(), item.LoginID); err != nil {
		t.Fatalf("Cancel returned error: %v", err)
	}
	if !session.cancelled {
		t.Fatal("session was not canceled")
	}
	if _, ok := store.Find(item.LoginID); ok {
		t.Fatal("Find returned canceled session")
	}
}

func TestQRLoginStoreExpiresOldSessions(t *testing.T) {
	store := NewQRLoginStore(time.Millisecond)
	session := &storeTestQRSession{
		token: telegram.QRLoginToken{URL: "tg://login?token=old", ExpiresAt: time.Now().UTC().Add(time.Minute)},
	}
	item := store.Add("/tmp/old.session.json", session)
	time.Sleep(2 * time.Millisecond)

	if _, ok := store.Find(item.LoginID); ok {
		t.Fatal("Find returned expired session")
	}
	if !session.cancelled {
		t.Fatal("expired session was not canceled")
	}
}

type storeTestQRSession struct {
	token     telegram.QRLoginToken
	cancelled bool
}

func (s *storeTestQRSession) Token() telegram.QRLoginToken {
	return s.token
}

func (s *storeTestQRSession) Poll(context.Context) (telegram.QRLoginPollResult, error) {
	return telegram.QRLoginPollResult{Status: telegram.QRLoginStatusPending, Token: s.token}, nil
}

func (s *storeTestQRSession) Cancel(context.Context) error {
	s.cancelled = true
	return nil
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/api -run TestQRLoginStore -count=1`

Expected: FAIL because `NewQRLoginStore` is missing.

- [ ] **Step 3: Implement QR store**

Create `internal/api/qr_login_store.go` with:

```go
package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"tg-search/internal/telegram"
)

type QRLoginStore struct {
	mu  sync.Mutex
	ttl time.Duration
	now func() time.Time
	byID map[string]QRLoginStoreItem
}

type QRLoginStoreItem struct {
	LoginID     string
	SessionPath string
	Session     telegram.QRLoginSession
	CreatedAt   time.Time
	Delivered   bool
}

func NewQRLoginStore(ttl time.Duration) *QRLoginStore {
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	return &QRLoginStore{ttl: ttl, now: time.Now, byID: map[string]QRLoginStoreItem{}}
}

func (s *QRLoginStore) Add(sessionPath string, session telegram.QRLoginSession) QRLoginStoreItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := QRLoginStoreItem{LoginID: newQRLoginID(), SessionPath: sessionPath, Session: session, CreatedAt: s.now().UTC()}
	s.byID[item.LoginID] = item
	return item
}

func (s *QRLoginStore) Find(loginID string) (QRLoginStoreItem, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.byID[loginID]
	if !ok {
		return QRLoginStoreItem{}, false
	}
	if s.now().Sub(item.CreatedAt) > s.ttl {
		delete(s.byID, loginID)
		_ = item.Session.Cancel(context.Background())
		return QRLoginStoreItem{}, false
	}
	return item, true
}

func (s *QRLoginStore) Remove(loginID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.byID, loginID)
}

func (s *QRLoginStore) Cancel(ctx context.Context, loginID string) error {
	s.mu.Lock()
	item, ok := s.byID[loginID]
	if ok {
		delete(s.byID, loginID)
	}
	s.mu.Unlock()
	if !ok {
		return nil
	}
	return item.Session.Cancel(ctx)
}

func newQRLoginID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(buf[:])
}
```

Add `QRLogins *QRLoginStore` to `Dependencies`, and default it in `NewRouter` when nil.

- [ ] **Step 4: Run store and API tests**

Run:

```bash
go test ./internal/api -run 'TestQRLoginStore|TestTelegramQRLoginStartAndPendingPoll' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/qr_login_store.go internal/api/qr_login_store_test.go internal/api/router.go internal/api/handlers.go
git commit -m "feat: store pending qr login sessions"
```

### Task 3: QR Finalization And Session Files

**Files:**
- Modify: `internal/session/session.go`
- Modify: `internal/session/session_test.go`
- Modify: `internal/api/handlers.go`
- Modify: `internal/api/handlers_test.go`
- Modify: `internal/telegram/client.go`

- [ ] **Step 1: Write failing session-manager test**

Add to `internal/session/session_test.go`:

```go
func TestManagerMovesTemporaryQRSessionToAccount(t *testing.T) {
	dir := t.TempDir()
	manager := NewManager(dir)
	temp := manager.PathForTemporary("qr-login-abc")
	if err := os.WriteFile(temp, []byte(`{"auth":"temp"}`), 0600); err != nil {
		t.Fatalf("write temp session: %v", err)
	}

	final, err := manager.MoveTemporaryToAccount(temp, 7)
	if err != nil {
		t.Fatalf("MoveTemporaryToAccount returned error: %v", err)
	}
	if final != manager.PathForAccount(7) {
		t.Fatalf("final path = %q, want account path", final)
	}
	if _, err := os.Stat(temp); !os.IsNotExist(err) {
		t.Fatalf("temp stat err = %v, want not exist", err)
	}
	data, err := os.ReadFile(final)
	if err != nil {
		t.Fatalf("read final session: %v", err)
	}
	if string(data) != `{"auth":"temp"}` {
		t.Fatalf("final data = %q", data)
	}
}
```

- [ ] **Step 2: Run session test to verify failure**

Run: `go test ./internal/session -run TestManagerMovesTemporaryQRSessionToAccount -count=1`

Expected: FAIL because temporary path and move methods are missing.

- [ ] **Step 3: Implement session helpers**

In `internal/session/session.go`, add:

```go
func (m *Manager) PathForTemporary(name string) string {
	return filepath.Join(m.dir, name+".session.json")
}

func (m *Manager) MoveTemporaryToAccount(tempPath string, accountID int64) (string, error) {
	finalPath := m.PathForAccount(accountID)
	if err := os.Rename(tempPath, finalPath); err != nil {
		return "", fmt.Errorf("move temporary session to account %d: %w", accountID, err)
	}
	return finalPath, nil
}

func (m *Manager) RemovePath(path string) error {
	err := os.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return fmt.Errorf("remove session path %q: %w", path, err)
}
```

- [ ] **Step 4: Write failing QR finalization API test**

Add near QR API tests:

```go
func TestTelegramQRLoginPollFinalizesConfirmedAccount(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	fake := &fakeTelegram{
		qrTokenURL: "tg://login?token=ready",
		qrExpires: time.Now().UTC().Add(time.Minute),
	}
	deps.Telegram = fake
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/telegram/login/qr/start", nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	var started struct{ LoginID string `json:"login_id"` }
	if err := json.Unmarshal(w.Body.Bytes(), &started); err != nil {
		t.Fatalf("invalid start JSON: %v", err)
	}
	if err := os.WriteFile(fake.qrSessionPath, []byte(`{"session":"qr"}`), 0600); err != nil {
		t.Fatalf("write qr session: %v", err)
	}
	fake.qrSession.result = telegram.QRLoginPollResult{
		Status: telegram.QRLoginStatusOnline,
		Profile: telegram.Profile{TelegramUserID: 99, Phone: "+19990000000", FirstName: "QR", LastName: "User", Username: "qr_user"},
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/telegram/login/qr/"+started.LoginID, nil)
	withAPIKey(t, deps, req)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("poll status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var body struct {
		Status  string        `json:"status"`
		Account model.Account `json:"account"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid poll JSON: %v body=%s", err, w.Body.String())
	}
	if body.Status != model.AccountStatusOnline || body.Account.Phone != "+19990000000" || body.Account.TelegramUserID != 99 {
		t.Fatalf("body = %+v, want online QR account", body)
	}
	if _, err := os.Stat(body.Account.SessionPath); err != nil {
		t.Fatalf("final session stat: %v", err)
	}
	if _, ok := deps.QRLogins.Find(started.LoginID); ok {
		t.Fatal("completed QR login session was not removed")
	}
	account, err := deps.Accounts.FindByPhone(ctx, "+19990000000")
	if err != nil {
		t.Fatalf("find account: %v", err)
	}
	if account.Status != model.AccountStatusOnline {
		t.Fatalf("stored status = %q, want ONLINE", account.Status)
	}
}
```

Extend `fakeTelegram.StartQRLogin` to record `qrSessionPath`.

- [ ] **Step 5: Run finalization test to verify failure**

Run: `go test ./internal/api -run TestTelegramQRLoginPollFinalizesConfirmedAccount -count=1`

Expected: FAIL because poll does not finalize confirmed sessions.

- [ ] **Step 6: Implement QR finalization and shared online response**

In `internal/api/handlers.go`, extract the JSON and metadata-sync part of `updateAccountProfile` into:

```go
func (h handlers) respondWithOnlineAccount(c *gin.Context, account model.Account) {
	metadataSync := gin.H{"status": "skipped", "channel_count": 0}
	// Move the existing ChannelSync and SyncQueue logic here unchanged.
	c.JSON(http.StatusOK, gin.H{
		"status":        model.AccountStatusOnline,
		"account":       account,
		"metadata_sync": metadataSync,
	})
}
```

Make `updateAccountProfile` set profile fields, update the row, then call `respondWithOnlineAccount`.

Implement `pollQRLogin`:

```go
item, ok := h.deps.QRLogins.Find(loginID)
if !ok {
	errorText(c, http.StatusNotFound, "qr login session not found")
	return
}
result, err := item.Session.Poll(c.Request.Context())
if err != nil {
	errorJSON(c, http.StatusInternalServerError, err)
	return
}
if result.Status != telegram.QRLoginStatusOnline {
	token := result.Token
	if token.URL == "" {
		token = item.Session.Token()
	}
	c.JSON(http.StatusOK, gin.H{"login_id": loginID, "status": telegram.QRLoginStatusPending, "qr_url": token.URL, "expires_at": token.ExpiresAt})
	return
}
account, err := h.saveQRProfile(c.Request.Context(), result.Profile, item.SessionPath)
if err != nil {
	_ = h.deps.Sessions.RemovePath(item.SessionPath)
	errorJSON(c, http.StatusInternalServerError, err)
	return
}
h.deps.QRLogins.Remove(loginID)
h.respondWithOnlineAccount(c, account)
```

Add `saveQRProfile` that uses `profile.Phone` or `tg:<telegram_user_id>`, saves the account, moves the temp session, updates `SessionPath`, and returns the stored account.

- [ ] **Step 7: Run targeted backend tests**

Run:

```bash
go test ./internal/session -run TestManagerMovesTemporaryQRSessionToAccount -count=1
go test ./internal/api -run 'TestTelegramQRLogin|TestTelegramLoginRoutes|TestTelegramSignInStartsMetadataSyncOnly' -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/session/session.go internal/session/session_test.go internal/api/handlers.go internal/api/handlers_test.go internal/telegram/client.go
git commit -m "feat: finalize qr login accounts"
```

### Task 4: Gotd QR Login Implementation

**Files:**
- Create: `internal/telegram/qr_login.go`
- Create: `internal/telegram/qr_login_test.go`
- Modify: `internal/telegram/gotd_client.go`
- Modify: `internal/telegram/client.go`

- [ ] **Step 1: Write focused tests for profile phone fallback**

Add to `internal/telegram/gotd_client_test.go`:

```go
func TestProfileFromUserIncludesPhone(t *testing.T) {
	profile := profileFromUser(&tg.User{
		ID:        42,
		FirstName: "Ada",
		LastName:  "Lovelace",
		Username:  "ada",
		Phone:     "15550000000",
	})

	if profile.TelegramUserID != 42 || profile.Phone != "+15550000000" || profile.Username != "ada" {
		t.Fatalf("profile = %+v, want id, normalized phone, username", profile)
	}
}
```

- [ ] **Step 2: Run phone-profile test to verify failure**

Run: `go test ./internal/telegram -run TestProfileFromUserIncludesPhone -count=1`

Expected: FAIL because `Profile.Phone` is missing.

- [ ] **Step 3: Add phone to profile mapping**

In `internal/telegram/client.go`, add `Phone string` to `Profile`.

In `profileFromUser`, normalize phone:

```go
phone, _ := user.GetPhone()
if phone != "" && phone[0] != '+' {
	phone = "+" + phone
}
```

Return `Phone: phone`.

- [ ] **Step 4: Add gotd QR session implementation**

Create `internal/telegram/qr_login.go` implementing `GotdClient.StartQRLogin`.

Key structure:

```go
type gotdQRLoginSession struct {
	mu       sync.Mutex
	cancel   context.CancelFunc
	done     chan error
	qr       qrlogin.QR
	loggedIn qrlogin.LoggedIn
	token    QRLoginToken
	profile  Profile
	complete bool
}
```

Use `tg.NewUpdateDispatcher()`, `qrlogin.OnLoginToken(dispatcher)`, and `gotdtelegram.NewClient` with `UpdateHandler: dispatcher` and `SessionStorage: &session.FileStorage{Path: sessionPath}`. Start `client.Run` in a goroutine, export the first token inside `Run`, and return only after the token is available or startup fails.

`Poll` checks the `loggedIn` channel non-blockingly. When signaled, call `q.Import(ctx)`, convert the returned authorization to `Profile`, mark complete, cancel the run context, and return `Status: QRLoginStatusOnline`.

If the token has expired and login is not complete, call `q.Export(ctx)` to refresh token URL and expiration, then return pending.

`Cancel` calls the stored cancel function and waits briefly for the run goroutine to exit.

- [ ] **Step 5: Run telegram package tests**

Run: `go test ./internal/telegram -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/telegram/client.go internal/telegram/gotd_client.go internal/telegram/gotd_client_test.go internal/telegram/qr_login.go
git commit -m "feat: implement gotd qr login session"
```

### Task 5: Frontend Store And Types

**Files:**
- Modify: `web/package.json`
- Modify: `web/package-lock.json`
- Modify: `web/src/api/types.ts`
- Modify: `web/src/stores/telegram.ts`
- Modify: `web/src/stores/telegram.test.ts`

- [ ] **Step 1: Install QR rendering dependency**

Run: `npm install --prefix web qrcode @types/qrcode`

Expected: `web/package.json` and `web/package-lock.json` include both dependencies.

- [ ] **Step 2: Write failing store test for QR paths**

Add to `web/src/stores/telegram.test.ts`:

```ts
it('uses qr login API paths and loads accounts after qr success', async () => {
  vi.mocked(apiPost).mockImplementationOnce((path: string) => {
    expect(path).toBe('/api/telegram/login/qr/start')
    return Promise.resolve({
      login_id: 'login-1',
      status: 'pending',
      qr_url: 'tg://login?token=one',
      expires_at: '2026-06-08T12:00:00Z'
    })
  })
  vi.mocked(apiGet).mockImplementationOnce((path: string) => {
    expect(path).toBe('/api/telegram/login/qr/login-1')
    return Promise.resolve({
      login_id: 'login-1',
      status: 'online',
      account: { id: 1, phone: '+10000000000', status: 'ONLINE', last_error: '' },
      metadata_sync: { status: 'succeeded', channel_count: 0 }
    })
  }).mockImplementationOnce((path: string) => {
    expect(path).toBe('/api/accounts')
    return Promise.resolve({ items: [] })
  })

  const store = useTelegramStore()
  const started = await store.startQRLogin()
  const polled = await store.pollQRLogin(started.login_id)

  expect(started.qr_url).toContain('tg://login')
  expect(polled.account?.status).toBe('ONLINE')
  expect(apiPost).toHaveBeenCalledWith('/api/telegram/login/qr/start', {})
})
```

- [ ] **Step 3: Run test to verify failure**

Run: `npm --prefix web run test -- telegram.test.ts`

Expected: FAIL because QR store actions and types are missing.

- [ ] **Step 4: Add QR types and store actions**

In `web/src/api/types.ts`, add:

```ts
export interface TelegramQRLoginStartResponse {
  login_id: string
  status: 'pending'
  qr_url: string
  expires_at: string
}

export interface TelegramQRLoginStatusResponse extends TelegramLoginResponse {
  login_id: string
  status: 'pending' | 'online'
  qr_url?: string
  expires_at?: string
}
```

In `telegram.ts`, add state:

```ts
qrLogin: null as TelegramQRLoginStatusResponse | TelegramQRLoginStartResponse | null
```

Add actions:

```ts
async startQRLogin() {
  return this.withLoading(async () => {
    this.passwordRequired = false
    this.qrLogin = await apiPost<TelegramQRLoginStartResponse>('/api/telegram/login/qr/start', {})
    return this.qrLogin
  })
},
async pollQRLogin(loginID: string) {
  const response = await apiGet<TelegramQRLoginStatusResponse>(`/api/telegram/login/qr/${loginID}`)
  this.qrLogin = response
  this.loginResult = response
  if (response.account) {
    await this.loadAccounts()
  }
  return response
},
async cancelQRLogin(loginID: string) {
  await apiDelete<{ canceled: boolean }>(`/api/telegram/login/qr/${loginID}`)
  this.qrLogin = null
}
```

- [ ] **Step 5: Run store tests**

Run: `npm --prefix web run test -- telegram.test.ts`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add web/package.json web/package-lock.json web/src/api/types.ts web/src/stores/telegram.ts web/src/stores/telegram.test.ts
git commit -m "feat: add qr login store actions"
```

### Task 6: Frontend Login Page QR Mode

**Files:**
- Modify: `web/src/views/SetupTelegramLoginView.vue`
- Modify: `web/src/views/SetupTelegramLoginView.test.ts`

- [ ] **Step 1: Write failing view tests**

Add tests:

```ts
it('renders qr login mode by default and can switch to code login', async () => {
  const wrapper = mount(SetupTelegramLoginView, {
    global: {
      stubs: {
        'n-form': { template: '<form><slot /></form>' },
        'n-form-item': { props: ['label'], template: '<label>{{ label }}<slot /></label>' },
        'n-input': true,
        'n-button': { emits: ['click'], template: `<button @click="$emit('click')"><slot /></button>` },
        'n-tabs': { emits: ['update:value'], template: '<div><button @click="$emit(`update:value`, `code`)">验证码登录</button><slot /></div>' },
        'n-tab-pane': { template: '<section><slot /></section>' }
      }
    }
  })

  expect(wrapper.text()).toContain('扫码登录')
  await wrapper.find('button').trigger('click')
  expect(wrapper.text()).toContain('手机号')
  expect(wrapper.text()).toContain('验证码')
})

it('starts qr login and finishes after poll succeeds', async () => {
  vi.mocked(apiPost).mockImplementation((path: string) => {
    if (path === '/api/telegram/login/qr/start') {
      return Promise.resolve({
        login_id: 'login-1',
        status: 'pending',
        qr_url: 'tg://login?token=one',
        expires_at: new Date(Date.now() + 60_000).toISOString()
      })
    }
    return Promise.resolve({ status: 'LOGIN_REQUIRED' })
  })
  vi.mocked(apiGet).mockImplementation((path: string) => {
    if (path === '/api/telegram/login/qr/login-1') {
      return Promise.resolve({
        login_id: 'login-1',
        status: 'online',
        account: { id: 1, phone: '+10000000000', status: 'ONLINE', last_error: '' },
        metadata_sync: { status: 'succeeded', channel_count: 0 }
      })
    }
    if (path === '/api/accounts') return Promise.resolve({ items: [] })
    if (path === '/api/setup/status') return Promise.resolve({
      complete: false,
      admin_configured: true,
      api_key_configured: false,
      api_key_step_complete: true,
      telegram_configured: true,
      telegram_login_complete: true,
      listen_rules_configured: false,
      current_step: 'listen_rules'
    })
    return Promise.resolve({})
  })

  const wrapper = mount(SetupTelegramLoginView, {
    global: {
      stubs: {
        'n-form': { template: '<form><slot /></form>' },
        'n-form-item': { props: ['label'], template: '<label>{{ label }}<slot /></label>' },
        'n-input': true,
        'n-button': { emits: ['click'], template: `<button @click="$emit('click')"><slot /></button>` },
        'n-tabs': { template: '<div><slot /></div>' },
        'n-tab-pane': { template: '<section><slot /></section>' }
      }
    }
  })

  await wrapper.findAll('button')[0].trigger('click')
  await flushPromises()

  expect(apiPost).toHaveBeenCalledWith('/api/telegram/login/qr/start', {})
  expect(push).toHaveBeenCalledWith('/setup/listen-rules')
})
```

- [ ] **Step 2: Run view test to verify failure**

Run: `npm --prefix web run test -- SetupTelegramLoginView.test.ts`

Expected: FAIL because QR mode UI is missing.

- [ ] **Step 3: Implement QR mode UI**

In `SetupTelegramLoginView.vue`:

- Import `onBeforeUnmount`, `nextTick`, and `QRCode` from `qrcode`.
- Add `loginMode = ref<'qr' | 'code'>('qr')`.
- Add `qrCanvas = ref<HTMLCanvasElement | null>(null)`, `qrPolling: number | undefined`, and `qrLoginID = ref('')`.
- Add `startQRLogin`, `renderQR`, `pollQRLogin`, `stopQRPolling`, and `cancelQRLogin`.
- Use `n-tabs` with panes for QR and code modes.
- Render `<canvas ref="qrCanvas" class="qr-canvas" />` and buttons for generating and canceling QR login.
- Keep the existing phone-code form unchanged inside the code pane.
- Call `stopQRPolling()` in `onBeforeUnmount`.

The polling function should call `telegram.pollQRLogin(qrLoginID.value)`, finish when `response.account` is present, and otherwise schedule another poll with `window.setTimeout(pollQRLogin, 2000)`.

- [ ] **Step 4: Run view tests**

Run: `npm --prefix web run test -- SetupTelegramLoginView.test.ts`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/views/SetupTelegramLoginView.vue web/src/views/SetupTelegramLoginView.test.ts
git commit -m "feat: add telegram qr login UI"
```

### Task 7: Full Verification

**Files:**
- Verify all changed files.

- [ ] **Step 1: Run backend tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 2: Run frontend tests**

Run: `npm --prefix web run test`

Expected: PASS.

- [ ] **Step 3: Run frontend typecheck**

Run: `npm --prefix web run typecheck`

Expected: PASS.

- [ ] **Step 4: Inspect final status**

Run: `git status --short`

Expected: clean working tree.

- [ ] **Step 5: Commit any final fixes**

If verification required fixes, commit only those fixed files:

```bash
git add <fixed-files>
git commit -m "test: verify telegram qr login"
```

If no fixes were needed, do not create an empty commit.

## Self-Review

Spec coverage:

- QR and code login are both supported by Tasks 1, 5, and 6.
- REST start, poll, and cancel endpoints are covered by Tasks 1, 2, and 3.
- Temporary QR session storage and cleanup are covered by Task 2.
- Account/session finalization and metadata sync reuse are covered by Task 3.
- gotd `client.QR()` implementation is covered by Task 4.
- Frontend QR rendering, polling, cleanup, and fallback code login are covered by Tasks 5 and 6.
- Full verification is covered by Task 7.

Placeholder scan:

- No placeholder task remains.
- Every task has exact files, exact commands, and expected results.

Type consistency:

- Backend status strings are `pending` and `online`.
- API response fields are `login_id`, `qr_url`, `expires_at`, `status`, `account`, and `metadata_sync`.
- Frontend uses the same snake_case API fields already used by existing types.
