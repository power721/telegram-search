# Phase 1A Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish the fresh-install `tg-search` foundation: project naming, default storage layout, storage quota config, admin setup/auth persistence, setup/auth APIs, and storage usage reporting.

**Architecture:** Treat the project as greenfield. There are no existing users and no deployed database to preserve, so this phase may reset the SQLite schema baseline instead of writing any old-version upgrade path. Keep the existing Go/Gin/SQLite structure, but rename it to `tg-search` and add foundation repositories/services/API handlers without changing Telegram sync/search behavior yet.

**Tech Stack:** Go 1.25, Gin, SQLite via `modernc.org/sqlite`, bcrypt from `golang.org/x/crypto/bcrypt`, YAML config, existing repository/service/API test patterns.

---

## Greenfield Assumption

This project has no production users and no database compatibility requirement.

Implications:

- Do not write old-data migration steps.
- Do not preserve `/data/tg-provider` defaults.
- Do not support both `tg-provider` and `tg-search` names.
- `internal/db/migrations.go` remains useful as a schema initializer for fresh SQLite databases, but the plan may reset its contents to a new `tg-search` baseline.
- Tests should verify fresh database creation, not upgrade behavior from old schemas.

## Future Phase Index

Writing only Phase 1A will not lose later work. Later phase plans should be generated from the product spec:

[tg-search Product Redesign Design](/home/harold/workspace/telegram-search/docs/superpowers/specs/2026-06-08-tg-search-product-redesign-design.md)

Planned sequence:

- **Phase 1A Foundation:** this plan.
- **Phase 1B Admin Shell:** Vue 3 app, login UI, shell layout, dashboard skeleton.
- **Phase 1C Telegram Onboarding:** Telegram API setup, phone/code/2FA login, account state, metadata sync.
- **Phase 1D Channel Control:** channel table, Sync Profile selection, Web Access Detection, listen rules, remote-search entry point.
- **Phase 1E Index/Search/Resources:** `telegram_message_contents`, `telegram_sync_cursors`, FTS updates, Global Search, Telegram Resource Library.
- **Phase 1F Runtime Reliability:** persistent tasks, SSE, FloodWait, reconnect, gap recovery, retry/cancel/pause.
- **Phase 1G Packaging/Ops:** Docker, Compose, release docs, backup, logs, smoke tests.

When Phase 1A is done, write Phase 1B from the same spec and this index. Do not rely on conversation memory.

## Scope

In scope:

- Rename module/import path and binary entry from `tg-provider` to `tg-search`.
- Change default config path and storage path to `/data/tg-search`.
- Add runtime directories: `uploads`, `index`, `thumbnails`.
- Add storage quota config: `storage.max_db_size`, `storage.max_media_cache`.
- Reset fresh SQLite schema baseline to include foundation tables.
- Add admin users, API keys, settings, and minimal auth/session support.
- Add setup status/admin/API key endpoints.
- Add auth login/logout/me endpoints for the future admin console.
- Add storage usage API.
- Update README and API docs to use `tg-search`.

Out of scope:

- Vue frontend.
- Telegram onboarding redesign.
- Sync Profile behavior inside history sync.
- Message contents split.
- Sync cursor table.
- Global Search.
- Telegram Resource Library.
- Persistent task runtime redesign.

## File Structure

