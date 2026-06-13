# Avatar Async Download Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement async background download of account and channel avatars with local file storage and manual sync button.

**Architecture:** New `internal/avatar` package manages download queue (using `scheduler.RetryQueue`), file storage (`storage.Path/avatars/{type}/{id}/{photoID}.jpg`), and hooks into login/sync flows. API endpoints check local files first before on-demand download.

**Tech Stack:** Go stdlib, existing `scheduler.RetryQueue`, `medialimit.Limiter`, Telegram gotd client

---

## File Structure

**New files:**
- `internal/avatar/service.go` — core avatar download service with queue management
- `internal/avatar/service_test.go` — service unit tests
- `internal/avatar/storage.go` — file path helpers and atomic write operations
- `internal/avatar/storage_test.go` — storage unit tests

**Modified files:**
- `internal/api/account_avatar.go` — check local file before on-demand download
- `internal/api/channel_avatar.go` — check local file before on-demand download
- `internal/api/handlers.go` — add manual sync endpoint, trigger avatar downloads after login
- `internal/api/router.go` — register manual sync route
- `cmd/tg-search/main.go` — initialize avatar service and queue
- `web/src/views/AccountsView.vue` — add "Sync Avatar" button, display avatars
- `web/src/api/admin.ts` — add `syncAccountAvatar()` API call

---

### Task 1: Create Avatar Storage Layer

**Files:**
- Create: `internal/avatar/storage.go`
- Create: `internal/avatar/storage_test.go`

- [ ] **Step 1: Write failing tests for avatar file paths**

```go
package avatar

import (
	"path/filepath"
	"testing"
)

func TestAvatarPath(t *testing.T) {
	tests := []struct {
		name     string
		typ      string
		id       int64
		photoID  int64
		want     string
	}{
		{"account avatar", "account", 123, 456789, "avatars/account/123/456789.jpg"},
		{"channel avatar", "channel", 999, 111222, "avatars/channel/999/111222.jpg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AvatarPath(tt.typ, tt.id, tt.photoID)
			if got != tt.want {
				t.Errorf("AvatarPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAvatarAbsolutePath(t *testing.T) {
	root := "/data/tg-search"
	got := AvatarAbsolutePath(root, "account", 123, 456789)
	want := filepath.Join(root, "avatars/account/123/456789.jpg")
	if got != want {
		t.Errorf("AvatarAbsolutePath() = %v, want %v", got, want)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/avatar -v -run TestAvatarPath`
Expected: FAIL with "undefined: AvatarPath"

- [ ] **Step 3: Implement avatar path helpers**

```go
package avatar

import (
	"fmt"
	"path/filepath"
)

// AvatarPath returns the relative path for an avatar file.
// typ is "account" or "channel".
func AvatarPath(typ string, id int64, photoID int64) string {
	return filepath.Join("avatars", typ, fmt.Sprintf("%d", id), fmt.Sprintf("%d.jpg", photoID))
}

// AvatarAbsolutePath returns the absolute path for an avatar file.
func AvatarAbsolutePath(storageRoot string, typ string, id int64, photoID int64) string {
	return filepath.Join(storageRoot, AvatarPath(typ, id, photoID))
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/avatar -v -run TestAvatarPath`
Expected: PASS

- [ ] **Step 5: Write failing test for file existence check**

```go
func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jpg")
	
	if FileExists(path) {
		t.Error("FileExists() = true for non-existent file, want false")
	}
	
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	
	if !FileExists(path) {
		t.Error("FileExists() = false for existing file, want true")
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/avatar -v -run TestFileExists`
Expected: FAIL with "undefined: FileExists"

- [ ] **Step 7: Implement file existence check and atomic write**

```go
import (
	"io/fs"
	"os"
)

// FileExists returns true if the file exists at the given path.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// WriteAvatarFile writes avatar data atomically to the given path.
// Creates parent directories if needed.
func WriteAvatarFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create avatar dir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".avatar-tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("commit avatar file: %w", err)
	}
	return nil
}
```

- [ ] **Step 8: Write test for atomic write**

```go
func TestWriteAvatarFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "avatars", "account", "123", "456.jpg")
	data := []byte("test avatar data")
	
	if err := WriteAvatarFile(path, data); err != nil {
		t.Fatalf("WriteAvatarFile() error = %v", err)
	}
	
	if !FileExists(path) {
		t.Error("file not created")
	}
	
	readData, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(readData) != string(data) {
		t.Errorf("file content = %q, want %q", readData, data)
	}
}
```

- [ ] **Step 9: Run all storage tests**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/avatar -v`
Expected: PASS

- [ ] **Step 10: Commit storage layer**

```bash
git add internal/avatar/storage.go internal/avatar/storage_test.go
git commit -m "feat: add avatar storage layer with file path helpers

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Create Avatar Download Service

