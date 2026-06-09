# Large Video Streaming Design

## Goal

Improve large Telegram video playback smoothness for `/v/:fileid` by borrowing the useful streaming reader pattern from `teldrive` while keeping this project's current single-account media proxy model.

The optimization should help both common playback paths:

- Seeking or scrubbing to a new position should return the first bytes for that range promptly.
- Long continuous playback should avoid waiting on every Telegram chunk serially.

## Current Behavior

The API layer resolves `/v/:fileid`, parses a single HTTP byte range, sets metadata headers, and calls `Telegram.StreamVideoRange`.

`internal/telegram/media_proxy.go` currently streams the requested range by calling `upload.getFile` sequentially in `512 KiB` chunks. Each chunk is requested only after the previous chunk has been written to the HTTP response. There is no per-chunk timeout, no bounded prefetch, and no explicit stream-abandoned error classification.

This works functionally but can make large videos stall when any single Telegram chunk is slow.

## Teldrive Reference

The relevant `teldrive` pieces are:

- `internal/reader/tg_reader.go`: creates a range reader that prefetches chunks in bounded concurrent batches, stores them in an ordered buffer channel, and cancels the fetch context when the reader is closed.
- `internal/reader/buffer.go`: tracks partial reads from a chunk buffer.
- `internal/tgc/helpers.go`: uses `upload.getFile` with `Precise: true` and dynamic chunk sizing.
- `pkg/services/file.go`: keeps HTTP range parsing and response headers separate from the Telegram reader.

This project should borrow the reader shape, not the full storage model. Do not copy `teldrive`'s multi-bot selection, encrypted part reader, Redis cache dependency, or file-location cache in this change.

## Proposed Design

Add a small Telegram range prefetch reader in `internal/telegram`.

The reader will:

- Accept a `ChunkSource` interface with `Chunk(ctx, offset, limit)` and `ChunkSize(start, end)`.
- Align the first fetch offset down to the chosen chunk size.
- Clip the first and last chunk so the caller receives exactly the requested HTTP range.
- Fetch chunks in bounded batches.
- Preserve output order even when chunks finish out of order.
- Buffer a small number of prefetched chunks ahead of the HTTP writer.
- Apply a timeout per chunk fetch.
- Cancel all in-flight and future chunk fetches when closed or when the request context is canceled.

`GotdClient.StreamVideoRange` will wrap gotd `upload.getFile` behind the `ChunkSource` and stream from the reader into the HTTP writer. The existing file-reference refresh behavior remains: if the first attempt fails with a file reference error before writing any bytes, refresh the Telegram document and retry the same requested range.

## Configuration

Add optional runtime config under `telegram.stream`:

- `concurrency`: number of concurrent chunk fetches. Default `2`.
- `buffers`: number of prefetched chunk buffers. Default `4`.
- `chunk_timeout`: timeout for one Telegram chunk request. Default `20s`.

Bounds should be applied when building runtime values:

- `concurrency < 1` becomes `1`.
- `buffers < 1` becomes `1`.
- `chunk_timeout <= 0` becomes the default timeout.

These values are intentionally conservative. Each chunk may be up to `1 MiB`; default memory held by prefetched chunks is therefore small and bounded.

## Chunk Sizing

Use a dynamic chunk size similar to `teldrive`:

- Start at `1 MiB`.
- For small requested ranges, halve the chunk size until it no longer greatly exceeds the range, with a lower bound of `1 KiB`.

The existing `512 KiB` fixed chunk size will be replaced for video range streaming only. Image download behavior is out of scope.

## Error Handling

The reader should expose clear sentinel errors:

- `ErrStreamAbandoned` when the buffer channel closes before the requested range is fully read.
- `ErrChunkTimeout` when a chunk request reaches its per-chunk timeout.

When the HTTP client disconnects, request cancellation should stop background chunk fetches. If writing to the response fails after bytes have been sent, the stream should return that write error without attempting file-reference refresh.

## API Surface

No HTTP route changes are required. `/v/:fileid` keeps the same headers, status codes, and Range semantics from the previous media proxy HTTP work.

The Telegram client interface can remain unchanged:

```go
StreamVideoRange(ctx context.Context, session AccountSession, channel MediaChannelRef, messageID int, file VideoFile, offset int64, length int64, w io.Writer) error
```

The prefetch reader stays internal to `internal/telegram`.

## Testing

Add focused tests in `internal/telegram`:

- chunk sizing returns smaller chunks for small ranges and defaults to `1 MiB` for larger ranges.
- the prefetch reader returns exactly the requested clipped range when start and end are not chunk-aligned.
- chunks are emitted in byte order even when concurrent chunk calls complete out of order.
- closing the reader cancels the chunk context.
- chunk timeout is reported as `ErrChunkTimeout`.

Keep existing API media proxy tests unchanged unless they need minor updates for new defaults. Existing `internal/api` range tests should continue proving HTTP behavior.

## Non-Goals

- Multiple Telegram bots or accounts for one stream.
- Redis or persistent chunk/file-location cache.
- Frontend player UI changes.
- Multi-range HTTP responses.
- Image proxy streaming changes.
- Download acceleration outside `/v/:fileid`.
