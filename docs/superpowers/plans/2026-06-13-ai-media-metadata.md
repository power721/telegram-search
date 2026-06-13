# AI Media Metadata Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build optional asynchronous OpenAI-compatible AI enhancement for media metadata on newly indexed cloud-drive links.

**Architecture:** Runtime settings store the provider config with key preservation and redacted API responses. Existing message/link persistence remains rule-based; after persistence, a `ai_media_metadata` sync task is queued per eligible message. The task worker loads the original message and all message links, sends one OpenAI-compatible chat completion request, validates JSON, maps outputs by `link_id` or `url`, updates media fields, and refreshes resource stats.

**Tech Stack:** Go 1.x, Gin, SQLite repositories, existing `sync_tasks` worker, Vue 3, TypeScript, Naive UI, Vitest.

---

## File Structure

- Modify `internal/config/config.go`: add AI config structs, defaults, validation.
- Modify `internal/config/runtime_settings.go`: include AI settings, add redacted runtime response and key-preserving merge helper.
- Modify `internal/repository/settings.go`: keep storing full runtime settings; no key redaction here.
- Modify `internal/api/handlers.go`: return redacted runtime settings, preserve AI key on update, add model list handler.
- Modify `internal/api/router.go`: add `GET /api/settings/ai/models` and dependency for AI model listing.
- Modify `internal/api/handlers_test.go`: settings/model endpoint coverage.
- Modify `internal/model/model.go`: add `TaskTypeAIMediaMetadata`.
- Modify `internal/task/payload.go`: add `AIMediaMetadataPayload`.
- Create `internal/ai/client.go`: OpenAI-compatible `/models` and `/chat/completions` client.
- Create `internal/ai/service.go`: task handler, prompt building, response mapping, link updates.
- Create `internal/ai/client_test.go` and `internal/ai/service_test.go`.
- Modify `internal/repository/message.go`: add `FindByID`.
- Modify `internal/repository/link.go`: add `ListByMessage` and `UpdateMediaMetadata`.
- Modify repository tests for new helper methods.
- Modify `internal/history/service.go` and `internal/update/processor.go`: enqueue AI task after link persistence.
- Modify their constructors/options to accept an AI task enqueue dependency.
- Modify `cmd/tg-search/main.go`: wire AI service into task worker and history/update enqueue paths.
- Modify `web/src/api/types.ts`: add AI runtime settings and model response types.
- Modify `web/src/views/SettingsView.vue`: add AI settings form and model fetch.
- Modify `web/src/views/SettingsView.test.ts`: frontend behavior tests.

---

### Task 1: Runtime AI Settings And API Surface

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/runtime_settings.go`
- Modify: `internal/api/handlers.go`
- Modify: `internal/api/router.go`
- Test: `internal/config/config_test.go`
- Test: `internal/api/handlers_test.go`

- [ ] **Step 1: Write failing config tests**

Add tests covering default disabled AI, enabled validation, and key-preserving runtime update:

```go
func TestRuntimeSettingsPreserveAIMediaMetadataAPIKey(t *testing.T) {
	defaults := Config{}
	applyDefaults(&defaults)
	existing := RuntimeSettingsFromConfig(defaults)
	existing.AI.MediaMetadata = AIMediaMetadataSettings{
		Enabled: true,
		BaseURL: "https://api.example.com/v1",
		APIKey: "stored-key",
		Model: "movie-model",
	}
	incoming := existing
	incoming.AI.MediaMetadata.APIKey = ""

	merged := PreserveRuntimeSecrets(incoming, existing)

	if merged.AI.MediaMetadata.APIKey != "stored-key" {
		t.Fatalf("api key = %q, want stored-key", merged.AI.MediaMetadata.APIKey)
	}
}

func TestApplyRuntimeSettingsRejectsEnabledAIMediaMetadataWithoutRequiredFields(t *testing.T) {
	cfg := defaultConfig()
	settings := RuntimeSettingsFromConfig(cfg)
	settings.AI.MediaMetadata.Enabled = true

	_, err := ApplyRuntimeSettings(cfg, settings)

	if err == nil || !strings.Contains(err.Error(), "ai.media_metadata.base_url") {
		t.Fatalf("ApplyRuntimeSettings error = %v, want base_url validation", err)
	}
}
```

- [ ] **Step 2: Run config tests and verify RED**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/config -run 'TestRuntimeSettingsPreserveAIMediaMetadataAPIKey|TestApplyRuntimeSettingsRejectsEnabledAIMediaMetadataWithoutRequiredFields' -v`

