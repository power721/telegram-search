# AI Media Metadata Engine v2 Design

## Goal

Upgrade the existing AI media metadata enhancement from a single OpenAI-compatible model call into a provider-aware engine with preset providers, regex hints, JSON validation and repair, and automatic fallback.

## Scope

This v2 slice keeps the current task model: one `ai_media_metadata` task per indexed message with cloud-drive links. It does not add backfill, streaming, Redis/NATS, multi-key account management, or TMDB/IMDb enrichment. Those can plug into the same engine later.

## User Experience

Settings adds a provider preset selector for AI media metadata. Presets include OpenAI, OpenAI Compatible, Ollama, Zhipu, Groq, Cerebras, SiliconFlow, and ModelScope. Selecting a preset fills the Base URL and default model while still allowing manual edits.

Each preset exposes:

- display name;
- default Base URL;
- default model;
- API key environment variable when applicable;
- whether the provider is local or usually free-tier friendly;
- official signup or documentation URL.

The UI keeps the existing API key behavior: empty key on save preserves the stored secret. Ollama does not require an API key.

## Architecture

The existing `internal/ai.Client` remains the OpenAI-compatible transport. v2 adds:

- provider registry for defaults, UI metadata, and fallback ordering;
- engine wrapper that builds an ordered provider chain from runtime settings and environment keys;
- preprocessor that extracts deterministic hints such as year, quality, season, episode, and size;
- validator that rejects malformed response items before metadata is applied;
- JSON repair layer that retries parsing after trimming markdown, removing trailing commas, and balancing simple truncation;
- settings API response fields for provider metadata and model presets.

## Data Flow

1. Telegram history sync or realtime update stores a message and extracted links.
2. Existing rule extraction fills initial link media fields.
3. The AI task worker loads message and cloud-drive links.
4. The preprocessor adds deterministic hints to `EnhancementRequest.Context` and `EnhancementLink.RawHint`.
5. The engine selects the configured provider first, then optional fallback providers with usable credentials.
6. Each provider call must return valid JSON and pass response validation.
7. The worker overlays only non-empty metadata fields onto matched links.
8. Resource grouped indexes refresh after updates.

## Provider Strategy

Default fallback order is:

1. Groq
2. SiliconFlow
3. ModelScope
4. Ollama
5. OpenAI

The selected provider always runs first. Fallback candidates that require an API key are skipped unless the selected provider key or provider-specific environment variable is available.

## Compatibility

Existing runtime configs with only `base_url`, `api_key`, and `model` remain valid. Missing `provider` is treated as `openai_compatible`.

Existing API responses still redact API keys and include `api_key_set`.

Existing link media storage stays string-based. `tags` remains a string because the database and public resource API currently store it that way.

## Out Of Scope

- TMDB/IMDb enrichment and duplicate matching.
- Storing multiple API keys in the runtime settings.
- Per-provider capability probing.
- Redis/NATS task queues.
- Changing public media metadata schema from strings to arrays or confidence scores.