- Move `cmd/tg-provider/main.go` to `cmd/tg-search/main.go`.
- Modify `go.mod`: module name becomes `tg-search`.
- Modify all Go imports from `tg-provider/...` to `tg-search/...`.
- Modify `internal/config/config.go`: `tg-search` defaults, storage quota fields, runtime directories.
- Add `internal/config/size.go`: parse human-readable byte sizes.
- Modify `internal/db/migrations.go`: reset fresh baseline schema for `tg-search`.
- Modify `internal/model/model.go`: add `User`, `APIKey`, `SetupStatus`, `StorageUsage`.
- Add `internal/repository/user.go`.
- Add `internal/repository/api_key.go`.
- Add `internal/repository/settings.go`.
- Add `internal/adminauth/service.go`.
- Add `internal/storage/usage.go`.
- Modify `internal/api/router.go`: setup/auth/storage routes and dependencies.
- Modify `internal/api/handlers.go`: setup/auth/storage handlers.
- Modify `cmd/tg-search/main.go`: wire new services.
- Update `README.md`, `docs/api.md`, `docs/api-response-contract.md`, `docs/production-deployment-checklist.md`, `docs/smoke-test-guide.md`.

## Task 1: Rename Module And Binary

**Files:**

- Move: `cmd/tg-provider/main.go` -> `cmd/tg-search/main.go`
- Modify: `go.mod`
- Modify: Go import paths under `cmd/` and `internal/`
- Modify: `README.md`, `docs/api.md`

- [ ] **Step 1: Inspect current names**

Run:

```bash
rg -n '"tg-provider/|module tg-provider|cmd/tg-provider|tg-provider|/data/tg-provider' go.mod cmd internal README.md docs
```

Expected: matches show the current old name and old data directory.

- [ ] **Step 2: Move command entry**

Run:

```bash
mkdir -p cmd/tg-search
git mv cmd/tg-provider/main.go cmd/tg-search/main.go
rmdir cmd/tg-provider
```

Expected: `git status --short` shows the file rename.

- [ ] **Step 3: Rename module and Go imports**

Run:

```bash
perl -0pi -e 's/module tg-provider/module tg-search/g; s/"tg-provider\//"tg-search\//g' go.mod $(rg --files -g '*.go')
```

Expected: `rg -n '"tg-provider/|module tg-provider' go.mod cmd internal` returns no matches.

- [ ] **Step 4: Rename runtime log strings**

In `cmd/tg-search/main.go`, replace:

```go
logs.App.Info("tg-provider starting", zap.String("address", config.Address(cfg)))
```

with:

```go
logs.App.Info("tg-search starting", zap.String("address", config.Address(cfg)))
```

Replace:

```go
logs.App.Info("tg-provider stopped")
```

with:

```go
logs.App.Info("tg-search stopped")
```

- [ ] **Step 5: Verify compile**

Run:

```bash
gofmt -w $(rg --files -g '*.go')
go test ./internal/config ./internal/db ./internal/repository ./internal/api
```

Expected: all packages pass.

- [ ] **Step 6: Commit**

Run:

```bash
git add go.mod cmd internal README.md docs/api.md
git commit -m "chore: rename project to tg-search"
```

## Task 2: Add Storage Quota Config

**Files:**

- Create: `internal/config/size.go`
- Create: `internal/config/size_test.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Add failing size parser tests**

Create `internal/config/size_test.go`:

```go
package config

import "testing"