Expected: FAIL because `AIMediaMetadataSettings`, `PreserveRuntimeSecrets`, and `RuntimeSettings.AI` do not exist.

- [ ] **Step 3: Implement config/runtime settings**

Add structs and validation:

```go
type AIConfig struct {
	MediaMetadata AIMediaMetadataSettings `yaml:"media_metadata" json:"media_metadata"`
}

type AIMediaMetadataSettings struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	BaseURL string `yaml:"base_url" json:"base_url"`
	APIKey  string `yaml:"api_key" json:"api_key,omitempty"`
	Model   string `yaml:"model" json:"model"`
}
```

Add `AI AIConfig` to `config.Config` and `AI RuntimeAISettings` to `RuntimeSettings`. `RuntimeSettingsFromConfig` and `ApplyRuntimeSettings` must copy `AI`. `validate` must require `base_url`, `api_key`, and `model` only when `enabled` is true. `PreserveRuntimeSecrets(incoming, existing RuntimeSettings)` must preserve `AI.MediaMetadata.APIKey` when incoming key is empty.

- [ ] **Step 4: Run config tests and verify GREEN**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/config -run 'TestRuntimeSettingsPreserveAIMediaMetadataAPIKey|TestApplyRuntimeSettingsRejectsEnabledAIMediaMetadataWithoutRequiredFields' -v`

Expected: PASS.

- [ ] **Step 5: Write failing API runtime settings tests**

Add `TestRuntimeSettingsRedactsAIMediaMetadataAPIKeyAndPreservesExistingKey` in `internal/api/handlers_test.go`. It should:

1. Save runtime settings with enabled AI and `api_key:"secret-key"`.
2. GET `/api/settings/runtime`.
3. Assert response contains `api_key_set:true` and does not contain `secret-key`.
4. PUT runtime settings with `api_key:""` and changed model.
5. Assert persisted settings still have `APIKey == "secret-key"`.

- [ ] **Step 6: Run API test and verify RED**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/api -run TestRuntimeSettingsRedactsAIMediaMetadataAPIKeyAndPreservesExistingKey -v`

Expected: FAIL because runtime response is not redacted and update does not preserve the key.

- [ ] **Step 7: Implement API redaction and key preservation**

Add response structs in `internal/config/runtime_settings.go`:

```go
type RuntimeSettingsResponse struct {
	Sync     RuntimeSyncSettings     `json:"sync"`
	Storage  RuntimeStorageSettings  `json:"storage"`
	Telegram RuntimeTelegramSettings `json:"telegram"`
	AI       RuntimeAISettingsResponse `json:"ai"`
}

type RuntimeAISettingsResponse struct {
	MediaMetadata AIMediaMetadataSettingsResponse `json:"media_metadata"`
}

type AIMediaMetadataSettingsResponse struct {
	Enabled   bool   `json:"enabled"`
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	APIKeySet bool   `json:"api_key_set"`
}
```

`getRuntimeSettings` must return `config.RedactRuntimeSettings(settings)`. `updateRuntimeSettings` must load existing runtime settings first, call `config.PreserveRuntimeSecrets`, validate with `ApplyRuntimeSettings`, save the merged full settings, and return the redacted response.

- [ ] **Step 8: Run API test and verify GREEN**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/api -run TestRuntimeSettingsRedactsAIMediaMetadataAPIKeyAndPreservesExistingKey -v`

Expected: PASS.

- [ ] **Step 9: Commit Task 1**

```bash
git add internal/config/config.go internal/config/runtime_settings.go internal/config/config_test.go internal/api/handlers.go internal/api/router.go internal/api/handlers_test.go
git commit -m "feat: add ai runtime settings"
```

---

### Task 2: OpenAI-Compatible Client And Model List Endpoint

**Files:**
- Create: `internal/ai/client.go`
- Create: `internal/ai/client_test.go`
- Modify: `internal/api/handlers.go`
- Modify: `internal/api/router.go`
- Test: `internal/api/handlers_test.go`

- [ ] **Step 1: Write failing AI client tests**

Test model listing and chat JSON extraction with an `httptest.Server`:

```go
func TestClientListModelsUsesOpenAICompatibleEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %s, want /v1/models", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4.1-mini"},{"id":"qwen-plus"}]}`))
	}))
	defer server.Close()

	client := NewClient(ClientOptions{BaseURL: server.URL + "/v1", APIKey: "test-key", HTTPClient: server.Client()})
	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if !reflect.DeepEqual(models, []string{"gpt-4.1-mini", "qwen-plus"}) {
		t.Fatalf("models = %#v", models)
	}
}
```

