# Media Proxy HTTP Design

Date: 2026-06-09

## Goal

Improve `/v/:fileid` and `/i/:fileid` media proxy behavior so browsers, players, and HTTP caches receive standard metadata without forcing unnecessary stream reads for video metadata checks.

## Scope

In scope:

- Add `HEAD /v/:fileid` for video metadata responses.
- Add `HEAD /i/:fileid` for image metadata responses.
- Add stable `ETag` headers for video and image media responses.
- Add `Last-Modified` from the indexed file row when available.
- Add `Content-Disposition` using the indexed file name.
- Add cache headers suitable for immutable Telegram media references.
- Keep file-id based media session resolution from `main`.

Out of scope:

- Telegram chunk prefetch.
- File reference or location caching.
- Database schema changes.
- Frontend changes.
- Signed URL method compatibility changes.

## Design

`mediaRequestContext` will return a small media context struct containing account session, Telegram channel reference, Telegram message ID, and the indexed `model.FileResult`. Video and image handlers will use that context to set HTTP headers from local metadata before streaming or writing image bytes.

For video, `HEAD` should resolve the Telegram `VideoFile` to know exact size and MIME type, set the same headers as `GET`, and return without calling `StreamVideoRange`. Range parsing remains unchanged for `GET`.

For images, `HEAD` may still call `DownloadMessageImage` because the current Telegram client interface does not expose image metadata separately. It will set the same headers as `GET` and return without writing the body.

## Header Rules

- `ETag`: deterministic weak validator from Telegram file ID, file size, and file updated time.
- `Last-Modified`: indexed file `UpdatedAt` formatted as HTTP time when non-zero.
- `Content-Disposition`: inline with the indexed file name.
- `Cache-Control`: `public, max-age=86400` for images and video.
- `Accept-Ranges`: `bytes` for video.

## Testing

Tests cover:

- `HEAD /v/:fileid` returns video headers and does not call `StreamVideoRange`.
- `GET /v/:fileid` includes ETag, Last-Modified, Content-Disposition, Cache-Control, and existing Range headers.
- `GET /i/:fileid` includes ETag, Last-Modified, Content-Disposition, and Cache-Control.
- Existing media auth and bad range behavior remains unchanged.