**Files:**
- Create: `internal/avatar/service.go`
- Create: `internal/avatar/service_test.go`

- [ ] **Step 1: Write failing test for service initialization**

```go
package avatar

import (
	"context"
	"testing"

	"go.uber.org/zap"
	
	"tg-search/internal/medialimit"
	"tg-search/internal/retry"
	"tg-search/internal/scheduler"
)

func TestNewService(t *testing.T) {
	queue := scheduler.NewRetryQueue(scheduler.RetryQueueOptions{
		Policy: retry.DefaultPolicy(),
		Logger: zap.NewNop(),
	})
	limiter := medialimit.New(5)
	
	svc := NewService(ServiceOptions{
		StorageRoot: "/tmp/test",
		Queue:       queue,
		Limiter:     limiter,
		Telegram:    nil,
		Logger:      zap.NewNop(),
	})
	
	if svc == nil {
		t.Fatal("NewService() returned nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/avatar -v -run TestNewService`
Expected: FAIL with "undefined: NewService"

- [ ] **Step 3: Implement service structure and constructor**

```go
package avatar

import (
	"context"
	"fmt"
	
	"go.uber.org/zap"
	
	"tg-search/internal/medialimit"
	"tg-search/internal/model"
	"tg-search/internal/scheduler"
	"tg-search/internal/telegram"
)

type ServiceOptions struct {
	StorageRoot string
	Queue       *scheduler.RetryQueue
	Limiter     *medialimit.Limiter
	Telegram    telegram.Client
	Logger      *zap.Logger
}

type Service struct {
	storageRoot string
	queue       *scheduler.RetryQueue
	limiter     *medialimit.Limiter
	telegram    telegram.Client
	logger      *zap.Logger
}

func NewService(opts ServiceOptions) *Service {
	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}
	if opts.Telegram == nil {
		opts.Telegram = telegram.NopClient{}
	}
	return &Service{
		storageRoot: opts.StorageRoot,
		queue:       opts.Queue,
		limiter:     opts.Limiter,
		telegram:    opts.Telegram,
		logger:      opts.Logger,
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/avatar -v -run TestNewService`
Expected: PASS

- [ ] **Step 5: Write failing test for account avatar enqueue**

```go
type mockTelegram struct {
	telegram.NopClient
	downloadUserAvatarCalled bool
}

func (m *mockTelegram) DownloadUserAvatar(ctx context.Context, session telegram.AccountSession, userID int64, photoID int64) (telegram.ImageData, error) {
	m.downloadUserAvatarCalled = true
	return telegram.ImageData{Data: []byte("avatar"), MIMEType: "image/jpeg"}, nil
}

func TestEnqueueAccountAvatar(t *testing.T) {
	tmpDir := t.TempDir()
	queue := scheduler.NewRetryQueue(scheduler.RetryQueueOptions{
		Policy: retry.DefaultPolicy(),
		Logger: zap.NewNop(),
	})
	limiter := medialimit.New(5)
	mock := &mockTelegram{}
	
	svc := NewService(ServiceOptions{
		StorageRoot: tmpDir,
		Queue:       queue,
		Limiter:     limiter,
		Telegram:    mock,
		Logger:      zap.NewNop(),
	})
	
	account := model.Account{
		ID:             123,
		Phone:          "+1234567890",
		TelegramUserID: 999,
		PhotoID:        456789,
	}
	
	job := svc.EnqueueAccountAvatar(context.Background(), account)
	if job.ID == "" {
		t.Fatal("EnqueueAccountAvatar() returned empty job ID")
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/avatar -v -run TestEnqueueAccountAvatar`
Expected: FAIL with "undefined: Service.EnqueueAccountAvatar"

- [ ] **Step 7: Implement EnqueueAccountAvatar method**

```go
// EnqueueAccountAvatar enqueues an account avatar download task.
// Skips if PhotoID is 0 or file already exists.
func (s *Service) EnqueueAccountAvatar(ctx context.Context, account model.Account) scheduler.RetryJob {
	if account.PhotoID <= 0 {
		return scheduler.RetryJob{}
	}
	path := AvatarAbsolutePath(s.storageRoot, "account", account.ID, account.PhotoID)
	if FileExists(path) {
		return scheduler.RetryJob{}
	}
	if s.queue == nil {
		return scheduler.RetryJob{}
	}
	
	name := fmt.Sprintf("avatar-account-%d-%d", account.ID, account.PhotoID)
	return s.queue.Enqueue(ctx, name, func(ctx context.Context) error {
		return s.downloadAccountAvatar(ctx, account, path)
	})
}

func (s *Service) downloadAccountAvatar(ctx context.Context, account model.Account, destPath string) error {
	// Check again in case another worker downloaded it
	if FileExists(destPath) {
		return nil
	}
	
	session := telegram.AccountSession{
		AccountID:   account.ID,
		Phone:       account.Phone,
		SessionPath: account.SessionPath,
	}
	
	downloadFn := func() error {
		img, err := s.telegram.DownloadUserAvatar(ctx, session, account.TelegramUserID, account.PhotoID)
		if err != nil {
			return fmt.Errorf("download user avatar: %w", err)
		}
		if err := WriteAvatarFile(destPath, img.Data); err != nil {
			return fmt.Errorf("write avatar file: %w", err)
		}
		return nil
	}
	
	if s.limiter != nil {
		return s.limiter.Run(ctx, downloadFn)
	}
	return downloadFn()
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/avatar -v -run TestEnqueueAccountAvatar`
Expected: PASS

