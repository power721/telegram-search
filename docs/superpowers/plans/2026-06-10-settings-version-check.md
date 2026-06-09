# Settings Version Check Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a settings-page version panel that displays the running version and checks GitHub Releases for updates.

**Architecture:** The backend exposes `GET /api/settings/version`, using `internal/build.Version` for the current version and a small HTTP client helper for GitHub Releases. The frontend keeps version state local to `SettingsView.vue` and reuses the existing `apiGet` helper.

**Tech Stack:** Go, Gin, standard `net/http`, Vue 3, TypeScript, Naive UI, Vitest.

---

### Task 1: Backend Version API

**Files:**
- Create: `internal/api/version.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/handlers_test.go`

- [ ] **Step 1: Write failing backend tests**

Add tests to `internal/api/handlers_test.go`:

```go
func TestVersionSettingsReportsGitHubRelease(t *testing.T) {
	originalVersion := build.Version
	originalURL := githubLatestReleaseURL
	originalClient := githubHTTPClient
	defer func() {
		build.Version = originalVersion
		githubLatestReleaseURL = originalURL
		githubHTTPClient = originalClient
	}()

	build.Version = "v1.2.3"
	githubLatestReleaseURL = "https://api.github.test/repos/power721/tg-search/releases/latest"
	githubHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != githubLatestReleaseURL {
			t.Fatalf("unexpected GitHub URL: %s", req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"tag_name":"v1.2.4","html_url":"https://github.com/power721/tg-search/releases/tag/v1.2.4"}`)),
		}, nil
	})}

	router := NewRouter(testDeps(t))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/settings/version?check_update=true", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var body model.VersionInfoResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.CurrentVersion != "v1.2.3" || body.LatestVersion != "v1.2.4" || !body.UpdateAvailable {
		t.Fatalf("version response = %+v", body)
	}
	if body.LatestURL != "https://github.com/power721/tg-search/releases/tag/v1.2.4" {
		t.Fatalf("latest url = %q", body.LatestURL)
	}
}

func TestVersionSettingsDoesNotClaimUpdateForDevVersion(t *testing.T) {
	originalVersion := build.Version
	originalURL := githubLatestReleaseURL
	originalClient := githubHTTPClient
	defer func() {
		build.Version = originalVersion
		githubLatestReleaseURL = originalURL
		githubHTTPClient = originalClient
	}()

	build.Version = "dev"
	githubLatestReleaseURL = "https://api.github.test/repos/power721/tg-search/releases/latest"
	githubHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"tag_name":"v9.9.9","html_url":"https://github.com/power721/tg-search/releases/tag/v9.9.9"}`)),
		}, nil
	})}

	router := NewRouter(testDeps(t))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/settings/version?check_update=true", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var body model.VersionInfoResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.CurrentVersion != "dev" || body.LatestVersion != "v9.9.9" || body.UpdateAvailable {
		t.Fatalf("version response = %+v", body)
	}
}
```

- [ ] **Step 2: Run backend tests and verify they fail**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/api -run 'TestVersionSettings'`

Expected: FAIL because `githubLatestReleaseURL`, `build`, and `model.VersionInfoResponse` are not defined/imported for these tests yet.

- [ ] **Step 3: Implement backend types and handler**

Create `internal/api/version.go` with:

```go
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tg-search/internal/build"
	"tg-search/internal/model"
)

var githubLatestReleaseURL = "https://api.github.com/repos/power721/tg-search/releases/latest"
var githubHTTPClient = http.DefaultClient

func (h handlers) getVersionSettings(c *gin.Context) {
	info, err := loadVersionInfo(c.Request.Context(), githubHTTPClient, shouldCheckUpdate(c.Query("check_update")))
	if err != nil {
		errorJSON(c, http.StatusBadGateway, err)
		return
	}
	c.JSON(http.StatusOK, info)
}

func loadVersionInfo(ctx context.Context, client *http.Client, checkUpdate bool) (model.VersionInfoResponse, error) {
	current := strings.TrimSpace(build.Version)
	if current == "" {
		current = "dev"
	}
	if !checkUpdate {
		return model.VersionInfoResponse{CurrentVersion: current}, nil
	}
	latest, err := fetchLatestGitHubRelease(ctx, client)
	if err != nil {
		return model.VersionInfoResponse{CurrentVersion: current}, err
	}
	return model.VersionInfoResponse{
		CurrentVersion:  current,
		LatestVersion:   latest.TagName,
		LatestURL:       latest.HTMLURL,
		UpdateAvailable: newerSemver(latest.TagName, current),
	}, nil
}

