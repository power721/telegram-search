# API Key Settings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make API keys mandatory for business API access, auto-generate the first key during setup, and let admins view or regenerate the full key from settings.

**Architecture:** Add encrypted API key storage in the repository layer, a focused `internal/apikey` service for generation/encryption/verification, and Gin middleware that protects business routes. The frontend keeps the current admin cookie flow for login/setup/key management, stores the loaded key in memory, and sends `X-API-Key` on business API requests.

**Tech Stack:** Go, Gin, SQLite migrations, bcrypt, AES-GCM, Vue 3, Pinia, Naive UI, Vitest.

---

## File Structure

- Modify `internal/db/migrations.go`: add migration version 3 for `api_keys.key_ciphertext`.
- Modify `internal/model/model.go`: add encrypted key field and API key response model.
- Modify `internal/repository/api_key.go`: active-key queries, regeneration transaction, verification candidates, last-used updates.
- Modify `internal/repository/api_key_test.go`: repository tests for active key lifecycle.
- Create `internal/apikey/service.go`: generate keys, encrypt/decrypt stored plaintext, verify request keys, expose settings responses.
- Create `internal/apikey/service_test.go`: service tests for generation, viewing, regeneration, old-key invalidation.
- Modify `internal/api/router.go`: wire API key service and route protection middleware.
- Modify `internal/api/handlers.go`: setup auto-generation, settings API key endpoints, remove skip semantics.
- Modify `internal/api/handlers_test.go`: route protection and endpoint tests.
- Modify `web/src/api/types.ts`: API key response type with timestamps.
- Modify `web/src/api/client.ts`: in-memory API key setter/getter and `X-API-Key` header injection.
- Modify `web/src/api/client.test.ts`: header injection tests.
- Modify `web/src/stores/setup.ts`: remove skip flow, store generated key in API client.
- Create `web/src/stores/apiKey.ts`: load/regenerate settings key and update API client memory.
- Modify `web/src/views/SetupAPIKeyView.vue`: auto-generate on mount and remove manual controls.
- Modify `web/src/views/SetupAPIKeyView.test.ts`: auto-generation tests.
- Modify `web/src/views/SettingsView.vue`: API key panel with full key and regenerate action.
- Create `web/src/views/SettingsView.test.ts`: settings panel tests.
- Modify `docs/api.md`: document required API key headers and settings endpoints.

---

### Task 1: Repository And Migration

**Files:**
- Modify: `internal/db/migrations.go`
- Modify: `internal/model/model.go`
- Modify: `internal/repository/api_key.go`
- Test: `internal/repository/api_key_test.go`

- [ ] **Step 1: Write failing repository tests**

Add these tests to `internal/repository/api_key_test.go`:

```go
func TestAPIKeyRepositoryActiveLifecycle(t *testing.T) {
	ctx := context.Background()
	conn := testDB(t)
	repo := NewAPIKeyRepository(conn)

	id, err := repo.Create(ctx, model.APIKey{
		Name: "default", KeyHash: "hash1", KeyCiphertext: "cipher1", Prefix: "abcd1234", Enabled: true,
	})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	active, err := repo.Active(ctx)
	if err != nil {
		t.Fatalf("active key: %v", err)
	}
	if active.ID != id || active.KeyCiphertext != "cipher1" || !active.Enabled {
		t.Fatalf("active = %+v, want id %d with ciphertext", active, id)
	}

	if err := repo.DisableEnabled(ctx); err != nil {
		t.Fatalf("disable enabled: %v", err)
	}
	if _, err := repo.Active(ctx); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("active after disable err = %v, want sql.ErrNoRows", err)
	}
}

func TestAPIKeyRepositoryVerificationCandidatesAndLastUsed(t *testing.T) {
	ctx := context.Background()
	conn := testDB(t)
	repo := NewAPIKeyRepository(conn)
	now := time.Now().UTC()

	id, err := repo.Create(ctx, model.APIKey{
		Name: "default", KeyHash: "hash1", KeyCiphertext: "cipher1", Prefix: "abcd1234", Enabled: true,
	})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	candidates, err := repo.EnabledByPrefix(ctx, "abcd1234")
	if err != nil {
		t.Fatalf("enabled by prefix: %v", err)
	}
	if len(candidates) != 1 || candidates[0].ID != id {
		t.Fatalf("candidates = %+v, want created key", candidates)
	}
	if err := repo.UpdateLastUsed(ctx, id, now); err != nil {
		t.Fatalf("update last used: %v", err)
	}
	active, err := repo.Active(ctx)
	if err != nil {
		t.Fatalf("active key: %v", err)
	}
	if active.LastUsedAt == nil || !active.LastUsedAt.Equal(now) {
		t.Fatalf("last used = %v, want %v", active.LastUsedAt, now)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/repository -run 'TestAPIKeyRepository' -v
```