- [ ] **Step 9: Write failing test for channel avatar enqueue**

```go
type mockTelegramChannel struct {
	telegram.NopClient
	downloadChannelAvatarCalled bool
}

func (m *mockTelegramChannel) DownloadChannelAvatar(ctx context.Context, session telegram.AccountSession, channelID int64, accessHash int64, photoID int64) (telegram.ImageData, error) {
	m.downloadChannelAvatarCalled = true
	return telegram.ImageData{Data: []byte("channel-avatar"), MIMEType: "image/jpeg"}, nil
}

func TestEnqueueChannelAvatar(t *testing.T) {
	tmpDir := t.TempDir()
	queue := scheduler.NewRetryQueue(scheduler.RetryQueueOptions{
		Policy: retry.DefaultPolicy(),
		Logger: zap.NewNop(),
	})
	limiter := medialimit.New(5)
	mock := &mockTelegramChannel{}
	
	svc := NewService(ServiceOptions{
		StorageRoot: tmpDir,
		Queue:       queue,
		Limiter:     limiter,
		Telegram:    mock,
		Logger:      zap.NewNop(),
	})
	
	account := model.Account{ID: 1, Phone: "+1234567890"}
	channel := model.Channel{
		ID:                2,
		AccountID:         1,
		TelegramChannelID: 888,
		AccessHash:        777,
		PhotoID:           555,
	}
	
	job := svc.EnqueueChannelAvatar(context.Background(), account, channel)
	if job.ID == "" {
		t.Fatal("EnqueueChannelAvatar() returned empty job ID")
	}
}
```

- [ ] **Step 10: Run test to verify it fails**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/avatar -v -run TestEnqueueChannelAvatar`
Expected: FAIL with "undefined: Service.EnqueueChannelAvatar"

- [ ] **Step 11: Implement EnqueueChannelAvatar method**

```go
// EnqueueChannelAvatar enqueues a channel avatar download task.
// Skips if PhotoID is 0 or file already exists.
func (s *Service) EnqueueChannelAvatar(ctx context.Context, account model.Account, channel model.Channel) scheduler.RetryJob {
	if channel.PhotoID <= 0 {
		return scheduler.RetryJob{}
	}
	path := AvatarAbsolutePath(s.storageRoot, "channel", channel.ID, channel.PhotoID)
	if FileExists(path) {
		return scheduler.RetryJob{}
	}
	if s.queue == nil {
		return scheduler.RetryJob{}
	}
	
	name := fmt.Sprintf("avatar-channel-%d-%d", channel.ID, channel.PhotoID)
	return s.queue.Enqueue(ctx, name, func(ctx context.Context) error {
		return s.downloadChannelAvatar(ctx, account, channel, path)
	})
}

func (s *Service) downloadChannelAvatar(ctx context.Context, account model.Account, channel model.Channel, destPath string) error {
	if FileExists(destPath) {
		return nil
	}
	
	session := telegram.AccountSession{
		AccountID:   account.ID,
		Phone:       account.Phone,
		SessionPath: account.SessionPath,
	}
	
	downloadFn := func() error {
		img, err := s.telegram.DownloadChannelAvatar(ctx, session, channel.TelegramChannelID, channel.AccessHash, channel.PhotoID)
		if err != nil {
			return fmt.Errorf("download channel avatar: %w", err)
		}
		if err := WriteAvatarFile(destPath, img.Data); err != nil {
			return fmt.Errorf("write avatar file: %w", err)
		}
		return nil
	}
	
	if s.limiter != nil {
		return s.limiter.Run(ctx, downloadFn)
	}
	return downloadFn()
}
```

- [ ] **Step 12: Run test to verify it passes**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/avatar -v -run TestEnqueueChannelAvatar`
Expected: PASS

- [ ] **Step 13: Write failing test for batch channel avatar enqueue**

