# API Key Resource Access Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restrict API keys to existing resource list/detail endpoints while requiring an administrator session for all other authenticated API routes.

**Architecture:** Split the current single business route group into an admin-only group and a resource-access group. Add an admin-session middleware, keep API key validation reusable for resource access, and route only `/api/resources`, `/api/resources/grouped`, and `/api/resources/:id` through the API-key-or-admin guard.

**Tech Stack:** Go, Gin, `net/http/httptest`, existing API key and admin session services.

---

### Task 1: Lock Down Auth Behavior With Failing Tests

**Files:**
- Modify: `internal/api/handlers_test.go`

- [ ] **Step 1: Replace the broad business auth test with scoped API key tests**

Replace `TestBusinessAPIAcceptsAdminSessionOrAPIKey` with:

```go
func TestAPIKeyOnlyAllowsResourceEndpoints(t *testing.T) {
	deps := testDeps(t)
	router := NewRouter(deps)
	key := createTestAPIKey(t, router)
	cookie := createAdminSession(t, router)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("resources without credentials code = %d body=%s, want 401", w.Code, w.Body.String())
	}

	for _, path := range []string{"/api/resources", "/api/resources/grouped"} {
		t.Run("api key "+path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("X-API-Key", key)
			router.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("%s with api key code = %d body=%s, want 200", path, w.Code, w.Body.String())
			}
		})

		t.Run("admin "+path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.AddCookie(cookie)
			router.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("%s with admin session code = %d body=%s, want 200", path, w.Code, w.Body.String())
			}
		})

		t.Run("invalid api key "+path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("X-API-Key", "invalid")
			router.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("%s with invalid api key code = %d body=%s, want 401", path, w.Code, w.Body.String())
			}
		})
	}
}

func TestAPIKeyAllowsResourceDetail(t *testing.T) {
	ctx := context.Background()
	deps := testDeps(t)
	accountID, _ := deps.Accounts.Save(ctx, model.Account{Phone: "+10000000000", Username: "main", Status: model.AccountStatusOnline})
	channelID, _ := deps.Channels.Save(ctx, model.Channel{AccountID: accountID, TelegramChannelID: 1, Title: "VIP", Type: model.ChannelTypeChannel})
	stored, err := deps.Messages.SaveBatch(ctx, []model.Message{{
		AccountID: accountID, ChannelID: channelID, TelegramMessageID: 1,
		Text: "ubuntu resources", RawJSON: "{}", Date: time.Now().UTC(),
	}})
	if err != nil {
		t.Fatalf("save messages: %v", err)
	}
	if _, err := deps.Links.SaveBatch(ctx, stored[0].ID, []model.Link{{Type: "url", Category: "http", URL: "https://example.com/ubuntu", Note: "ubuntu"}}); err != nil {
		t.Fatalf("save links: %v", err)
	}
	router := NewRouter(deps)
	key := createTestAPIKey(t, router)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources?q=ubuntu", nil)
	req.Header.Set("X-API-Key", key)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("resources code = %d body=%s, want 200", w.Code, w.Body.String())
	}
	var list resource.ListResult
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("invalid list JSON: %v", err)
	}
	if len(list.Items) == 0 {
		t.Fatalf("resources list has no items: %+v", list)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/resources/"+url.PathEscape(list.Items[0].ID), nil)
	req.Header.Set("Authorization", "Bearer "+key)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("resource detail code = %d body=%s, want 200", w.Code, w.Body.String())
	}
}

func TestAPIKeyCannotAccessAdminOnlyEndpoints(t *testing.T) {
	deps := testDeps(t)
	router := NewRouter(deps)
	key := createTestAPIKey(t, router)
	cookie := createAdminSession(t, router)

	for _, path := range []string{"/api/status", "/api/tasks", "/api/search/global?q=ubuntu"} {
		t.Run("api key denied "+path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("X-API-Key", key)
			router.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("%s with api key code = %d body=%s, want 401", path, w.Code, w.Body.String())
			}
		})

		t.Run("admin allowed "+path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.AddCookie(cookie)
			router.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("%s with admin session code = %d body=%s, want 200", path, w.Code, w.Body.String())
			}
		})
	}
}
```

- [ ] **Step 2: Confirm detail URL escaping import**