func TestParseSize(t *testing.T) {
	tests := []struct {
		input string
		want  Size
	}{
		{"10GB", Size(10 * 1000 * 1000 * 1000)},
		{"20gb", Size(20 * 1000 * 1000 * 1000)},
		{"512MB", Size(512 * 1000 * 1000)},
		{"1024", Size(1024)},
	}
	for _, tt := range tests {
		got, err := ParseSize(tt.input)
		if err != nil {
			t.Fatalf("ParseSize(%q) error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Fatalf("ParseSize(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseSizeRejectsInvalidValue(t *testing.T) {
	for _, input := range []string{"", "abc", "-1GB", "10XB"} {
		if _, err := ParseSize(input); err == nil {
			t.Fatalf("ParseSize(%q) returned nil error", input)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/config -run 'TestParseSize' -v
```

Expected: FAIL because `Size` and `ParseSize` are undefined.

- [ ] **Step 3: Implement size parser**

Create `internal/config/size.go`:

```go
package config

import (
	"fmt"
	"strconv"
	"strings"
)

type Size int64

func ParseSize(input string) (Size, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return 0, fmt.Errorf("size is required")
	}
	upper := strings.ToUpper(value)
	multiplier := int64(1)
	for _, suffix := range []struct {
		text string
		mul  int64
	}{
		{"GB", 1000 * 1000 * 1000},
		{"MB", 1000 * 1000},
		{"KB", 1000},
		{"B", 1},
	} {
		if strings.HasSuffix(upper, suffix.text) {
			multiplier = suffix.mul
			value = strings.TrimSpace(value[:len(value)-len(suffix.text)])
			break
		}
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse size %q: %w", input, err)
	}
	if n < 0 {
		return 0, fmt.Errorf("size must be non-negative")
	}
	return Size(n * multiplier), nil
}

func (s *Size) UnmarshalYAML(unmarshal func(any) error) error {
	var raw any
	if err := unmarshal(&raw); err != nil {
		return err
	}
	switch value := raw.(type) {
	case int:
		*s = Size(value)
		return nil
	case int64:
		*s = Size(value)
		return nil
	case string:
		parsed, err := ParseSize(value)
		if err != nil {
			return err
		}
		*s = parsed
		return nil
	default:
		return fmt.Errorf("unsupported size value %T", raw)
	}
}
```

- [ ] **Step 4: Update config defaults**

In `internal/config/config.go`, set:

```go
const DefaultPath = "/data/tg-search/config.yaml"
```

Change `StorageConfig`:

```go
type StorageConfig struct {
	Path           string `yaml:"path"`
	MaxDBSize      Size   `yaml:"max_db_size"`
	MaxMediaCache  Size   `yaml:"max_media_cache"`
}
```

Use defaults:

```go
Storage: StorageConfig{
	Path:          "/data/tg-search",
	MaxDBSize:     Size(10 * 1000 * 1000 * 1000),
	MaxMediaCache: Size(20 * 1000 * 1000 * 1000),
},
```

In `EnsureRuntimeDirs`, create:

```go
filepath.Join(cfg.Storage.Path, "sessions"),
filepath.Join(cfg.Storage.Path, "logs"),
filepath.Join(cfg.Storage.Path, "backup"),
filepath.Join(cfg.Storage.Path, "uploads"),
filepath.Join(cfg.Storage.Path, "index"),
filepath.Join(cfg.Storage.Path, "thumbnails"),
```

- [ ] **Step 5: Update config tests**

In `internal/config/config_test.go`, assert:

```go
if cfg.Storage.Path != "/data/tg-search" {
	t.Fatalf("storage path = %q, want /data/tg-search", cfg.Storage.Path)
}
if cfg.Storage.MaxDBSize != Size(10*1000*1000*1000) {
	t.Fatalf("max db size = %d, want 10GB", cfg.Storage.MaxDBSize)
}
if cfg.Storage.MaxMediaCache != Size(20*1000*1000*1000) {
	t.Fatalf("max media cache = %d, want 20GB", cfg.Storage.MaxMediaCache)
}
```

Update runtime directory assertion:

```go
for _, rel := range []string{"sessions", "logs", "backup", "uploads", "index", "thumbnails"} {
```

- [ ] **Step 6: Run tests and commit**

Run:

```bash
go test ./internal/config -v
git add internal/config
git commit -m "feat: add storage quota config"
```

Expected: tests pass and commit succeeds.

## Task 3: Reset Fresh SQLite Baseline

**Files:**

- Modify: `internal/db/migrations.go`
- Modify: `internal/db/db_test.go`
- Modify: `internal/model/model.go`

- [ ] **Step 1: Add fresh schema test**

In `internal/db/db_test.go`, add:

```go
func TestMigrateCreatesFreshFoundationSchema(t *testing.T) {
	ctx := context.Background()
	conn, err := Open(filepath.Join(t.TempDir(), "tg-search.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer conn.Close()
	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	for _, table := range []string{
		"telegram_accounts",
		"telegram_channels",
		"telegram_messages",
		"telegram_links",
		"telegram_watch_rules",
		"users",
		"api_keys",
		"settings",
	} {
		var name string
		err := conn.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if err != nil {
			t.Fatalf("table %s missing: %v", table, err)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/db -run TestMigrateCreatesFreshFoundationSchema -v
```

Expected: FAIL because `users`, `api_keys`, and `settings` do not exist yet.

- [ ] **Step 3: Reset baseline schema**

Modify `internal/db/migrations.go` so fresh migration version 1 creates the current project baseline:

- `telegram_accounts`
- `telegram_channels` with `web_access`, `web_access_checked_at`, `web_access_error`
- `telegram_messages`
- `telegram_links` with `note`
- `telegram_watch_rules`
- `users`
- `api_keys`
- `settings`
- current indexes
- current FTS5 table and triggers

Keep the schema runner, but remove old-version `ALTER TABLE` upgrade entries. The file can still use multiple entries if useful for organization, but they should represent fresh schema setup, not upgrade behavior.

- [ ] **Step 4: Add foundation model structs**

In `internal/model/model.go`, add:

```go
const UserRoleAdmin = "admin"

type User struct {
	ID           int64      `json:"id"`
	Username     string     `json:"username"`
	PasswordHash string     `json:"-"`
	Role         string     `json:"role"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type APIKey struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	KeyHash    string     `json:"-"`
	Prefix     string     `json:"prefix"`
	Enabled    bool       `json:"enabled"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type SetupStatus struct {
	Complete           bool `json:"complete"`
	AdminConfigured    bool `json:"admin_configured"`
	APIKeyConfigured   bool `json:"api_key_configured"`
	TelegramConfigured bool `json:"telegram_configured"`
}

type StorageUsage struct {
	DBBytes         int64 `json:"db_bytes"`
	IndexBytes      int64 `json:"index_bytes"`
	MediaCacheBytes int64 `json:"media_cache_bytes"`
	TotalBytes      int64 `json:"total_bytes"`
	MaxDBBytes      int64 `json:"max_db_bytes"`
	MaxMediaBytes   int64 `json:"max_media_bytes"`
	DBOverQuota     bool  `json:"db_over_quota"`
	MediaOverQuota  bool  `json:"media_over_quota"`
}
```

- [ ] **Step 5: Run tests and commit**

Run:

```bash
go test ./internal/db ./internal/model ./internal/repository
git add internal/db internal/model
git commit -m "feat: reset fresh tg-search schema baseline"
```

Expected: tests pass and commit succeeds.

## Task 4: Add Foundation Repositories

**Files:**

- Create: `internal/repository/user.go`
- Create: `internal/repository/user_test.go`
- Create: `internal/repository/api_key.go`
- Create: `internal/repository/api_key_test.go`
- Create: `internal/repository/settings.go`
- Create: `internal/repository/settings_test.go`

- [ ] **Step 1: Add user repository test**

Create `internal/repository/user_test.go`:

```go
package repository

import (
	"context"
	"path/filepath"
	"testing"

	"tg-search/internal/db"
	"tg-search/internal/model"
)

func TestUserRepositoryCreatesAndFindsAdmin(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "tg-search.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewUserRepository(conn)
	id, err := repo.Create(ctx, model.User{Username: "admin", PasswordHash: "hash", Role: model.UserRoleAdmin})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	user, err := repo.FindByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("find user: %v", err)
	}
	if user.ID != id || user.PasswordHash != "hash" || user.Role != model.UserRoleAdmin {
		t.Fatalf("user = %+v", user)
	}
	count, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
}
```

- [ ] **Step 2: Add API key repository test**

Create `internal/repository/api_key_test.go`:

```go
package repository

import (
	"context"
	"path/filepath"
	"testing"

	"tg-search/internal/db"
	"tg-search/internal/model"
)

func TestAPIKeyRepositoryCreatesAndCountsKeys(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "tg-search.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewAPIKeyRepository(conn)
	id, err := repo.Create(ctx, model.APIKey{Name: "cli", KeyHash: "hash", Prefix: "abcd1234", Enabled: true})
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}
	if id == 0 {
		t.Fatal("api key id = 0")
	}
	count, err := repo.CountEnabled(ctx)
	if err != nil {
		t.Fatalf("count enabled: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
}
```

- [ ] **Step 3: Add settings repository test**

Create `internal/repository/settings_test.go`:

```go
package repository

import (
	"context"
	"path/filepath"
	"testing"

	"tg-search/internal/db"
)

func TestSettingsRepositoryUpsertsValues(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "tg-search.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewSettingsRepository(conn)
	if err := repo.Set(ctx, "setup.complete", `true`); err != nil {
		t.Fatalf("set: %v", err)
	}
	value, ok, err := repo.Get(ctx, "setup.complete")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !ok || value != `true` {
		t.Fatalf("value=%q ok=%v, want true", value, ok)
	}
}
```

- [ ] **Step 4: Implement repositories**

Create repository files with these methods:

```go
func NewUserRepository(db *sql.DB) *UserRepository
func (r *UserRepository) Create(ctx context.Context, user model.User) (int64, error)
func (r *UserRepository) FindByUsername(ctx context.Context, username string) (model.User, error)
func (r *UserRepository) Count(ctx context.Context) (int64, error)
```

```go
func NewAPIKeyRepository(db *sql.DB) *APIKeyRepository
func (r *APIKeyRepository) Create(ctx context.Context, key model.APIKey) (int64, error)
func (r *APIKeyRepository) CountEnabled(ctx context.Context) (int64, error)
```

```go
func NewSettingsRepository(db *sql.DB) *SettingsRepository
func (r *SettingsRepository) Set(ctx context.Context, key string, valueJSON string) error
func (r *SettingsRepository) Get(ctx context.Context, key string) (string, bool, error)
```

- [ ] **Step 5: Run tests and commit**

Run:

```bash
go test ./internal/repository -v
git add internal/repository
git commit -m "feat: add foundation repositories"
```

Expected: tests pass and commit succeeds.

## Task 5: Add Admin Auth And Storage Usage Services

**Files:**

- Create: `internal/adminauth/service.go`
- Create: `internal/adminauth/service_test.go`
- Create: `internal/storage/usage.go`
- Create: `internal/storage/usage_test.go`

- [ ] **Step 1: Add admin auth service test**

Create `internal/adminauth/service_test.go`:

```go
package adminauth

import (
	"context"
	"path/filepath"
	"testing"

	"tg-search/internal/db"
	"tg-search/internal/model"
	"tg-search/internal/repository"
)

func TestServiceCreatesAdminAndAuthenticates(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "tg-search.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	users := repository.NewUserRepository(conn)
	service := NewService(users)
	if _, err := service.CreateAdmin(ctx, "admin", "secret123"); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	user, err := service.Authenticate(ctx, "admin", "secret123")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if user.Username != "admin" || user.Role != model.UserRoleAdmin {
		t.Fatalf("user = %+v", user)
	}
	if _, err := service.Authenticate(ctx, "admin", "wrong"); err == nil {
		t.Fatal("Authenticate with wrong password returned nil error")
	}
}
```

- [ ] **Step 2: Add storage usage test**

Create `internal/storage/usage_test.go`:

```go
package storage

import (
	"os"
	"path/filepath"
	"testing"

	"tg-search/internal/config"
)

func TestUsageCountsDBIndexAndMediaCache(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "tg-search.db"), 10)
	writeFile(t, filepath.Join(root, "index", "fts.data"), 20)
	writeFile(t, filepath.Join(root, "thumbnails", "thumb.bin"), 30)
	service := NewUsageService(config.Config{
		Storage: config.StorageConfig{
			Path:          root,
			MaxDBSize:     config.Size(9),
			MaxMediaCache: config.Size(100),
		},
	})
	usage, err := service.Usage()
	if err != nil {
		t.Fatalf("Usage returned error: %v", err)
	}
	if usage.DBBytes != 10 || usage.IndexBytes != 20 || usage.MediaCacheBytes != 30 || usage.TotalBytes != 60 {
		t.Fatalf("usage = %+v", usage)
	}
	if !usage.DBOverQuota {
		t.Fatalf("DBOverQuota = false, want true")
	}
	if usage.MediaOverQuota {
		t.Fatalf("MediaOverQuota = true, want false")
	}
}

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
```

- [ ] **Step 3: Implement services**

Create `internal/adminauth/service.go` with:

- `NewService(users *repository.UserRepository) *Service`
- `CreateAdmin(ctx, username, password) (int64, error)`
- `Authenticate(ctx, username, password) (model.User, error)`
- `CreateSession(user model.User) (string, error)`
- `UserForSession(token string) (model.User, bool)`
- `DeleteSession(token string)`

Use bcrypt for password hashes and an in-memory session map for Phase 1A.

Create `internal/storage/usage.go` with:

- `NewUsageService(cfg config.Config) *UsageService`
- `Usage() (model.StorageUsage, error)`

Count:

- `/data/tg-search/tg-search.db`
- `/data/tg-search/index`
- `/data/tg-search/thumbnails`

- [ ] **Step 4: Run tests and commit**

Run:

```bash
go test ./internal/adminauth ./internal/storage -v
git add internal/adminauth internal/storage
git commit -m "feat: add auth and storage foundation services"
```

Expected: tests pass and commit succeeds.

## Task 6: Add Setup, Auth, And Storage APIs

**Files:**

- Modify: `internal/api/router.go`
- Modify: `internal/api/handlers.go`
- Modify: `internal/api/handlers_test.go`
- Modify: `cmd/tg-search/main.go`

- [ ] **Step 1: Add API tests**

Add tests in `internal/api/handlers_test.go` for:

- `GET /api/setup/status` returns `admin_configured=false` on fresh DB.
- `POST /api/setup/admin` creates admin and returns `201`.
- `POST /api/auth/login` sets an `HttpOnly` session cookie.
- `GET /api/auth/me` returns the logged-in admin when cookie is present.
- `POST /api/auth/logout` clears the session.
- `GET /api/storage/usage` returns DB/index/media/total byte fields.

- [ ] **Step 2: Add dependencies and routes**

In `internal/api/router.go`, add dependencies:

```go
Users        *repository.UserRepository
APIKeys      *repository.APIKeyRepository
Settings     *repository.SettingsRepository
AdminAuth    *adminauth.Service
StorageUsage *storage.UsageService
```

Add routes:

```go
api.GET("/setup/status", h.setupStatus)
api.POST("/setup/admin", h.setupAdmin)
api.POST("/setup/api-key", h.setupAPIKey)
api.POST("/setup/complete", h.setupComplete)
api.POST("/auth/login", h.authLogin)
api.POST("/auth/logout", h.authLogout)
api.GET("/auth/me", h.authMe)
api.GET("/storage/usage", h.storageUsage)
```

- [ ] **Step 3: Implement handlers**

Implement:

- `setupStatus`
- `setupAdmin`
- `setupAPIKey`
- `setupComplete`
- `authLogin`
- `authLogout`
- `authMe`
- `storageUsage`

Use cookie name:

```go
const adminSessionCookie = "tg_search_session"
```

Use `HttpOnly` cookie options:

```go
c.SetCookie(adminSessionCookie, token, 86400, "/", "", false, true)
```

For API keys, generate 32 random bytes, encode hex, bcrypt-hash the key, store an 8-character prefix, and return the plaintext key only in the creation response.

- [ ] **Step 4: Wire `cmd/tg-search/main.go`**

After existing repositories:

```go
users := repository.NewUserRepository(conn)
apiKeys := repository.NewAPIKeyRepository(conn)
settings := repository.NewSettingsRepository(conn)
adminAuth := adminauth.NewService(users)
storageUsage := storage.NewUsageService(cfg)
```

Pass these into `api.Dependencies`.

- [ ] **Step 5: Run tests and commit**

Run:

```bash
go test ./internal/api ./internal/adminauth ./internal/repository ./internal/storage
git add internal/api cmd/tg-search/main.go
git commit -m "feat: add setup auth and storage APIs"
```

Expected: tests pass and commit succeeds.

## Task 7: Update Docs And Verify

**Files:**

- Modify: `README.md`
- Modify: `docs/api.md`
- Modify: `docs/api-response-contract.md`
- Modify: `docs/production-deployment-checklist.md`
- Modify: `docs/smoke-test-guide.md`

- [ ] **Step 1: Update docs**

Document:

- Product and binary name: `tg-search`.
- Default data directory: `/data/tg-search`.
- Config quota fields:

```yaml
storage:
  path: /data/tg-search
  max_db_size: 10GB
  max_media_cache: 20GB
```

- Foundation APIs:

```text
GET    /api/setup/status
POST   /api/setup/admin
POST   /api/setup/api-key
POST   /api/setup/complete
POST   /api/auth/login
POST   /api/auth/logout
GET    /api/auth/me
GET    /api/storage/usage
```

- Storage usage response:

```json
{
  "db_bytes": 3200000000,
  "index_bytes": 1100000000,
  "media_cache_bytes": 0,
  "total_bytes": 4300000000,
  "max_db_bytes": 10000000000,
  "max_media_bytes": 20000000000,
  "db_over_quota": false,
  "media_over_quota": false
}
```

- [ ] **Step 2: Run full verification**

Run:

```bash
go test ./...
rg -n 'tg-provider|/data/tg-provider|cmd/tg-provider' README.md docs/api.md docs/api-response-contract.md docs/production-deployment-checklist.md docs/smoke-test-guide.md cmd internal go.mod
```

Expected:

- `go test ./...` passes.
- `rg` returns no matches in active code/docs. Historical design docs may still mention the old name as background, but active runtime docs should not.

- [ ] **Step 3: Commit**

Run:

```bash
git add README.md docs/api.md docs/api-response-contract.md docs/production-deployment-checklist.md docs/smoke-test-guide.md
git commit -m "docs: update tg-search foundation docs"
```

## Plan Self-Review

Spec coverage for Phase 1A:

- Product rename: Task 1.
- `/data/tg-search` default and runtime layout: Task 2.
- Storage quota config: Task 2.
- Fresh foundation schema: Task 3.
- Admin setup data model: Task 3 and Task 4.
- API key persistence: Task 3 and Task 4.
- Setup/auth API foundation: Task 5 and Task 6.
- Storage usage reporting: Task 5 and Task 6.
- Documentation: Task 7.

Greenfield constraints:

- No old user compatibility.
- No old database migration path.
- No `/data/tg-provider` fallback.
- No dual naming.

Deferred to later phase plans:

- Vue frontend: Phase 1B.
- Telegram onboarding redesign: Phase 1C.
- Channel control and Sync Profile behavior: Phase 1D.
- Message contents split and sync cursors: Phase 1E.
- Global Search and Telegram Resource Library: Phase 1E.
- Persistent task runtime: Phase 1F.
- Docker/release ops completion: Phase 1G.

Verification commands:

```bash
go test ./...
rg -n 'tg-provider|/data/tg-provider|cmd/tg-provider' README.md docs/api.md docs/api-response-contract.md docs/production-deployment-checklist.md docs/smoke-test-guide.md cmd internal go.mod
```