```go
func TestEnqueueChannelAvatars(t *testing.T) {
	tmpDir := t.TempDir()
	queue := scheduler.NewRetryQueue(scheduler.RetryQueueOptions{
		Policy: retry.DefaultPolicy(),
		Logger: zap.NewNop(),
	})
	limiter := medialimit.New(5)
	mock := &mockTelegramChannel{}
	
	svc := NewService(ServiceOptions{
		StorageRoot: tmpDir,
		Queue:       queue,
		Limiter:     limiter,
		Telegram:    mock,
		Logger:      zap.NewNop(),
	})
	
	account := model.Account{ID: 1, Phone: "+1234567890"}
	channels := []model.Channel{
		{ID: 2, AccountID: 1, TelegramChannelID: 888, AccessHash: 777, PhotoID: 555},
		{ID: 3, AccountID: 1, TelegramChannelID: 999, AccessHash: 666, PhotoID: 444},
		{ID: 4, AccountID: 1, TelegramChannelID: 111, AccessHash: 222, PhotoID: 0}, // no photo
	}
	
	jobs := svc.EnqueueChannelAvatars(context.Background(), account, channels)
	if len(jobs) != 2 {
		t.Errorf("EnqueueChannelAvatars() returned %d jobs, want 2", len(jobs))
	}
}
```

- [ ] **Step 14: Run test to verify it fails**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/avatar -v -run TestEnqueueChannelAvatars`
Expected: FAIL with "undefined: Service.EnqueueChannelAvatars"

- [ ] **Step 15: Implement EnqueueChannelAvatars batch method**

```go
// EnqueueChannelAvatars enqueues avatar download tasks for multiple channels.
func (s *Service) EnqueueChannelAvatars(ctx context.Context, account model.Account, channels []model.Channel) []scheduler.RetryJob {
	var jobs []scheduler.RetryJob
	for _, channel := range channels {
		job := s.EnqueueChannelAvatar(ctx, account, channel)
		if job.ID != "" {
			jobs = append(jobs, job)
		}
	}
	return jobs
}
```

- [ ] **Step 16: Run test to verify it passes**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/avatar -v -run TestEnqueueChannelAvatars`
Expected: PASS

- [ ] **Step 17: Run all service tests**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/avatar -v`
Expected: PASS

- [ ] **Step 18: Commit service layer**

```bash
git add internal/avatar/service.go internal/avatar/service_test.go
git commit -m "feat: add avatar download service with queue integration

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Hook Avatar Downloads After Login

**Files:**
- Modify: `internal/api/handlers.go:2899-2918` (updateAccountProfile)

- [ ] **Step 1: Add avatar service to Dependencies struct**

In `internal/api/router.go:35-79`, add after line 45 (AvatarCache):

```go
AvatarService    *avatar.Service
```

- [ ] **Step 2: Update handlers.go to trigger avatar download after login**

In `internal/api/handlers.go`, add import at top:

```go
import (
	// ... existing imports
	avatarpkg "tg-search/internal/avatar"
)
```

Then modify `updateAccountProfile` function (line 2899):

```go
func (h handlers) updateAccountProfile(c *gin.Context, account model.Account, profile telegram.Profile) {
	account.TelegramUserID = profile.TelegramUserID
	if profile.Phone != "" {
		account.Phone = profile.Phone
	}
	account.FirstName = profile.FirstName
	account.LastName = profile.LastName
	account.Username = profile.Username
	account.PhotoID = profile.PhotoID
	account.Status = model.AccountStatusOnline
	account.SessionPath = h.sessionPath(account.ID)
	now := time.Now().UTC()
	account.LastOnlineAt = &now
	account.LastError = ""
	if err := h.deps.Accounts.Update(c.Request.Context(), account); err != nil {
		errorJSON(c, http.StatusInternalServerError, err)
		return
	}
	
	// Trigger avatar download in background
	if h.deps.AvatarService != nil && account.PhotoID > 0 {
		h.deps.AvatarService.EnqueueAccountAvatar(context.WithoutCancel(c.Request.Context()), account)
	}
	
	h.respondWithOnlineAccount(c, account)
}
```

- [ ] **Step 3: Hook avatar downloads after channel sync**

In `internal/api/handlers.go`, modify the `respondWithOnlineAccount` function around line 2975 (inside the success path of metadata sync):

Find this block:
```go
if h.deps.ChannelWebAccess != nil {
	channelIDs := make([]int64, 0, len(items))
	for _, item := range items {
		channelIDs = append(channelIDs, item.ID)
	}
	if len(channelIDs) > 0 {
		if _, err := h.deps.ChannelWebAccess.CheckMany(ctx, channelIDs); err != nil {
			return err
		}
	}
}
return nil
```

Replace with:
```go
if h.deps.ChannelWebAccess != nil {
	channelIDs := make([]int64, 0, len(items))
	for _, item := range items {
		channelIDs = append(channelIDs, item.ID)
	}
	if len(channelIDs) > 0 {
		if _, err := h.deps.ChannelWebAccess.CheckMany(ctx, channelIDs); err != nil {
			return err
		}
	}
}
// Trigger channel avatar downloads in background
if h.deps.AvatarService != nil {
	h.deps.AvatarService.EnqueueChannelAvatars(ctx, accountForSync, items)
}
return nil
```