Expected: FAIL because `KeyCiphertext`, `Active`, `DisableEnabled`, `EnabledByPrefix`, and `UpdateLastUsed` do not exist.

- [ ] **Step 3: Implement migration, model, and repository methods**

Update `internal/model/model.go`:

```go
type APIKey struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	KeyHash       string     `json:"-"`
	KeyCiphertext string     `json:"-"`
	Prefix        string     `json:"prefix"`
	Enabled       bool       `json:"enabled"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type APIKeyResponse struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Key        string     `json:"key"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
```

Add migration version 3 in `internal/db/migrations.go` after version 2:

```go
{
	version: 3,
	name:    "api_key_ciphertext",
	sql: `
ALTER TABLE api_keys ADD COLUMN key_ciphertext TEXT NOT NULL DEFAULT '';
`,
},
```

Update `internal/repository/api_key.go` so `Create` inserts `key_ciphertext`, then add:

```go
func (r *APIKeyRepository) Active(ctx context.Context) (model.APIKey, error) {
	var key model.APIKey
	err := r.db.QueryRowContext(ctx, `
SELECT id, name, key_hash, key_ciphertext, prefix, enabled, last_used_at, created_at, updated_at
FROM api_keys
WHERE enabled = 1
ORDER BY id DESC
LIMIT 1`).Scan(&key.ID, &key.Name, &key.KeyHash, &key.KeyCiphertext, &key.Prefix, &key.Enabled, &key.LastUsedAt, &key.CreatedAt, &key.UpdatedAt)
	if err != nil {
		return model.APIKey{}, fmt.Errorf("active api key: %w", err)
	}
	return key, nil
}

func (r *APIKeyRepository) DisableEnabled(ctx context.Context) error {
	now := time.Now().UTC()
	if _, err := r.db.ExecContext(ctx, `UPDATE api_keys SET enabled = 0, updated_at = ? WHERE enabled = 1`, now); err != nil {
		return fmt.Errorf("disable enabled api keys: %w", err)
	}
	return nil
}

func (r *APIKeyRepository) EnabledByPrefix(ctx context.Context, prefix string) ([]model.APIKey, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, name, key_hash, key_ciphertext, prefix, enabled, last_used_at, created_at, updated_at
FROM api_keys
WHERE enabled = 1 AND prefix = ?
ORDER BY id DESC`, prefix)
	if err != nil {
		return nil, fmt.Errorf("enabled api keys by prefix: %w", err)
	}
	defer rows.Close()
	var keys []model.APIKey
	for rows.Next() {
		var key model.APIKey
		if err := rows.Scan(&key.ID, &key.Name, &key.KeyHash, &key.KeyCiphertext, &key.Prefix, &key.Enabled, &key.LastUsedAt, &key.CreatedAt, &key.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api keys: %w", err)
	}
	return keys, nil
}

func (r *APIKeyRepository) UpdateLastUsed(ctx context.Context, id int64, at time.Time) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE api_keys SET last_used_at = ?, updated_at = ? WHERE id = ?`, at, at, id); err != nil {
		return fmt.Errorf("update api key last used: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run repository tests**

Run:

```bash
go test ./internal/repository -run 'TestAPIKeyRepository' -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/db/migrations.go internal/model/model.go internal/repository/api_key.go internal/repository/api_key_test.go
git commit -m "feat: extend api key repository"
```

---

### Task 2: API Key Service

**Files:**
- Create: `internal/apikey/service.go`
- Test: `internal/apikey/service_test.go`

- [ ] **Step 1: Write failing service tests**

Create `internal/apikey/service_test.go`:

```go
package apikey

import (
	"context"
	"path/filepath"
	"testing"

	"tg-search/internal/db"
	"tg-search/internal/repository"
)

func TestServiceEnsureActiveCreatesViewableKey(t *testing.T) {
	ctx := context.Background()
	conn := testDB(t)
	service := NewService(repository.NewAPIKeyRepository(conn), repository.NewSettingsRepository(conn))

	resp, err := service.EnsureActive(ctx)
	if err != nil {
		t.Fatalf("ensure active: %v", err)
	}
	if resp.Key == "" || len(resp.Prefix) != 8 || resp.Prefix != resp.Key[:8] {
		t.Fatalf("response = %+v, want viewable key with prefix", resp)
	}

	again, err := service.EnsureActive(ctx)
	if err != nil {
		t.Fatalf("ensure active again: %v", err)
	}
	if again.Key != resp.Key || again.ID != resp.ID {
		t.Fatalf("ensure active again = %+v, want same key id %d", again, resp.ID)
	}
}