Confirm `net/url` is already present in the import block in `internal/api/handlers_test.go`. The new resource detail test uses `url.PathEscape(list.Items[0].ID)`.

- [ ] **Step 3: Run the targeted failing tests**

Run:

```bash
go test ./internal/api -run 'TestAPIKeyOnlyAllowsResourceEndpoints|TestAPIKeyAllowsResourceDetail|TestAPIKeyCannotAccessAdminOnlyEndpoints' -count=1
```

Expected: fail because `/api/status`, `/api/tasks`, and `/api/search/global` still accept API keys.

- [ ] **Step 4: Commit the failing tests**

```bash
git add internal/api/handlers_test.go
git commit -m "test: capture scoped api key access"
```

### Task 2: Split Admin and Resource Route Guards

**Files:**
- Modify: `internal/api/handlers.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Add explicit middleware functions**

In `internal/api/handlers.go`, replace `requireAPIKey` with these functions:

```go
func (h handlers) requireAdminSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.hasAdminSession(c) {
			c.Next()
			return
		}
		errorText(c, http.StatusUnauthorized, "not authenticated")
		c.Abort()
	}
}

func (h handlers) requireResourceAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.hasAdminSession(c) {
			c.Next()
			return
		}
		key := apiKeyFromRequest(c.Request)
		if key == "" {
			errorText(c, http.StatusUnauthorized, "api key is required")
			c.Abort()
			return
		}
		_, ok, err := h.deps.APIKeyService.Verify(c.Request.Context(), key)
		if err != nil {
			errorJSON(c, http.StatusInternalServerError, err)
			c.Abort()
			return
		}
		if !ok {
			errorText(c, http.StatusUnauthorized, "invalid api key")
			c.Abort()
			return
		}
		c.Next()
	}
}
```

- [ ] **Step 2: Replace the single business group with two route groups**

In `internal/api/router.go`, create:

```go
	adminOnly := api.Group("")
	adminOnly.Use(h.requireAdminSession())

	resourceAccess := api.Group("")
	resourceAccess.Use(h.requireResourceAccess())
```

Move only these routes to `resourceAccess`:

```go
	resourceAccess.GET("/resources/grouped", h.resourcesGrouped)
	resourceAccess.GET("/resources/:id", h.resource)
	resourceAccess.GET("/resources", h.resources)
```

Move all other current `business` routes to `adminOnly`, preserving existing paths and methods.

- [ ] **Step 3: Run the targeted tests**

Run:

```bash
go test ./internal/api -run 'TestAPIKeyOnlyAllowsResourceEndpoints|TestAPIKeyAllowsResourceDetail|TestAPIKeyCannotAccessAdminOnlyEndpoints' -count=1
```

Expected: pass.

- [ ] **Step 4: Run router and API package tests**

Run:

```bash
go test ./internal/api -count=1
```

Expected: pass.

- [ ] **Step 5: Commit the implementation**

```bash
git add internal/api/handlers.go internal/api/router.go
git commit -m "feat: restrict api keys to resource endpoints"
```

### Task 3: Update API Documentation and Full Verification

**Files:**
- Modify: `docs/api.md`

- [ ] **Step 1: Update the authentication section**

Replace the current authentication text with:

```markdown
## Authentication

Administrator browser/API routes require an authenticated admin session cookie created by `POST /api/auth/login`.

API keys are limited external credentials. They can access only:

- `GET /api/resources`
- `GET /api/resources/grouped`
- `GET /api/resources/:id`

Send API keys with one of:

```text
Authorization: Bearer <api-key>
X-API-Key: <api-key>
```

The `api_key` query parameter is also accepted for resource endpoints and future media proxy endpoints that cannot set headers.

Health, readiness, first-run setup, and admin login/session endpoints remain available without an API key where required for bootstrap. API key management requires an authenticated admin session.
```

- [ ] **Step 2: Run full backend tests**

Run:

```bash
go test ./...
```

Expected: pass.

- [ ] **Step 3: Run frontend tests for regression confidence**

Run:

```bash
npm --prefix web run test
```

Expected: pass. No frontend changes are expected, but this confirms the existing client tests still pass.

- [ ] **Step 4: Commit docs**

```bash
git add docs/api.md
git commit -m "docs: clarify scoped api key access"
```