- [ ] **Step 4: Build to verify no compilation errors**

Run: `go build -o /tmp/tg-search ./cmd/tg-search`
Expected: SUCCESS

- [ ] **Step 5: Commit login and sync hooks**

```bash
git add internal/api/handlers.go internal/api/router.go
git commit -m "feat: trigger avatar downloads after login and channel sync

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

### Task 4: Modify Avatar API Endpoints to Use Local Files

**Files:**
- Modify: `internal/api/account_avatar.go:16-91`
- Modify: `internal/api/channel_avatar.go:19-101`

- [ ] **Step 1: Update account avatar endpoint to check local file first**

In `internal/api/account_avatar.go`, replace the `serveAccountAvatar` function (lines 16-91):

```go
func (h handlers) serveAccountAvatar(c *gin.Context) {
	if h.deps.Telegram == nil {
		errorText(c, http.StatusServiceUnavailable, "telegram client is unavailable")
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		errorText(c, http.StatusBadRequest, "id must be a positive integer")
		return
	}
	account, err := h.deps.Accounts.FindByID(c.Request.Context(), id)
	if err != nil {
		errorText(c, http.StatusNotFound, "account not found")
		return
	}
	if account.PhotoID <= 0 {
		errorText(c, http.StatusNotFound, "account has no avatar")
		return
	}

	// Set ETag based on photo ID for efficient browser caching
	etag := fmt.Sprintf(`"acc-%d-%d"`, account.ID, account.PhotoID)
	c.Header("ETag", etag)

	// Check If-None-Match header for 304 Not Modified response
	if match := c.GetHeader("If-None-Match"); match == etag {
		c.Status(http.StatusNotModified)
		return
	}

	// Check local file first
	if h.deps.RuntimeConfig.Storage.Path != "" {
		localPath := avatarpkg.AvatarAbsolutePath(h.deps.RuntimeConfig.Storage.Path, "account", account.ID, account.PhotoID)
		if avatarpkg.FileExists(localPath) {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
			c.File(localPath)
			return
		}
	}

	// File not found locally, return 404
	errorText(c, http.StatusNotFound, "avatar not downloaded yet")
}
```

Add import at top:

```go
import (
	// ... existing imports
	avatarpkg "tg-search/internal/avatar"
)
```

- [ ] **Step 2: Update channel avatar endpoint to check local file first**

In `internal/api/channel_avatar.go`, replace the `serveChannelAvatar` function (lines 19-101):

```go
func (h handlers) serveChannelAvatar(c *gin.Context) {
	if h.deps.Telegram == nil {
		errorText(c, http.StatusServiceUnavailable, "telegram client is unavailable")
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		errorText(c, http.StatusBadRequest, "id must be a positive integer")
		return
	}
	channel, err := h.deps.Channels.FindByID(c.Request.Context(), id)
	if err != nil {
		errorText(c, http.StatusNotFound, "channel not found")
		return
	}
	if channel.PhotoID <= 0 {
		errorText(c, http.StatusNotFound, "channel has no avatar")
		return
	}

	// Set ETag based on photo ID for efficient browser caching
	etag := fmt.Sprintf(`"ch-%d-%d"`, channel.ID, channel.PhotoID)
	c.Header("ETag", etag)

	// Check If-None-Match header for 304 Not Modified response
	if match := c.GetHeader("If-None-Match"); match == etag {
		c.Status(http.StatusNotModified)
		return
	}

	// Check local file first
	if h.deps.RuntimeConfig.Storage.Path != "" {
		localPath := avatarpkg.AvatarAbsolutePath(h.deps.RuntimeConfig.Storage.Path, "channel", channel.ID, channel.PhotoID)
		if avatarpkg.FileExists(localPath) {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
			c.File(localPath)
			return
		}
	}

	// File not found locally, return 404
	errorText(c, http.StatusNotFound, "avatar not downloaded yet")
}
```

Add import at top:

```go
import (
	// ... existing imports
	avatarpkg "tg-search/internal/avatar"
)
```

- [ ] **Step 3: Remove unused helper functions from channel_avatar.go**

In `internal/api/channel_avatar.go`, delete lines 103-151 (serveAvatarData, channelAvatarCacheKey, avatarCacheGet, avatarCacheSet, downloadAvatar helper functions - no longer needed).

- [ ] **Step 4: Remove unused helper function from account_avatar.go**

In `internal/api/account_avatar.go`, delete lines 93-96 (accountAvatarCacheKey helper function - no longer needed).

- [ ] **Step 5: Build to verify no compilation errors**

Run: `go build -o /tmp/tg-search ./cmd/tg-search`
Expected: SUCCESS

- [ ] **Step 6: Commit avatar endpoint changes**

```bash
git add internal/api/account_avatar.go internal/api/channel_avatar.go
git commit -m "feat: update avatar endpoints to serve local files only

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Add Manual Sync API Endpoint