- [ ] **Step 2: Run AI client tests and verify RED**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/ai -run TestClientListModelsUsesOpenAICompatibleEndpoint -v`

Expected: FAIL because `internal/ai` does not exist.

- [ ] **Step 3: Implement `internal/ai/client.go`**

Implement:

```go
type ClientOptions struct {
	BaseURL string
	APIKey string
	Model string
	HTTPClient *http.Client
}

type Client struct { /* baseURL, apiKey, model, httpClient */ }

func NewClient(opts ClientOptions) *Client
func (c *Client) ListModels(ctx context.Context) ([]string, error)
func (c *Client) Enhance(ctx context.Context, req EnhancementRequest) (EnhancementResponse, error)
```

Normalize trailing slashes, call `GET /models`, call `POST /chat/completions`, set `Authorization: Bearer <key>`, parse OpenAI-compatible responses, and return useful errors for non-2xx statuses.

- [ ] **Step 4: Run AI client tests and verify GREEN**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/ai -v`

Expected: PASS.

- [ ] **Step 5: Write failing API model endpoint test**

Add `TestAIModelsEndpointListsConfiguredProviderModels`. It saves runtime AI settings with base URL pointing at an `httptest.Server`, then calls authenticated `GET /api/settings/ai/models` and expects:

```json
{"items":["gpt-4.1-mini","qwen-plus"]}
```

- [ ] **Step 6: Run API model endpoint test and verify RED**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/api -run TestAIModelsEndpointListsConfiguredProviderModels -v`

Expected: FAIL with 404 because the endpoint is not registered.

- [ ] **Step 7: Implement model list handler**

Add route:

```go
adminOnly.GET("/settings/ai/models", h.aiModels)
```

`aiModels` loads runtime settings, validates AI base URL/key, uses `ai.NewClient`, calls `ListModels`, and returns `gin.H{"items": models}`. The handler must not include the API key in the response or logs.

- [ ] **Step 8: Run model endpoint test and verify GREEN**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/api -run TestAIModelsEndpointListsConfiguredProviderModels -v`

Expected: PASS.

- [ ] **Step 9: Commit Task 2**

```bash
git add internal/ai/client.go internal/ai/client_test.go internal/api/handlers.go internal/api/router.go internal/api/handlers_test.go
git commit -m "feat: add openai compatible ai client"
```

---

### Task 3: AI Metadata Task Handler And Repository Helpers

**Files:**
- Modify: `internal/model/model.go`
- Modify: `internal/task/payload.go`
- Modify: `internal/repository/message.go`
- Modify: `internal/repository/link.go`
- Create: `internal/ai/service.go`
- Create: `internal/ai/service_test.go`
- Test: `internal/repository/repository_test.go`

- [ ] **Step 1: Write failing repository helper tests**

Add a test that saves one message with two links, calls `messages.FindByID`, `links.ListByMessage`, then `links.UpdateMediaMetadata` for one link and verifies only that link changes.

- [ ] **Step 2: Run repository helper tests and verify RED**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/repository -run TestRepositoriesLoadMessageLinksAndUpdateMediaMetadata -v`

Expected: FAIL because helper methods do not exist.

- [ ] **Step 3: Implement repository helpers**

Add:

```go
func (r *MessageRepository) FindByID(ctx context.Context, id int64) (model.Message, error)
func (r *LinkRepository) ListByMessage(ctx context.Context, messageID int64) ([]model.Link, error)
func (r *LinkRepository) UpdateMediaMetadata(ctx context.Context, link model.Link) error
```

`FindByID` joins `telegram_message_contents`. `ListByMessage` orders by `id ASC`. `UpdateMediaMetadata` updates only media columns by `id`.

- [ ] **Step 4: Run repository helper tests and verify GREEN**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/repository -run TestRepositoriesLoadMessageLinksAndUpdateMediaMetadata -v`

