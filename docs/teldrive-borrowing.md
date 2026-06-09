# Teldrive Borrowing Notes

Date: 2026-06-09

This note records what `tg-search` can realistically borrow from `/home/harold/workspace/teldrive`. The two projects overlap around Telegram access and media handling, but their product goals are different: `teldrive` is a Telegram-backed file drive, while `tg-search` is a local Telegram search and resource library.

## High-Value Borrowing Targets

### Telegram Client Reliability

`teldrive` wraps gotd clients with proxy support, FloodWait handling, rate limiting, recovery middleware, exponential reconnect backoff, configurable device parameters, retry limits, and dial timeouts.

For `tg-search`, the useful subset is:

- `telegram.proxy` for HTTP/SOCKS5 access.
- gotd FloodWait middleware for API calls.
- gotd rate limit middleware with configurable rate and burst.
- reconnect backoff and dial timeout options.
- configurable Telegram client logging.

This should be the first backend optimization because it improves login, channel sync, remote search, media proxy, and realtime listeners without changing the product model.

### Media Proxy Streaming

`teldrive` has a richer Telegram file reader with chunk location caching, parallel chunk prefetch, per-chunk timeout, Range clipping, and stream cancellation.

For `tg-search`, useful additions are:

- `HEAD` support for media URLs.
- `ETag`, `Last-Modified`, and `Content-Disposition` headers.
- cached Telegram file locations or refreshed file references.
- bounded concurrent prefetch for video ranges.
- clearer stream-abandoned and chunk-timeout error handling.

This is high leverage because `tg-search` already exposes `/v/:channel/:msgid` and `/i/:channel/:msgid` media endpoints.

### Lightweight Cache Abstraction

`teldrive` uses a small `Cacher` interface with memory and Redis backends. `tg-search` should not need Redis immediately, but the pattern is useful.

Good first uses:

- channel username resolution.
- media file metadata or file references.
- resource grouped counts.
- web access detection results.

Redis should remain a future deployment option, not a default dependency.

### Event Broadcasting

`teldrive` supports both Redis Pub/Sub and DB-polling broadcasters. `tg-search` currently has an in-memory SSE broker for task/account/listener/activity events.

Borrow the design only when multi-instance support becomes a requirement:

- persist events first.
- broadcast locally for single instance.
- optionally fan out through Redis for multiple instances.
- deduplicate events by ID.

### Configuration Surface

`teldrive` has a full `koanf + cobra + env + flags + validator` stack. `tg-search` should stay simpler, but it can add targeted config keys:

- `server.read_timeout`
- `server.write_timeout`
- `telegram.proxy`
- `telegram.rate_limit.enabled`
- `telegram.rate_limit.rate`
- `telegram.rate_limit.burst`
- `telegram.reconnect_timeout`
- `media.stream_concurrency`
- `media.chunk_timeout`
- `logging.level`

Avoid replacing the existing config loader unless runtime configuration becomes much more complex.

## Medium-Value Borrowing Targets

### File and Resource Categories

`teldrive` classifies files into document, image, video, audio, archive, and other. `tg-search` can use a similar extension map to improve resource filtering and grouped counts.

### HTTP Access Logging

`teldrive` has configurable HTTP logging with status, method, path, latency, request size, response size, user-agent truncation, and static-asset skip rules.

`tg-search` already writes structured logs. Adding a Gin access log middleware would improve operational debugging, especially for slow search queries and media proxy failures.

### Session Storage

`teldrive` supports Postgres, BoltDB, and memory session storage. `tg-search` currently stores one session file per account, which is appropriate for a single-node local app. A Bolt or SQLite session backend is worth considering only if session file management becomes fragile.

### Release Workflow

`teldrive` uses GoReleaser for changelog groups, checksums, archives, OCI labels, and draft releases. `tg-search` already has a working release workflow. Borrow only:

- generated changelog grouping.
- OCI image labels.
- optional draft release mode.

## Things Not To Copy

- Do not switch `tg-search` from SQLite/FTS5 to Postgres/GORM. SQLite is aligned with local self-hosted search.
- Do not migrate the Gin API to `ogen` unless API contract generation becomes a project goal.
- Do not bring in upload, drive, file-copy, bot-token rotation, or Telegram-backed storage features. Those belong to `teldrive`'s product model.
- Do not use `teldrive`'s default Telegram app ID/hash. Keep the current admin-configured Telegram API setup.

## Suggested Optimization Order

1. Add targeted Telegram client reliability config and gotd middleware.
2. Improve media proxy HTTP behavior and stream robustness.
3. Introduce a small in-memory cache abstraction where repeated Telegram lookups are expensive.
4. Expand file/resource category classification.
5. Add HTTP access logging and slow request visibility.
6. Revisit event persistence only if multi-instance or restart replay becomes important.

