# Saved Search and Notification Roadmap

This roadmap turns tg-search from a passive Telegram resource index into an active resource discovery and integration service. It prioritizes retention and ecosystem integrations while keeping the current SQLite, FTS5, listener, history sync, resource library, and public API architecture intact.

## Goals

- Let users save searches such as `哪吒3` and receive updates when matching resources enter the local index.
- Provide webhook delivery for automation tools such as n8n, Dify, FastGPT, Home Assistant, and custom services.
- Reuse the same event and delivery foundation for RSS feeds, Telegram Bot integration, and future notification channels.
- Keep existing `/api/search` and resource APIs backward compatible.

## Priority Order

1. Saved Search and notifications.
2. Webhook event center.
3. Telegram Bot integration.
4. RSS feeds.
5. Resource score and hot ranking.
6. Automatic channel classification.
7. API usage dashboard.
8. OpenSearch browser search integration.
9. AList/TVBox dedicated API.
10. AI resource normalization.

## Product Slices

### V1.2a: Event and Webhook Foundation

Add a durable event delivery layer that can fan out internal events to configured targets.

Initial events:

- `resource.created`
- `resource.updated`
- `task.completed`
- `task.failed`
- `account.offline`
- `channel.sync.completed`
- `saved_search.matched`

Admin APIs:

- `GET /api/webhooks`
- `POST /api/webhooks`
- `GET /api/webhooks/:id`
- `PUT /api/webhooks/:id`
- `DELETE /api/webhooks/:id`
- `GET /api/notification-deliveries`

Acceptance criteria:

- Admins can create, list, update, disable, and delete webhooks.
- Webhook event filters are stored as structured JSON.
- Delivery attempts are persisted with status, retry count, and last error.
- Failed deliveries can be retried by a later worker without losing payloads.

### V1.2b: Saved Search

Add saved searches that match newly indexed resources.

Admin APIs:

- `GET /api/saved-searches`
- `POST /api/saved-searches`
- `GET /api/saved-searches/:id`
- `PUT /api/saved-searches/:id`
- `DELETE /api/saved-searches/:id`
- `POST /api/saved-searches/:id/test`

Data captured per saved search:

- Name.
- Keyword.
- Optional filters such as resource type, category, account, channel, and cloud providers.
- Enabled state.
- Notification channels: RSS, webhook, Telegram Bot.

Event flow:

```text
listener/history sync
  -> resource.created
  -> saved search matcher
  -> saved_search.matched
  -> webhook/rss/bot delivery adapters
```

Acceptance criteria:

- A saved search with keyword `哪吒3` matches new resource titles, notes, snippets, and media title fields.
- Disabled saved searches do not match or enqueue deliveries.
- Matching records include the saved search ID, resource ID, title, provider, source channel, and message time.

### V1.2c: RSS Feeds

Expose RSS feeds backed by resource search and saved search matches.

Routes:

- `GET /feeds/latest`
- `GET /feeds/search?q=keyword`
- `GET /feeds/saved/:id`
- `GET /opensearch.xml`

Acceptance criteria:

- Feeds use stable GUIDs based on resource IDs or saved search match IDs.
- Feed items link to existing resource/media endpoints where possible.
- Search feeds support the same keyword and resource filters as the resource API.

### V1.2d: Telegram Bot Integration

Add a Telegram Bot adapter that reuses the existing resource search and saved search services.

Commands:

- `/search <keyword>`
- `/subscribe <keyword>`
- `/unsubscribe <id>`
- `/subscriptions`

Acceptance criteria:

- Bot search calls the same resource search service used by `/api/search`.
- Bot subscriptions create saved searches with Telegram notification enabled.
- Bot notification delivery uses the same durable delivery table.

### V1.3: Resource Score and Hot Ranking

Add a resource scoring layer for better default ordering.

Initial score:

```text
resource_score =
  source_channel_count * 10
+ message_count * 3
+ provider_count * 6
+ recency_score
+ resource_type_score
+ metadata_quality_score
```

Acceptance criteria:

- Search results can be sorted by `hot`.
- Score components are explainable in API responses for admin/debug views.
- Existing date and quality sorting remain available.

### V1.4: Channel Classification and Usage Dashboard

Classify channels from recent indexed messages and expose usage analytics.

Channel categories:

- Movie.
- Anime.
- Music.
- Software.
- Ebook.
- Tutorial.
- Other.

Usage dashboard:

- API calls over time.
- Popular keywords.
- Popular resources.
- Popular categories.
- API key activity.

### V2.0: AI Normalization and Telegram Media Library

Use deterministic parsing first, then optional AI normalization to map related messages to canonical works.

Target fields:

- Title.
- Year.
- Type.
- Resolution.
- Language.
- Source.
- TMDB/IMDb/Douban IDs where available.

Acceptance criteria:

- `流浪地球`, `流浪地球2`, and `流浪地球导演剪辑版` can be represented as related but distinct works.
- Resource pages group links, channels, and providers under normalized work pages.
- AI output is stored as reviewed metadata and does not replace raw Telegram messages.

## Data Model

Minimum tables for V1.2:

```text
saved_searches
- id
- name
- keyword
- filters_json
- notify_rss
- notify_webhook
- notify_telegram
- enabled
- created_at
- updated_at

webhooks
- id
- name
- url
- events_json
- secret
- enabled
- created_at
- updated_at

notification_deliveries
- id
- event_type
- target_type
- target_id
- payload_json
- status
- retry_count
- last_error
- next_run_at
- delivered_at
- created_at
- updated_at
```

Recommended future tables:

```text
saved_search_matches
- id
- saved_search_id
- resource_id
- event_type
- title
- provider
- channel_id
- telegram_message_id
- matched_at

resource_scores
- resource_id
- score
- source_channel_count
- message_count
- provider_count
- recency_score
- type_score
- metadata_score
- updated_at
```

## Implementation Notes

- Use repository/service boundaries that match existing `internal/repository`, `internal/resource`, and `internal/api` patterns.
- Keep all admin endpoints behind the existing admin session middleware.
- Keep public `/api/search` unchanged until a dedicated `res=groups` or feed endpoint is added.
- Store webhook secrets but never return them in list/detail responses after creation unless explicitly needed.
- Persist notification payloads before attempting delivery so failed deliveries are observable and retryable.
- Do not call external webhooks synchronously from history sync or listener hot paths.

