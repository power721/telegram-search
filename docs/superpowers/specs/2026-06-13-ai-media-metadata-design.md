# AI Media Metadata Design

## Goal

Add optional AI media metadata enhancement for newly indexed cloud-drive resources. Admin users can configure an OpenAI-compatible endpoint, API key, and model in Settings, fetch available models, and enable asynchronous AI correction of media metadata extracted by rules.

## Scope

This feature applies to messages indexed after AI enhancement is enabled. It does not automatically backfill existing resources. A backfill task can be added later as a separate feature.

The enhancement targets cloud-drive links and leaves magnets, ED2K, generic HTTP links, and Telegram files out of the initial scope unless they are already represented as cloud-drive links.

## User Experience

Settings adds an AI media metadata section with:

- Enabled switch.
- Base URL input for OpenAI-compatible APIs.
- API key password input. Empty values on save preserve the existing key.
- Model input or select.
- Button to fetch models from the current form values. If the API key field is empty, the backend uses the saved key.

The settings response never returns the API key. It returns `api_key_set` so the UI can show that a key has already been configured.

## Architecture

The implementation reuses the existing `sync_tasks` table and task worker. Link extraction and message persistence remain synchronous and rule-based. After a message is saved and its links are persisted, the history/update pipeline enqueues one AI enhancement task per message when AI is enabled and the message has cloud-drive links.

The worker loads the message, raw Telegram JSON, media summary, and all links for that message. It sends one model request per message so a single prompt can reason about messages containing multiple cloud-drive links and multiple media entries. The response maps enhanced media metadata back to existing links by `link_id` first and `url` second.

## Data Flow

1. Telegram history sync or realtime update stores a message.
2. Existing rules extract links and media metadata.
3. If AI enhancement is enabled and the message has cloud-drive links, enqueue `ai_media_metadata` with `message_id`.
4. Task worker loads the message and links.
5. AI client calls:
   - `GET {base_url}/models` for the settings model list endpoint.
   - `POST {base_url}/chat/completions` for enhancement.
6. Worker validates model output as JSON.
7. Worker updates only the matched links' media fields.
8. Resource statistics are refreshed after updates.

## AI Request Contract

The worker sends structured JSON in the user message:

```json
{
  "message": {
    "id": 123,
    "text": "original message text",
    "raw_json": "{...}",
    "message_type": "text",
    "media_summary": ""
  },
  "links": [
    {
      "link_id": 456,
      "type": "quark",
      "url": "https://pan.quark.cn/s/abc",
      "password": "",
      "note": "rule note",
      "source_snippet": "nearby text",
      "media": {
        "title": "rule title",
        "year": "2026",
        "season": "",
        "episode": "更新06集",
        "quality": "4K",
        "size": "",
        "tmdb_id": "",
        "category": "国产剧",
        "tags": ""
      }
    }
  ]
}
```

The model must return JSON only:

```json
{
  "items": [
    {
      "link_id": 456,
      "url": "https://pan.quark.cn/s/abc",
      "media": {
        "title": "迷墙",
        "year": "2026",
        "season": "S01",
        "episode": "更新06集",
        "quality": "4K",
        "size": "",
        "tmdb_id": "",
        "category": "国产剧",
        "tags": "悬疑 国产剧"
      }
    }
  ]
}
```

The worker accepts `items` as the top-level array container. Each item must match an existing link by `link_id` or `url`. Unknown links are ignored. Empty media fields are ignored so AI output does not erase rule-derived metadata unless the field has a non-empty replacement.

## Settings Storage

Runtime settings gain:

```json
{
  "ai": {
    "media_metadata": {
      "enabled": true,
      "base_url": "https://api.openai.com/v1",
      "api_key": "secret",
      "model": "gpt-4.1-mini"
    }
  }
}
```

The persisted runtime setting keeps the API key because the existing settings repository stores runtime JSON. API responses use a redacted shape that preserves the current runtime settings contract and adds `api_key_set`. Save requests accept an empty API key to keep the stored key.

## Error Handling

AI task failures do not fail Telegram sync or message persistence. If the provider is unavailable, credentials are invalid, the model is missing, or JSON validation fails, the task fails with the provider or validation error. Existing task retry support can be used from the Tasks page.

The model list endpoint accepts the current form `base_url` and optional `api_key`. If `api_key` is empty, it uses the saved key. It returns a normal API error if configuration is missing or the provider call fails. It does not persist anything.

## Security

The API key is never returned to the browser, never written to logs, and never included in task payload JSON. Tasks store only `message_id`.

## Testing

Backend tests cover:

- Runtime AI settings save/load, preserving an existing key on empty update, and redacted responses.
- Model list proxy using an OpenAI-compatible response.
- AI worker handling one message with multiple cloud-drive links and mapping multiple media outputs to the correct links.
- AI worker ignoring unknown output links and preserving existing fields when output fields are empty.
- Enqueue behavior after message/link persistence without blocking sync.

Frontend tests cover:

- Settings page rendering the AI section from runtime settings.
- Saving AI settings without leaking or requiring a stored key.
- Fetching model list and selecting a model.

## Out Of Scope

- Automatic backfill for existing resources.
- Streaming model responses.
- Provider-specific model capability detection.
- Per-channel AI settings.
- AI enhancement of Telegram files.