func TestServiceRegenerateInvalidatesOldKey(t *testing.T) {
	ctx := context.Background()
	conn := testDB(t)
	service := NewService(repository.NewAPIKeyRepository(conn), repository.NewSettingsRepository(conn))
	first, err := service.EnsureActive(ctx)
	if err != nil {
		t.Fatalf("ensure active: %v", err)
	}
	second, err := service.Regenerate(ctx)
	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if second.Key == first.Key || second.ID == first.ID {
		t.Fatalf("regenerated key = %+v, first = %+v", second, first)
	}
	if _, ok, err := service.Verify(ctx, first.Key); err != nil || ok {
		t.Fatalf("old verify id=%d ok=%v err=%v, want invalid", second.ID, ok, err)
	}
	if id, ok, err := service.Verify(ctx, second.Key); err != nil || !ok || id != second.ID {
		t.Fatalf("new verify id=%d ok=%v err=%v, want id %d", id, ok, err, second.ID)
	}
}

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "tg-search.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(context.Background(), conn); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return conn
}
```

Add `database/sql` to the test imports because `testDB` returns `*sql.DB`.

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/apikey -v
```

Expected: FAIL because package `internal/apikey` does not exist.

- [ ] **Step 3: Implement service**

Create `internal/apikey/service.go`:

```go
package apikey

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"golang.org/x/crypto/bcrypt"

	"tg-search/internal/model"
	"tg-search/internal/repository"
)

const settingsSecretKey = "api_key.encryption_secret"

type Service struct {
	keys     *repository.APIKeyRepository
	settings *repository.SettingsRepository
}

func NewService(keys *repository.APIKeyRepository, settings *repository.SettingsRepository) *Service {
	return &Service{keys: keys, settings: settings}
}

func (s *Service) EnsureActive(ctx context.Context) (model.APIKeyResponse, error) {
	active, err := s.keys.Active(ctx)
	if err == nil && active.KeyCiphertext != "" {
		return s.response(ctx, active)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return model.APIKeyResponse{}, err
	}
	return s.Regenerate(ctx)
}

func (s *Service) Regenerate(ctx context.Context) (model.APIKeyResponse, error) {
	plaintext, err := newPlaintext()
	if err != nil {
		return model.APIKeyResponse{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return model.APIKeyResponse{}, fmt.Errorf("hash api key: %w", err)
	}
	ciphertext, err := s.encrypt(ctx, plaintext)
	if err != nil {
		return model.APIKeyResponse{}, err
	}
	if err := s.keys.DisableEnabled(ctx); err != nil {
		return model.APIKeyResponse{}, err
	}
	id, err := s.keys.Create(ctx, model.APIKey{
		Name: "default", KeyHash: string(hash), KeyCiphertext: ciphertext, Prefix: plaintext[:8], Enabled: true,
	})
	if err != nil {
		return model.APIKeyResponse{}, err
	}
	active, err := s.keys.Active(ctx)
	if err != nil {
		return model.APIKeyResponse{}, err
	}
	active.ID = id
	return toResponse(active, plaintext), nil
}

func (s *Service) Active(ctx context.Context) (model.APIKeyResponse, error) {
	active, err := s.keys.Active(ctx)
	if err != nil {
		return model.APIKeyResponse{}, err
	}
	if active.KeyCiphertext == "" {
		return s.Regenerate(ctx)
	}
	return s.response(ctx, active)
}

func (s *Service) Verify(ctx context.Context, plaintext string) (int64, bool, error) {
	if len(plaintext) < 8 {
		return 0, false, nil
	}
	candidates, err := s.keys.EnabledByPrefix(ctx, plaintext[:8])
	if err != nil {
		return 0, false, err
	}
	for _, candidate := range candidates {
		if bcrypt.CompareHashAndPassword([]byte(candidate.KeyHash), []byte(plaintext)) == nil {
			if err := s.keys.UpdateLastUsed(ctx, candidate.ID, time.Now().UTC()); err != nil {
				return 0, false, err
			}
			return candidate.ID, true, nil
		}
	}
	return 0, false, nil
}

func (s *Service) response(ctx context.Context, key model.APIKey) (model.APIKeyResponse, error) {
	plaintext, err := s.decrypt(ctx, key.KeyCiphertext)
	if err != nil {
		return model.APIKeyResponse{}, err
	}
	return toResponse(key, plaintext), nil
}

func toResponse(key model.APIKey, plaintext string) model.APIKeyResponse {
	return model.APIKeyResponse{
		ID: key.ID, Name: key.Name, Prefix: key.Prefix, Key: plaintext,
		LastUsedAt: key.LastUsedAt, CreatedAt: key.CreatedAt, UpdatedAt: key.UpdatedAt,
	}
}

func newPlaintext() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate api key: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func (s *Service) encrypt(ctx context.Context, plaintext string) (string, error) {
	aead, err := s.aead(ctx)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate api key nonce: %w", err)
	}
	sealed := aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func (s *Service) decrypt(ctx context.Context, ciphertext string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode api key ciphertext: %w", err)
	}
	aead, err := s.aead(ctx)
	if err != nil {
		return "", err
	}
	if len(raw) < aead.NonceSize() {
		return "", fmt.Errorf("api key ciphertext is too short")
	}
	nonce := raw[:aead.NonceSize()]
	data := raw[aead.NonceSize():]
	opened, err := aead.Open(nil, nonce, data, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt api key: %w", err)
	}
	return string(opened), nil
}

func (s *Service) aead(ctx context.Context) (cipher.AEAD, error) {
	secret, ok, err := s.settings.Get(ctx, settingsSecretKey)
	if err != nil {
		return nil, err
	}
	if !ok {
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			return nil, fmt.Errorf("generate api key secret: %w", err)
		}
		secret = hex.EncodeToString(raw)
		if err := s.settings.Set(ctx, settingsSecretKey, secret); err != nil {
			return nil, err
		}
	}
	key, err := hex.DecodeString(secret)
	if err != nil {
		return nil, fmt.Errorf("decode api key secret: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create api key cipher: %w", err)
	}
	return cipher.NewGCM(block)
}
```

