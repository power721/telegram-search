# PanSou Gap Closure Design

## Goal

Implement the Phase 5C PanSou-inspired result-processing pieces that help this provider expose logged-in Telegram resources in an AList/TVBox-friendly shape, without adding PanSou's public Telegram Web scraping path.

## Scope

This phase includes:

- Persisting an optional per-link `note`.
- Inferring `note` from nearby message text when a message contains several resource-title and link pairs.
- Returning merged cloud-drive links grouped by type.
- Deduplicating merged links by URL and keeping the newest message context.
- Ranking merged links by message date plus title markers.
- Documenting why PanSou cannot access the channels/groups this project targets.

This phase does not include:

- Scraping `https://t.me/s/...`.
- Calling PanSou as an upstream dependency.
- Validating cloud-drive link availability against provider APIs.
- Changing the existing `/api/search` or `/api/links` response contracts incompatibly.

## Architecture

Keep the data pipeline unchanged:

```text
gotd logged-in account -> history/update processor -> link extractor -> SQLite -> API
```

Enhance only the link metadata and read-side presentation:

```text
message text
  -> Extractor.Extract(text)
  -> []model.Link{Type, URL, Password, Note}
  -> telegram_links.note
  -> /api/links/merged grouped response
```

The extractor remains independent of SQLite, Telegram, HTTP, and API packages.

## Link Notes

`model.Link` gains:

```go
Note string `json:"note,omitempty"`
```

`telegram_links` gains:

```sql
note TEXT
```

The note is optional. Existing links remain valid with an empty note.

The note inference rule is deliberately conservative:

- If the line before a link looks like a title, use it.
- If the same line uses `标题：链接` or `标题: 链接`, use the title part unless it is only a provider label such as `夸克` or `百度网盘`.
- If there is no clear title, leave note empty.

The extractor must not invent notes from broad message headers like `链接`, `地址`, `资源`, or provider labels.

## Merged API

Add:

```text
GET /api/links/merged
```

Supported query parameters:

- `q`: optional keyword matched against message text and link note.
- `type`: optional link type.
- `account_id`
- `channel_id`
- `date_from`
- `date_to`
- `limit`
- `offset`

Response:

```json
{
  "total": 2,
  "merged_by_type": {
    "quark": [
      {
        "url": "https://pan.quark.cn/s/abc",
        "password": "",
        "note": "庆余年 S02",
        "datetime": "2026-06-07T12:00:00Z",
        "source": "tg:VIP",
        "channel_id": 1,
        "telegram_message_id": 8001
      }
    ]
  }
}
```

`total` is the total number of returned merged links after filtering and deduplication.

## Deduplication And Ranking

Deduplicate by exact stored URL. If the same URL appears multiple times, keep the item with the newest message date.

Sort within each type by:

1. Title-marker score.
2. Message date descending.
3. Link id descending.

Title markers:

```text
合集, 系列, 全, 完, 最新, complete
```

This is a read-side ranking only. It does not mutate stored data.

## Error Handling

Invalid query parameters follow the existing API error envelope:

```json
{
  "error": {
    "code": "bad_request",
    "message": "..."
  }
}
```

Database failures return `internal_error`.

## Testing

Add tests for:

- Migration adds `telegram_links.note`.
- Extractor assigns note for title-before-link messages.
- Extractor leaves note empty for provider-label-only lines.
- Repository persists and loads `note`.
- Merged repository/API groups by type, deduplicates by URL, and keeps the newest context.
- `/api/links/merged` validates common query parameters consistently with existing read APIs.