**Files:**
- Modify: `internal/api/handlers.go` (add new handler)
- Modify: `internal/api/router.go` (register route)

- [ ] **Step 1: Add manual sync handler to handlers.go**

In `internal/api/handlers.go`, add after the `password` function (around line 1320):

```go
func (h handlers) syncAccountAvatar(c *gin.Context) {
	if h.deps.AvatarService == nil {
		errorText(c, http.StatusServiceUnavailable, "avatar service is unavailable")
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		errorText(c, http.StatusBadRequest, "id must be a positive integer")
		return
	}
	account, err := h.deps.Accounts.FindByID(c.Request.Context(), id)
	if err != nil {
		errorText(c, http.StatusNotFound, "account not found")
		return
	}
	if account.PhotoID <= 0 {
		errorText(c, http.StatusBadRequest, "account has no avatar to sync")
		return
	}
	
	job := h.deps.AvatarService.EnqueueAccountAvatar(context.WithoutCancel(c.Request.Context()), account)
	c.JSON(http.StatusAccepted, gin.H{"status": "queued", "job_id": job.ID})
}
```

- [ ] **Step 2: Register route in router.go**

In `internal/api/router.go`, find the admin authenticated routes section (around line 140), add after account routes:

```go
admin.POST("/accounts/:id/sync-avatar", h.syncAccountAvatar)
```

- [ ] **Step 3: Build to verify no compilation errors**

Run: `go build -o /tmp/tg-search ./cmd/tg-search`
Expected: SUCCESS

- [ ] **Step 4: Commit manual sync API**

```bash
git add internal/api/handlers.go internal/api/router.go
git commit -m "feat: add manual account avatar sync API endpoint

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: Initialize Avatar Service in main.go

**Files:**
- Modify: `cmd/tg-search/main.go:138-296`

- [ ] **Step 1: Add avatar service initialization**

In `cmd/tg-search/main.go`, add import at top:

```go
import (
	// ... existing imports
	avatarpkg "tg-search/internal/avatar"
)
```

After line 140 (after syncQueue initialization), add:

```go
avatarQueue := scheduler.NewRetryQueue(scheduler.RetryQueueOptions{Policy: retryPolicy, Logger: logs.App})
avatarService := avatarpkg.NewService(avatarpkg.ServiceOptions{
	StorageRoot: cfg.Storage.Path,
	Queue:       avatarQueue,
	Limiter:     avatarLimiter,
	Telegram:    tgClient,
	Logger:      logs.App,
})
```

- [ ] **Step 2: Add AvatarService to Dependencies**

In `cmd/tg-search/main.go`, find the `api.NewRouter` call (around line 288), add after ImageCache line:

```go
AvatarService: avatarService,
```

- [ ] **Step 3: Build to verify integration**

Run: `go build -o /tmp/tg-search ./cmd/tg-search`
Expected: SUCCESS

- [ ] **Step 4: Run Go tests**

Run: `GOCACHE=/tmp/go-build-cache go test ./...`
Expected: PASS

- [ ] **Step 5: Commit main.go integration**

```bash
git add cmd/tg-search/main.go
git commit -m "feat: initialize avatar service in main

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: Add Frontend Avatar Display and Sync Button

**Files:**
- Modify: `web/src/views/AccountsView.vue`
- Modify: `web/src/api/admin.ts`

- [ ] **Step 1: Add syncAccountAvatar API call**

In `web/src/api/admin.ts`, add after existing account functions:

```typescript
export async function syncAccountAvatar(accountId: number): Promise<{ status: string; job_id?: string }> {
  const response = await fetch(`/api/admin/accounts/${accountId}/sync-avatar`, {
    method: 'POST',
    credentials: 'include'
  })
  if (!response.ok) {
    throw new Error(await response.text())
  }
  return response.json()
}
```

- [ ] **Step 2: Add avatar image and sync button to AccountsView.vue**

In `web/src/views/AccountsView.vue`, find the `<script setup>` section and add import:

```typescript
import { syncAccountAvatar } from '@/api/admin'
```

Add reactive state for syncing accounts:

```typescript
const syncingAvatarIds = ref(new Set<number>())
```

Add handler function:

```typescript
async function handleSyncAvatar(account: TelegramAccount) {
  if (syncingAvatarIds.value.has(account.id)) return
  syncingAvatarIds.value.add(account.id)
  try {
    await syncAccountAvatar(account.id)
    window.$message?.success('头像同步已排队')
  } catch (err) {
    window.$message?.error(`同步失败: ${err}`)
  } finally {
    syncingAvatarIds.value.delete(account.id)
  }
}

function avatarUrl(account: TelegramAccount): string | null {
  if (!account.photo_id || account.photo_id <= 0) return null
  return `/api/admin/accounts/${account.id}/avatar?t=${account.photo_id}`
}
```