- [ ] **Step 4: Run service tests**

Run:

```bash
go test ./internal/apikey -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/apikey/service.go internal/apikey/service_test.go
git commit -m "feat: add api key service"
```

---

### Task 3: API Routes And Protection

**Files:**
- Modify: `internal/api/router.go`
- Modify: `internal/api/handlers.go`
- Test: `internal/api/handlers_test.go`

- [ ] **Step 1: Write failing API tests**

Add tests to `internal/api/handlers_test.go` near the existing API key setup tests:

```go
func TestSetupAPIKeyAutoGeneratesAndSkipIsDisabled(t *testing.T) {
	deps := testDeps(t)
	router := NewRouter(deps)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/setup/api-key", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("api key code = %d body=%s", w.Code, w.Body.String())
	}
	var body model.APIKeyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode api key: %v", err)
	}
	if body.Key == "" || body.Prefix != body.Key[:8] {
		t.Fatalf("api key response = %+v", body)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/setup/api-key/skip", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("skip code = %d body=%s, want 404 or 405", w.Code, w.Body.String())
	}
}

func TestBusinessAPIRequiresAPIKey(t *testing.T) {
	deps := testDeps(t)
	router := NewRouter(deps)

	key := createTestAPIKey(t, router)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status without key code = %d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status bearer code = %d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.Header.Set("X-API-Key", key)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status x-api-key code = %d body=%s", w.Code, w.Body.String())
	}
}

func TestAPIKeySettingsViewAndRegenerate(t *testing.T) {
	deps := testDeps(t)
	router := NewRouter(deps)
	cookie := createAdminSession(t, router)
	first := createTestAPIKey(t, router)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/settings/api-key", nil)
	req.AddCookie(cookie)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("settings api key code = %d body=%s", w.Code, w.Body.String())
	}
	var current model.APIKeyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &current); err != nil {
		t.Fatalf("decode current key: %v", err)
	}
	if current.Key != first {
		t.Fatalf("current key = %q, want first key", current.Key)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/settings/api-key/regenerate", nil)
	req.AddCookie(cookie)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("regenerate code = %d body=%s", w.Code, w.Body.String())
	}
	var regenerated model.APIKeyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &regenerated); err != nil {
		t.Fatalf("decode regenerated key: %v", err)
	}
	if regenerated.Key == first {
		t.Fatalf("regenerated key matched old key")
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.Header.Set("X-API-Key", first)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("old key code = %d body=%s, want 401", w.Code, w.Body.String())
	}
}
```

Add helpers:

```go
func createTestAPIKey(t *testing.T, router *gin.Engine) string {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/setup/api-key", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create api key code = %d body=%s", w.Code, w.Body.String())
	}
	var body model.APIKeyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode api key: %v", err)
	}
	return body.Key
}

func createAdminSession(t *testing.T, router *gin.Engine) *http.Cookie {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/setup/admin", bytes.NewBufferString(`{"username":"admin","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create admin code = %d body=%s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login code = %d body=%s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("login did not set session cookie")
	}
	return cookies[0]
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/api -run 'TestSetupAPIKeyAutoGeneratesAndSkipIsDisabled|TestBusinessAPIRequiresAPIKey|TestAPIKeySettingsViewAndRegenerate' -v
```

Expected: FAIL because the router still exposes skip, has no settings API key routes, and does not enforce API keys.

- [ ] **Step 3: Wire API key service into dependencies and handlers**

Modify `internal/api/router.go`:

```go
import "tg-search/internal/apikey"