Expected: PASS.

- [ ] **Step 5: Write failing AI service test for multiple links/media**

Create `internal/ai/service_test.go` with a fake enhancer returning two items for one message:

```go
func TestServiceEnhancesMultipleLinksInOneMessage(t *testing.T) {
	// Save message text containing two cloud-drive links.
	// Save two links with rule-derived media.
	// Configure runtime AI media metadata as enabled.
	// Run Service.RunMediaMetadataTask with payload {"message_id": stored.ID}.
	// Assert link 1 title/year and link 2 title/episode were updated independently.
}
```

- [ ] **Step 6: Run AI service test and verify RED**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/ai -run TestServiceEnhancesMultipleLinksInOneMessage -v`

Expected: FAIL because the service does not exist.

- [ ] **Step 7: Implement AI service**

Add:

```go
type Enhancer interface {
	Enhance(context.Context, EnhancementRequest) (EnhancementResponse, error)
}

type ServiceOptions struct {
	Settings *repository.SettingsRepository
	Defaults config.Config
	Messages *repository.MessageRepository
	Links *repository.LinkRepository
	Resources *resource.Service
	NewEnhancer func(config.AIMediaMetadataSettings) Enhancer
	Logger *zap.Logger
}

func (s *Service) RunMediaMetadataTask(ctx context.Context, item model.Task, progress task.ProgressSink) error
```

The handler must parse `task.AIMediaMetadataPayload`, load settings, skip cleanly when disabled or no cloud-drive links exist, build the request, call enhancer once, map output by `link_id` then `url`, overlay non-empty AI fields onto existing link media fields, call `UpdateMediaMetadata`, and refresh resources.

- [ ] **Step 8: Run AI service tests and verify GREEN**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/ai -v`

Expected: PASS.

- [ ] **Step 9: Commit Task 3**

```bash
git add internal/model/model.go internal/task/payload.go internal/repository/message.go internal/repository/link.go internal/repository/repository_test.go internal/ai/service.go internal/ai/service_test.go
git commit -m "feat: add ai media metadata task handler"
```

---

### Task 4: Enqueue AI Tasks After New Message Persistence

**Files:**
- Modify: `internal/history/service.go`
- Modify: `internal/history/service_test.go`
- Modify: `internal/update/processor.go`
- Modify: `internal/update/processor_test.go`
- Modify: `cmd/tg-search/main.go`

- [ ] **Step 1: Write failing history enqueue test**

Add a test that configures history service with AI enqueue enabled, stores a message with two cloud-drive links, and asserts exactly one `ai_media_metadata` task payload with the stored `message_id`.

- [ ] **Step 2: Run history enqueue test and verify RED**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/history -run TestHistoryStoreBatchEnqueuesAIMediaMetadataTaskForCloudLinks -v`

Expected: FAIL because no AI task is enqueued.

- [ ] **Step 3: Implement history enqueue dependency**

Add an option to `history.Options`:

```go
AIMediaMetadataTasks interface {
	Enqueue(context.Context, string, any) (model.Task, error)
}
```

After `ReplaceForMessageTx` succeeds and the transaction commits, enqueue `model.TaskTypeAIMediaMetadata` with `task.AIMediaMetadataPayload{MessageID: msg.ID}` for messages with at least one cloud-drive link.

- [ ] **Step 4: Run history enqueue test and verify GREEN**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/history -run TestHistoryStoreBatchEnqueuesAIMediaMetadataTaskForCloudLinks -v`

Expected: PASS.

- [ ] **Step 5: Write failing update enqueue test**

Add a processor test for a realtime new message with multiple cloud-drive links. Assert one AI task is enqueued after the message is stored.

- [ ] **Step 6: Run update enqueue test and verify RED**

Run: `GOCACHE=/tmp/go-build-cache go test ./internal/update -run TestProcessorEnqueuesAIMediaMetadataTaskForCloudLinks -v`

Expected: FAIL because processor does not enqueue AI tasks.

- [ ] **Step 7: Implement update enqueue dependency**