- [ ] **Step 3: Update template to display avatar and sync button**

In `web/src/views/AccountsView.vue`, find the account row template in the desktop table view (look for the phone column), replace the placeholder span with:

```vue
<td class="phone-cell">
  <div class="account-row">
    <img 
      v-if="avatarUrl(account)" 
      :src="avatarUrl(account)" 
      class="account-avatar"
      loading="lazy"
      @error="$event.target.style.display = 'none'"
    />
    <span v-else class="account-avatar-placeholder">
      {{ account.first_name ? account.first_name.charAt(0).toUpperCase() : '?' }}
    </span>
    <div class="account-info">
      <span>{{ account.phone }}</span>
    </div>
  </div>
</td>
```

Add sync button in the actions column:

```vue
<n-button 
  v-if="account.photo_id > 0"
  size="small" 
  :loading="syncingAvatarIds.has(account.id)" 
  @click="handleSyncAvatar(account)"
>
  同步头像
</n-button>
```

- [ ] **Step 4: Add CSS styles for avatar display**

In `web/src/views/AccountsView.vue`, add to the `<style scoped>` section:

```css
.account-row {
  align-items: center;
  display: flex;
  gap: 10px;
}

.account-avatar {
  border-radius: 50%;
  flex-shrink: 0;
  height: 32px;
  object-fit: cover;
  width: 32px;
}

.account-avatar-placeholder {
  align-items: center;
  background: var(--app-border-strong, #e0e0e0);
  border-radius: 50%;
  color: var(--app-muted, #888);
  display: flex;
  flex-shrink: 0;
  font-size: 14px;
  font-weight: 600;
  height: 32px;
  justify-content: center;
  width: 32px;
}

.account-info {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
```

- [ ] **Step 5: Run frontend type check**

Run: `npm run web:typecheck`
Expected: SUCCESS with no errors

- [ ] **Step 6: Run frontend tests**

Run: `npm run web:test`
Expected: PASS

- [ ] **Step 7: Build frontend**

Run: `npm run web:build`
Expected: SUCCESS

- [ ] **Step 8: Commit frontend changes**

```bash
git add web/src/views/AccountsView.vue web/src/api/admin.ts
git commit -m "feat: add account avatar display and manual sync button

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

### Task 8: Add Channel Avatar Display (No Sync Button)

**Files:**
- Modify: `web/src/views/ChannelsView.vue`

- [ ] **Step 1: Add avatar URL helper function**

In `web/src/views/ChannelsView.vue`, find the `<script setup>` section and add:

```typescript
function channelAvatarUrl(channel: TelegramChannel): string | null {
  if (!channel.photo_id || channel.photo_id <= 0) return null
  return `/api/admin/channels/${channel.id}/avatar?t=${channel.photo_id}`
}
```

- [ ] **Step 2: Update desktop table view to display channel avatars**

In `web/src/views/ChannelsView.vue`, find the desktop table row with the title cell (look for `class="title-cell"`), update the avatar section:

```vue
<img 
  v-if="channelAvatarUrl(channel)" 
  :src="channelAvatarUrl(channel)" 
  class="channel-avatar"
  loading="lazy"
  @error="$event.target.style.display = 'none'"
/>
<span v-else class="channel-avatar-placeholder">
  {{ channel.title.charAt(0).toUpperCase() }}
</span>
```

- [ ] **Step 3: Update mobile card view to display channel avatars**

Find the mobile card header section (look for `class="mobile-card-header"`), update:

```vue
<img 
  v-if="channelAvatarUrl(channel)" 
  :src="channelAvatarUrl(channel)" 
  class="channel-avatar"
  loading="lazy"
  @error="$event.target.style.display = 'none'"
/>
<span v-else class="channel-avatar-placeholder">
  {{ channel.title.charAt(0).toUpperCase() }}
</span>
```

- [ ] **Step 4: Update CSS to add .channel-avatar class**

In the `<style scoped>` section, add after existing .channel-avatar-placeholder:

```css
.channel-avatar {
  border-radius: 50%;
  flex-shrink: 0;
  height: 32px;
  object-fit: cover;
  width: 32px;
}
```

- [ ] **Step 5: Run frontend type check**

Run: `npm run web:typecheck`
Expected: SUCCESS with no errors

- [ ] **Step 6: Run frontend tests**

Run: `npm run web:test`
Expected: PASS

- [ ] **Step 7: Build frontend**

Run: `npm run web:build`
Expected: SUCCESS

- [ ] **Step 8: Commit channel avatar display**

```bash
git add web/src/views/ChannelsView.vue
git commit -m "feat: add channel avatar display in channels view

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 9: Manual Testing and Verification

**Files:**
- None (manual testing only)

- [ ] **Step 1: Start the application**