type Dependencies struct {
	// existing fields
	APIKeyService *apikey.Service
}
```

At the start of `NewRouter`, after `h := handlers{deps: deps}`:

```go
if h.deps.APIKeyService == nil && h.deps.APIKeys != nil && h.deps.Settings != nil {
	h.deps.APIKeyService = apikey.NewService(h.deps.APIKeys, h.deps.Settings)
}
```

Modify setup routes:

```go
api.POST("/setup/api-key", h.setupAPIKey)
```

Remove:

```go
api.POST("/setup/api-key/skip", h.skipSetupAPIKey)
```

Add settings routes before protected business routes:

```go
api.GET("/settings/api-key", h.getAPIKeySettings)
api.POST("/settings/api-key/regenerate", h.regenerateAPIKey)
```

- [ ] **Step 4: Add middleware and handlers**

In `internal/api/router.go`, register business routes under a group:

```go
business := api.Group("")
business.Use(h.requireAPIKey())
business.GET("/storage/usage", h.storageUsage)
business.GET("/status", h.status)
business.GET("/tasks", h.tasks)
// move all post-setup business routes currently registered on api into business
```

Keep these on `api` without API key middleware:

```go
api.GET("/health", h.health)
api.GET("/ready", h.ready)
api.GET("/setup/status", h.setupStatus)
api.POST("/setup/admin", h.setupAdmin)
api.POST("/setup/api-key", h.setupAPIKey)
api.POST("/setup/telegram-api", h.saveSetupTelegramAPI)
api.POST("/setup/listen-rules", h.setupListenRules)
api.POST("/setup/complete", h.setupComplete)
api.POST("/auth/login", h.authLogin)
api.POST("/auth/logout", h.authLogout)
api.GET("/auth/me", h.authMe)
api.GET("/settings/telegram-api", h.getTelegramAPISettings)
api.PUT("/settings/telegram-api", h.updateTelegramAPISettings)
api.GET("/settings/api-key", h.getAPIKeySettings)
api.POST("/settings/api-key/regenerate", h.regenerateAPIKey)
```

Add to `internal/api/handlers.go`:

```go
func (h handlers) setupAPIKey(c *gin.Context) {
	resp, err := h.deps.APIKeyService.EnsureActive(c.Request.Context())
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	if err := h.deps.Settings.Set(c.Request.Context(), setupAPIKeyDoneKey, `true`); err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (h handlers) getAPIKeySettings(c *gin.Context) {
	if !h.hasAdminSession(c) {
		errorText(c, http.StatusUnauthorized, "not authenticated")
		return
	}
	resp, err := h.deps.APIKeyService.Active(c.Request.Context())
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h handlers) regenerateAPIKey(c *gin.Context) {
	if !h.hasAdminSession(c) {
		errorText(c, http.StatusUnauthorized, "not authenticated")
		return
	}
	resp, err := h.deps.APIKeyService.Regenerate(c.Request.Context())
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (h handlers) requireAPIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := apiKeyFromRequest(c.Request)
		if key == "" {
			errorText(c, http.StatusUnauthorized, "api key is required")
			c.Abort()
			return
		}
		if _, ok, err := h.deps.APIKeyService.Verify(c.Request.Context(), key); err != nil {
			errorJSON(c, http.StatusInternalServerError, err)
			c.Abort()
			return
		} else if !ok {
			errorText(c, http.StatusUnauthorized, "invalid api key")
			c.Abort()
			return
		}
		c.Next()
	}
}

func (h handlers) hasAdminSession(c *gin.Context) bool {
	cookie, err := c.Cookie(adminSessionCookie)
	if err != nil {
		return false
	}
	_, ok := h.deps.AdminAuth.UserForSession(cookie)
	return ok
}

func apiKeyFromRequest(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return strings.TrimSpace(r.Header.Get("X-API-Key"))
}
```

Delete `skipSetupAPIKey`.

- [ ] **Step 5: Run API tests**

Run:

```bash
go test ./internal/api -run 'TestSetupAPIKey|TestBusinessAPIRequiresAPIKey|TestAPIKeySettingsViewAndRegenerate' -v
```

Expected: PASS after updating any older setup API key tests to expect automatic generation and skip removal.

- [ ] **Step 6: Commit**

```bash
git add internal/api/router.go internal/api/handlers.go internal/api/handlers_test.go
git commit -m "feat: protect business api with api keys"
```

---

### Task 4: Frontend API Key State And Client

**Files:**
- Modify: `web/src/api/types.ts`
- Modify: `web/src/api/client.ts`
- Test: `web/src/api/client.test.ts`

- [ ] **Step 1: Write failing client tests**

Add to `web/src/api/client.test.ts`:

```ts
import { clearAPIKey, setAPIKey } from './client'