Add the same dependency to `update.ProcessorOptions`, enqueue after the transaction commits, and only for stored messages with cloud-drive links.

- [ ] **Step 8: Wire main task worker**

In `cmd/tg-search/main.go`, instantiate `ai.Service` and register:

```go
model.TaskTypeAIMediaMetadata: aiService.RunMediaMetadataTask,
```

Pass `taskService` as the AI task enqueue dependency to history and update processors.

- [ ] **Step 9: Run enqueue tests and verify GREEN**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./internal/history -run TestHistoryStoreBatchEnqueuesAIMediaMetadataTaskForCloudLinks -v
GOCACHE=/tmp/go-build-cache go test ./internal/update -run TestProcessorEnqueuesAIMediaMetadataTaskForCloudLinks -v
```

Expected: PASS.

- [ ] **Step 10: Commit Task 4**

```bash
git add internal/history/service.go internal/history/service_test.go internal/update/processor.go internal/update/processor_test.go cmd/tg-search/main.go
git commit -m "feat: enqueue ai media metadata tasks"
```

---

### Task 5: Settings UI For AI Media Metadata

**Files:**
- Modify: `web/src/api/types.ts`
- Modify: `web/src/views/SettingsView.vue`
- Modify: `web/src/views/SettingsView.test.ts`

- [ ] **Step 1: Write failing frontend test**

Add a SettingsView test that:

1. Mocks runtime settings with `ai.media_metadata`.
2. Asserts base URL/model inputs are populated.
3. Clicks “拉取模型”.
4. Selects returned model.
5. Saves and asserts `apiPut('/api/settings/runtime', payload)` includes `ai.media_metadata`.

- [ ] **Step 2: Run frontend test and verify RED**

Run: `npm run web:test -- SettingsView`

Expected: FAIL because the AI settings controls do not exist.

- [ ] **Step 3: Implement TypeScript types**

Extend `RuntimeSettings`:

```ts
ai: {
  media_metadata: {
    enabled: boolean
    base_url: string
    api_key?: string
    api_key_set?: boolean
    model: string
  }
}
```

Add:

```ts
export interface AIModelsResponse {
  items: string[]
}
```

- [ ] **Step 4: Implement SettingsView AI form**

Add `aiModels`, `aiModelsLoading`, and AI fields to `runtimeForm`. Add `loadAIModels()` calling `/api/settings/ai/models`. Add an AI panel under the runtime tab or a dedicated AI section with:

- Checkbox `data-testid="ai-media-enabled-input"`.
- Base URL input `data-testid="ai-base-url-input"`.
- API key input `data-testid="ai-api-key-input"`.
- Model select/input `data-testid="ai-model-input"`.
- Button `data-testid="fetch-ai-models"`.

`runtimePayload()` must include `ai.media_metadata`. `fillRuntimeForm()` must map `api_key_set` without populating the secret.

- [ ] **Step 5: Run frontend test and verify GREEN**

Run: `npm run web:test -- SettingsView`

Expected: PASS.

- [ ] **Step 6: Run frontend typecheck**

Run: `npm run web:typecheck`

Expected: PASS.

- [ ] **Step 7: Commit Task 5**

```bash
git add web/src/api/types.ts web/src/views/SettingsView.vue web/src/views/SettingsView.test.ts
git commit -m "feat: add ai media metadata settings ui"
```

---

### Task 6: Full Verification

**Files:**
- No planned file edits unless verification finds a bug.

- [ ] **Step 1: Run Go tests**

Run: `GOCACHE=/tmp/go-build-cache go test ./...`

Expected: PASS.

- [ ] **Step 2: Run frontend typecheck**

Run: `npm run web:typecheck`

Expected: PASS.

- [ ] **Step 3: Run frontend tests**

Run: `npm run web:test`

Expected: PASS.

- [ ] **Step 4: Review final diff**

Run: `git diff --stat main...HEAD`

Expected: changes are limited to AI metadata settings, task handling, enqueue integration, frontend settings, and design/plan docs.

- [ ] **Step 5: Final commit if needed**

If verification fixes were required:

```bash
git add <changed-files>
git commit -m "fix: stabilize ai media metadata integration"
```

Expected: no uncommitted changes remain after the final commit.