Run: `go build -o /tmp/tg-search ./cmd/tg-search && /tmp/tg-search`
Expected: Server starts without errors

- [ ] **Step 2: Test account login triggers avatar download**

1. Open browser to admin interface
2. Login to admin account
3. Add a new Telegram account (send code + sign in)
4. Check server logs for "avatar-account-" queue message
5. Wait 5-10 seconds
6. Verify file exists: `ls -la data/tg-search/avatars/account/<account-id>/`
7. Refresh page and verify avatar image displays

- [ ] **Step 3: Test channel sync triggers batch avatar downloads**

1. Click "Sync Channels" on the account
2. Check server logs for multiple "avatar-channel-" queue messages
3. Wait 10-30 seconds (depending on channel count)
4. Verify files exist: `ls -la data/tg-search/avatars/channel/*/`
5. Navigate to Channels view
6. Verify channel avatars display for channels with photos

- [ ] **Step 4: Test manual sync button**

1. Navigate to Accounts view
2. Find an account with `photo_id > 0`
3. Delete the avatar file: `rm data/tg-search/avatars/account/<id>/<photo_id>.jpg`
4. Refresh page - avatar should show placeholder
5. Click "同步头像" button
6. Wait 5 seconds
7. Refresh page - avatar should display again

- [ ] **Step 5: Test browser caching with ETag**

1. Open browser DevTools (Network tab)
2. Load Accounts view with avatars
3. Note the avatar requests return 200 OK
4. Refresh the page (Ctrl+R or Cmd+R)
5. Verify avatar requests return 304 Not Modified
6. Check response headers contain ETag

- [ ] **Step 6: Test 404 for missing avatars**

1. Find an account with `photo_id > 0` but no local file
2. Open browser DevTools
3. Load Accounts view
4. Verify avatar request returns 404
5. Verify placeholder displays (first letter circle)

- [ ] **Step 7: Test PhotoID = 0 accounts**

1. Find or create an account with `photo_id = 0`
2. Verify no avatar image is requested (check DevTools Network tab)
3. Verify placeholder displays
4. Verify "同步头像" button does NOT display for this account

---

### Task 10: Final Integration and Cleanup

**Files:**
- None (verification only)

- [ ] **Step 1: Run full test suite**

Run: `GOCACHE=/tmp/go-build-cache go test ./...`
Expected: All tests PASS

- [ ] **Step 2: Run frontend tests**

Run: `npm run web:test`
Expected: All tests PASS

- [ ] **Step 3: Build final artifacts**

Run: `npm run web:build && go build -o /tmp/tg-search ./cmd/tg-search`
Expected: SUCCESS

- [ ] **Step 4: Verify no console errors or warnings**

1. Start application
2. Open browser DevTools Console
3. Navigate through Accounts and Channels views
4. Verify no JavaScript errors or warnings
5. Verify no 500 server errors in Network tab

- [ ] **Step 5: Check avatar directory structure**

Run: `tree data/tg-search/avatars/ -L 3` (or `find data/tg-search/avatars/`)
Expected output structure:
```
avatars/
├── account/
│   └── 123/
│       └── 456789.jpg
└── channel/
    ├── 2/
    │   └── 111222.jpg
    └── 3/
        └── 333444.jpg
```

- [ ] **Step 6: Final commit**

```bash
git add -A
git status
# Verify all changes are committed
git log --oneline -10
```

---

## Self-Review Checklist

**Spec coverage check:**
- ✅ Avatar storage layer with file path helpers (Task 1)
- ✅ Avatar download service with retry queue (Task 2)
- ✅ Account login triggers avatar download (Task 3)
- ✅ Channel sync triggers batch avatar downloads (Task 3)
- ✅ API endpoints check local files first, return 404 if missing (Task 4)
- ✅ Manual sync button for accounts (Tasks 5, 7)
- ✅ No manual sync for channels (per spec) (Task 8)
- ✅ Frontend displays avatars with placeholder fallback (Tasks 7, 8)
- ✅ Infinite cache via immutable Cache-Control header (Task 4)
- ✅ ETag support for 304 responses (Task 4)

**No placeholders - all code shown:**
- ✅ Complete storage layer implementation with tests
- ✅ Complete service layer with enqueue methods
- ✅ Complete API handler modifications
- ✅ Complete frontend components with TypeScript
- ✅ All import statements included
- ✅ All test assertions included

**Type consistency:**
- ✅ `AvatarPath()` function signature consistent across all uses
- ✅ `scheduler.RetryJob` return type consistent
- ✅ `model.Account` and `model.Channel` fields match existing schema
- ✅ Frontend API types match backend responses

## Summary

This plan implements async avatar downloads with:
- Background queue using existing `scheduler.RetryQueue`
- Auto-trigger on login and channel sync
- Manual sync button for accounts only
- Local file storage with infinite cache
- 404 fallback to placeholder display
