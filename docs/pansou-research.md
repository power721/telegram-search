# PanSou Telegram Resource Search Research

Date: 2026-06-07

## Summary

PanSou is a high-performance public resource aggregation service. Its Telegram path searches public Telegram Web pages and does not log in to Telegram. This project should not copy that Telegram access path. The useful parts are PanSou's link parsing, per-link title attribution, link grouping, deduplication, and result ranking ideas.

This project remains focused on a different gap: logging in with a user's Telegram account and indexing channels, groups, and saved messages that public Telegram Web search cannot access.

## How PanSou Searches Telegram

PanSou's Telegram search builds a public Web URL:

```text
https://t.me/s/{channel}?q={keyword}
```

The relevant code is in `/home/harold/workspace/pansou/util/http_util.go`, where `BuildSearchURL` joins `https://t.me/s/`, the channel username, and the `q` query parameter.

PanSou then:

1. Runs concurrent HTTP GET requests across configured public channel usernames.
2. Parses the returned HTML using `.tgme_widget_message_wrap`.
3. Extracts message id, time, text, anchor `href` values, and cloud-drive links.
4. Merges those Telegram results with plugin results from other public resource sites.
5. Groups final links by cloud-drive type.

There is no MTProto session, no gotd client, no Telegram login, and no access to private channels, private groups, joined-only groups, saved messages, or public channels whose Web preview is unavailable.

## Implication For This Project

PanSou and `tg-provider` are complementary:

- PanSou covers public Web-accessible channels and public plugin sources.
- `tg-provider` covers the user's logged-in Telegram scope: joined channels, groups, and saved messages.

This project should not add `t.me/s` scraping as the primary search path. Doing so would duplicate PanSou's public-channel approach and still not solve the inaccessible-channel problem.

## What To Borrow

Borrow these result-processing ideas:

- Provider-specific link parsing for common cloud drives.
- Password/code extraction from URL parameters and nearby text.
- Per-link title attribution when one Telegram message contains many resources.
- `merged_by_type` responses for AList/TVBox-friendly consumption.
- URL deduplication that keeps the freshest message context.
- Sorting that favors newer resources and useful title markers such as `合集`, `系列`, `全`, `完`, `最新`, and `complete`.

## What Not To Borrow

Do not borrow these as project direction:

- Public `https://t.me/s/...` HTML scraping as the main Telegram search mechanism.
- Replacing logged-in Telegram indexing with public plugin aggregation.
- Treating HTTP scrape timeouts and cache misses as the core data model.

## Implementation Phase

Add a focused phase called `Phase 5C: PanSou Gap Closure`, placed before the existing AList-TVBox integration phase.

The phase keeps the existing logged-in sync/search architecture and adds:

1. A `note` field for extracted links.
2. Link title attribution during history sync and realtime update processing.
3. A merged-by-type link API.
4. Freshness and title-marker ranking for the merged API.
5. Tests for parser behavior, repository persistence, and API response shape.