it('sends X-API-Key when an api key is loaded', async () => {
  globalThis.fetch = vi.fn().mockResolvedValue({
    ok: true,
    json: async () => ({ service: 'ok' })
  } as Response)

  setAPIKey('secret-key')
  await apiGet('/api/status')

  expect(globalThis.fetch).toHaveBeenCalledWith('/api/status', {
    credentials: 'include',
    headers: { Accept: 'application/json', 'X-API-Key': 'secret-key' }
  })
})

it('clears X-API-Key when requested', async () => {
  globalThis.fetch = vi.fn().mockResolvedValue({
    ok: true,
    json: async () => ({ service: 'ok' })
  } as Response)

  setAPIKey('secret-key')
  clearAPIKey()
  await apiGet('/api/status')

  expect(globalThis.fetch).toHaveBeenCalledWith('/api/status', {
    credentials: 'include',
    headers: { Accept: 'application/json' }
  })
})
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
npm --prefix web run test -- client.test.ts
```

Expected: FAIL because `setAPIKey` and `clearAPIKey` do not exist.

- [ ] **Step 3: Implement API key memory and types**

Add to `web/src/api/types.ts`:

```ts
export interface APIKeyResponse {
  id: number
  name: string
  prefix: string
  key: string
  last_used_at?: string
  created_at?: string
  updated_at?: string
}
```

Update `APIKeySetupResponse` to extend it:

```ts
export type APIKeySetupResponse = APIKeyResponse
```

Update `web/src/api/client.ts`:

```ts
let apiKey = ''

export function setAPIKey(key: string) {
  apiKey = key
}

export function clearAPIKey() {
  apiKey = ''
}

function jsonHeaders(contentType = false) {
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (contentType) {
    headers['Content-Type'] = 'application/json'
  }
  if (apiKey) {
    headers['X-API-Key'] = apiKey
  }
  return headers
}
```

Use `jsonHeaders()` for GET and DELETE, and `jsonHeaders(true)` for POST and PATCH.

- [ ] **Step 4: Run client tests**

Run:

```bash
npm --prefix web run test -- client.test.ts
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/api/types.ts web/src/api/client.ts web/src/api/client.test.ts
git commit -m "feat: store api key in web client"
```

---

### Task 5: Setup API Key View

**Files:**
- Modify: `web/src/stores/setup.ts`
- Modify: `web/src/views/SetupAPIKeyView.vue`
- Test: `web/src/views/SetupAPIKeyView.test.ts`
- Test: `web/src/stores/setup.test.ts`

- [ ] **Step 1: Write failing setup view test**

Replace `SetupAPIKeyView.test.ts` expectations with:

```ts
import { apiPost, setAPIKey } from '@/api/client'

vi.mock('@/api/client', () => ({
  apiGet: vi.fn().mockResolvedValue({ current_step: 'telegram_api' }),
  apiPost: vi.fn().mockResolvedValue({
    id: 1,
    name: 'default',
    prefix: '12345678',
    key: '12345678123456781234567812345678'
  }),
  setAPIKey: vi.fn()
}))

it('auto-generates and stores the api key on mount', async () => {
  const wrapper = mount(SetupAPIKeyView, {
    global: {
      stubs: {
        'n-button': { emits: ['click'], template: `<button @click="$emit('click')"><slot /></button>` }
      }
    }
  })

  await flushPromises()

  expect(apiPost).toHaveBeenCalledWith('/api/setup/api-key')
  expect(setAPIKey).toHaveBeenCalledWith('12345678123456781234567812345678')
  expect(wrapper.text()).toContain('12345678123456781234567812345678')
  expect(wrapper.text()).not.toContain('跳过')
  expect(wrapper.find('input').exists()).toBe(false)
})
```

Update `web/src/stores/setup.test.ts` so it no longer calls `skipAPIKey` and expects `createAPIKey()` without a name.

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
npm --prefix web run test -- SetupAPIKeyView.test.ts setup.test.ts
```

Expected: FAIL because the view still requires clicking create and the store still supports skip.

- [ ] **Step 3: Implement setup store and view**

Update `web/src/stores/setup.ts`:

```ts
import { apiGet, apiPost, setAPIKey } from '@/api/client'

async createAPIKey() {
  this.createdAPIKey = await apiPost<APIKeySetupResponse>('/api/setup/api-key')
  setAPIKey(this.createdAPIKey.key)
  await this.load()
  return this.createdAPIKey
}
```

Remove `skipAPIKey`.

Replace `SetupAPIKeyView.vue` script:

