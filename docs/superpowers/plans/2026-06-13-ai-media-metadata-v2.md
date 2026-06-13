# AI Media Metadata Engine v2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add provider presets, fallback execution, regex hints, JSON repair/validation, and settings UI support for AI media metadata.

**Architecture:** Keep the current task worker and OpenAI-compatible transport. Add a provider registry and engine wrapper around the existing client, enrich requests with deterministic hints, validate provider output before applying metadata, and expose provider presets to the Settings UI.

**Tech Stack:** Go, Gin, SQLite-backed runtime settings, Vue 3, TypeScript, Naive UI, Vitest.

---

### Task 1: Provider Registry And Runtime Contract

**Files:**
- Create: `internal/ai/providers.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/runtime_settings.go`
- Modify: `internal/config/config_test.go`
- Modify: `web/src/api/types.ts`

- [ ] Write failing backend tests that runtime settings preserve `provider` and allow Ollama without an API key.
- [ ] Add `Provider`, `FallbackEnabled`, and provider preset metadata.
- [ ] Update validation so enabled non-Ollama providers require API keys while Ollama does not.
- [ ] Run `go test ./internal/config`.

### Task 2: Engine, Fallback, Preprocessor, Validator, Repair

**Files:**
- Create: `internal/ai/preprocess.go`
- Create: `internal/ai/engine.go`
- Create: `internal/ai/validation.go`
- Modify: `internal/ai/client.go`
- Modify: `internal/ai/service.go`
- Modify: `internal/ai/client_test.go`
- Create: `internal/ai/engine_test.go`
- Create: `internal/ai/preprocess_test.go`

- [ ] Write failing tests for regex hints on year, quality, season, episode, and size.
- [ ] Write failing tests for fallback skipping failed providers and accepting the first valid response.
- [ ] Write failing tests for JSON repair of fenced JSON and trailing commas.
- [ ] Implement preprocessing, repair, validation, and engine wrapper.
- [ ] Wire `Service` to use `NewEngine` by default.
- [ ] Run `go test ./internal/ai`.

### Task 3: Provider Preset API And UI

**Files:**
- Modify: `internal/api/handlers.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/handlers_test.go`
- Modify: `web/src/views/SettingsView.vue`
- Modify: `web/src/views/SettingsView.test.ts`

- [ ] Write failing API tests for `GET /api/settings/ai/providers`.
- [ ] Return provider preset metadata from the backend.
- [ ] Write failing frontend tests for provider dropdown defaulting Base URL/model and rendering signup URL.
- [ ] Add provider select, website link, fallback toggle, and preset model options.
- [ ] Include `provider` and `fallback_enabled` in save and model-list payloads.
- [ ] Run `npm run web:typecheck` and `npm run web:test`.

### Task 4: Full Verification And Commit

**Files:**
- All modified files.

- [ ] Run `go test ./...`.
- [ ] Run `npm run web:typecheck`.
- [ ] Run `npm run web:test`.
- [ ] Review `git diff`.
- [ ] Commit with `feat: add ai media metadata provider routing`.