func shouldCheckUpdate(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "1" || value == "true" || value == "yes"
}

type githubReleaseResponse struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

func fetchLatestGitHubRelease(ctx context.Context, client *http.Client) (githubReleaseResponse, error) {
	requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, githubLatestReleaseURL, nil)
	if err != nil {
		return githubReleaseResponse{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "tg-search")
	resp, err := client.Do(req)
	if err != nil {
		return githubReleaseResponse{}, fmt.Errorf("check GitHub release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return githubReleaseResponse{}, fmt.Errorf("check GitHub release: status %d", resp.StatusCode)
	}
	var latest githubReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&latest); err != nil {
		return githubReleaseResponse{}, fmt.Errorf("decode GitHub release: %w", err)
	}
	if strings.TrimSpace(latest.TagName) == "" {
		return githubReleaseResponse{}, fmt.Errorf("GitHub release tag_name is empty")
	}
	return latest, nil
}

func newerSemver(latest string, current string) bool {
	latestParts, ok := parseSemverParts(latest)
	if !ok {
		return false
	}
	currentParts, ok := parseSemverParts(current)
	if !ok {
		return false
	}
	for i := range latestParts {
		if latestParts[i] > currentParts[i] {
			return true
		}
		if latestParts[i] < currentParts[i] {
			return false
		}
	}
	return false
}

func parseSemverParts(value string) ([3]int, bool) {
	value = strings.TrimSpace(strings.TrimPrefix(value, "v"))
	base, _, _ := strings.Cut(value, "-")
	segments := strings.Split(base, ".")
	if len(segments) != 3 {
		return [3]int{}, false
	}
	var parts [3]int
	for i, segment := range segments {
		n, err := strconv.Atoi(segment)
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		parts[i] = n
	}
	return parts, true
}
```

Add `api.GET("/settings/version", h.getVersionSettings)` in `internal/api/router.go`. The handler should call GitHub only when `check_update=true`; otherwise it should return the current version immediately.

Add this model type to `internal/model/model.go`:

```go
type VersionInfoResponse struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version,omitempty"`
	LatestURL       string `json:"latest_url,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
}
```

- [ ] **Step 4: Run backend tests and verify they pass**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/api -run 'TestVersionSettings'`

Expected: PASS.

### Task 2: Settings Page Version Panel

**Files:**
- Modify: `web/src/api/types.ts`
- Modify: `web/src/views/SettingsView.test.ts`
- Modify: `web/src/views/SettingsView.vue`

- [ ] **Step 1: Write failing frontend tests**

Update the `apiGet` mock in `web/src/views/SettingsView.test.ts` to return a current-version response for `/api/settings/version` and a latest-release response for `/api/settings/version?check_update=true`. Add a test that mounts the settings view, verifies the current version is shown, clicks the version check button, and verifies the update-check URL was called.

- [ ] **Step 2: Run frontend test and verify it fails**

Run: `npm run web:test -- --run web/src/views/SettingsView.test.ts`

Expected: FAIL until `SettingsView.vue` renders the version panel.

- [ ] **Step 3: Implement frontend type and panel**

Add `VersionInfoResponse` to `web/src/api/types.ts`, add local refs and `loadVersionInfo()` to `SettingsView.vue`, call `loadVersionInfo(false)` on mount, and render a compact panel with `data-testid="check-version"` and `data-testid="current-version"`.

- [ ] **Step 4: Run frontend test and verify it passes**

Run: `npm run web:test -- --run web/src/views/SettingsView.test.ts`

Expected: PASS.

### Task 3: Final Verification and Commit

**Files:**
- All files from Tasks 1 and 2.

- [ ] **Step 1: Run full backend tests**

Run: `GOCACHE=/tmp/go-build-cache go test ./...`

Expected: PASS.

- [ ] **Step 2: Run frontend typecheck**

Run: `npm run web:typecheck`

Expected: PASS.

- [ ] **Step 3: Run frontend tests**

Run: `npm run web:test`

Expected: PASS.

- [ ] **Step 4: Commit implementation**

Run:

```bash
git add internal/api/version.go internal/api/router.go internal/api/handlers_test.go internal/model/model.go web/src/api/types.ts web/src/views/SettingsView.vue web/src/views/SettingsView.test.ts docs/superpowers/plans/2026-06-10-settings-version-check.md
git commit -m "feat: add settings version check"
```

Expected: Commit succeeds on `feat/settings-version-check`.