```ts
import { useMessage } from 'naive-ui'
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useSetupStore } from '@/stores/setup'

const router = useRouter()
const message = useMessage()
const setup = useSetupStore()
const createdKey = ref('')

async function ensureKey() {
  try {
    const response = await setup.createAPIKey()
    createdKey.value = response.key
    message.success('API 密钥已自动生成')
  } catch (error) {
    message.error(error instanceof Error ? error.message : '无法生成 API 密钥')
  }
}

onMounted(ensureKey)
```

Replace template controls with one continue button:

```vue
<div v-if="createdKey" class="key-result">
  <p>API 密钥</p>
  <code>{{ createdKey }}</code>
  <n-button type="primary" @click="router.push('/setup/telegram-api')">继续</n-button>
</div>
<n-button v-else type="primary" :loading="setup.loading" disabled>正在生成密钥</n-button>
```

- [ ] **Step 4: Run setup frontend tests**

Run:

```bash
npm --prefix web run test -- SetupAPIKeyView.test.ts setup.test.ts
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/stores/setup.ts web/src/stores/setup.test.ts web/src/views/SetupAPIKeyView.vue web/src/views/SetupAPIKeyView.test.ts
git commit -m "feat: auto-generate setup api key"
```

---

### Task 6: Settings API Key Panel

**Files:**
- Create: `web/src/stores/apiKey.ts`
- Modify: `web/src/views/SettingsView.vue`
- Test: `web/src/views/SettingsView.test.ts`

- [ ] **Step 1: Write failing settings tests**

Create `web/src/views/SettingsView.test.ts`:

```ts
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiGet, apiPost, setAPIKey } from '@/api/client'
import SettingsView from './SettingsView.vue'

vi.mock('naive-ui', async () => {
  const actual = await vi.importActual<typeof import('naive-ui')>('naive-ui')
  return {
    ...actual,
    useMessage: () => ({ error: vi.fn(), success: vi.fn() })
  }
})

vi.mock('@/api/client', () => ({
  apiGet: vi.fn().mockResolvedValue({
    id: 1,
    name: 'default',
    prefix: '12345678',
    key: '12345678123456781234567812345678',
    created_at: '2026-06-08T00:00:00Z'
  }),
  apiPost: vi.fn().mockResolvedValue({
    id: 2,
    name: 'default',
    prefix: '87654321',
    key: '87654321876543218765432187654321',
    created_at: '2026-06-08T01:00:00Z'
  }),
  setAPIKey: vi.fn()
}))

describe('SettingsView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('loads and displays the full api key', async () => {
    const wrapper = mount(SettingsView, {
      global: {
        stubs: {
          'n-button': { emits: ['click'], template: `<button @click="$emit('click')"><slot /></button>` }
        }
      }
    })
    await flushPromises()

    expect(apiGet).toHaveBeenCalledWith('/api/settings/api-key')
    expect(setAPIKey).toHaveBeenCalledWith('12345678123456781234567812345678')
    expect(wrapper.text()).toContain('12345678123456781234567812345678')
    expect(wrapper.text()).toContain('12345678')
  })

  it('regenerates and displays the replacement key', async () => {
    const wrapper = mount(SettingsView, {
      global: {
        stubs: {
          'n-button': { emits: ['click'], template: `<button @click="$emit('click')"><slot /></button>` }
        }
      }
    })
    await flushPromises()
    await wrapper.findAll('button').at(0)?.trigger('click')
    await flushPromises()

    expect(apiPost).toHaveBeenCalledWith('/api/settings/api-key/regenerate')
    expect(setAPIKey).toHaveBeenCalledWith('87654321876543218765432187654321')
    expect(wrapper.text()).toContain('87654321876543218765432187654321')
  })
})
```

- [ ] **Step 2: Run test to verify failure**

Run:

```bash
npm --prefix web run test -- SettingsView.test.ts
```

Expected: FAIL because the store and panel do not exist.

- [ ] **Step 3: Implement store and panel**

Create `web/src/stores/apiKey.ts`:

```ts
import { defineStore } from 'pinia'
import { apiGet, apiPost, setAPIKey } from '@/api/client'
import type { APIKeyResponse } from '@/api/types'

export const useAPIKeyStore = defineStore('apiKey', {
  state: () => ({
    current: undefined as APIKeyResponse | undefined,
    loading: false,
    error: ''
  }),
  actions: {
    async load() {
      this.loading = true
      this.error = ''
      try {
        this.current = await apiGet<APIKeyResponse>('/api/settings/api-key')
        setAPIKey(this.current.key)
        return this.current
      } catch (error) {
        this.error = error instanceof Error ? error.message : '无法加载 API 密钥'
        throw error
      } finally {
        this.loading = false
      }
    },
    async regenerate() {
      this.loading = true
      this.error = ''
      try {
        this.current = await apiPost<APIKeyResponse>('/api/settings/api-key/regenerate')
        setAPIKey(this.current.key)
        return this.current
      } catch (error) {
        this.error = error instanceof Error ? error.message : '无法重新生成 API 密钥'
        throw error
      } finally {
        this.loading = false
      }
    }
  }
})
```

Update `SettingsView.vue` script:

```vue
<script setup lang="ts">
import { useMessage } from 'naive-ui'
import { onMounted } from 'vue'
import { useAPIKeyStore } from '@/stores/apiKey'

const message = useMessage()
const apiKey = useAPIKeyStore()

onMounted(() => {
  apiKey.load().catch((error) => {
    message.error(error instanceof Error ? error.message : '无法加载 API 密钥')
  })
})

async function regenerate() {
  try {
    await apiKey.regenerate()
    message.success('API 密钥已重新生成')
  } catch (error) {
    message.error(error instanceof Error ? error.message : '无法重新生成 API 密钥')
  }
}

function formatTime(value?: string) {
  return value ? new Date(value).toLocaleString() : '-'
}
</script>
```

Replace the admin panel in the template with:

```vue
<section class="panel api-key-panel">
  <div class="panel-header">
    <h2>API 密钥</h2>
    <n-button size="small" type="primary" :loading="apiKey.loading" @click="regenerate">重新生成</n-button>
  </div>
  <dl v-if="apiKey.current">
    <div>
      <dt>前缀</dt>
      <dd>{{ apiKey.current.prefix }}</dd>
    </div>
    <div>
      <dt>创建时间</dt>
      <dd>{{ formatTime(apiKey.current.created_at) }}</dd>
    </div>
    <div>
      <dt>最后使用</dt>
      <dd>{{ formatTime(apiKey.current.last_used_at) }}</dd>
    </div>
  </dl>
  <code v-if="apiKey.current">{{ apiKey.current.key }}</code>
  <p v-else>正在加载 API 密钥</p>
</section>
```

Add CSS:

```css
.panel-header {
  align-items: center;
  display: flex;
  justify-content: space-between;
  gap: 12px;
}

.api-key-panel {
  display: grid;
  gap: 12px;
}

code {
  background: #f6f8fb;
  border: 1px solid #d9dee7;
  border-radius: 6px;
  color: #101828;
  display: block;
  overflow-wrap: anywhere;
  padding: 8px;
}
```

- [ ] **Step 4: Run settings test**

Run:

```bash
npm --prefix web run test -- SettingsView.test.ts
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/stores/apiKey.ts web/src/views/SettingsView.vue web/src/views/SettingsView.test.ts
git commit -m "feat: add api key settings panel"
```

---

### Task 7: Documentation And Full Verification

**Files:**
- Modify: `docs/api.md`

- [ ] **Step 1: Update API docs**

Add this section near the top of `docs/api.md` after the error response:

```markdown
## Authentication

Business API endpoints require an API key. Send it with either header:

```text
Authorization: Bearer <api-key>
X-API-Key: <api-key>
```

Health, readiness, setup, login, session, and API key management endpoints are available without an API key where required for bootstrap. API key management requires an authenticated admin session.
```

Add settings endpoints:

```markdown
### `GET /api/settings/api-key`

Returns the active API key. Requires an authenticated admin session. The full key is returned so it can be viewed in settings.

### `POST /api/settings/api-key/regenerate`

Creates a replacement API key, disables old keys, and returns the new full key.
```

Update `POST /api/setup/api-key` wording to say it auto-generates the first API key and has no request body.

- [ ] **Step 2: Run backend verification**

Run:

```bash
go test ./...
```

Expected: PASS for all Go packages.

- [ ] **Step 3: Run frontend verification**

Run:

```bash
npm --prefix web run test
npm --prefix web run typecheck
```

Expected: PASS for all Vitest tests and Vue type checking.

- [ ] **Step 4: Check worktree status**

Run:

```bash
git status --short
```

Expected: only intended files changed.

- [ ] **Step 5: Commit docs and any verification fixes**

```bash
git add docs/api.md
git commit -m "docs: document api key authentication"
```

If verification required code fixes, include the exact fixed files in the same commit only when the fixes are directly tied to this feature.

---

## Self-Review Notes

- Spec coverage: mandatory API key access is covered by Task 3; auto-generated setup key by Tasks 2, 3, and 5; viewable/regenerable full key by Tasks 2, 3, and 6; frontend header injection by Task 4; docs by Task 7.
- Scope check: this remains one feature spanning backend auth and frontend settings, with one active key only. Multiple keys, scopes, deletion, and external secret management stay out of scope.
- Type consistency: response type is `model.APIKeyResponse` in Go and `APIKeyResponse` in TypeScript; setup aliases to the same shape.
