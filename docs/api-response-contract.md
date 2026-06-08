# API Response Contract

`tg-search` REST APIs use stable JSON envelopes so the Vue admin console and future scripts can handle responses consistently.

## Errors

All errors use:

```json
{
  "error": {
    "code": "bad_request",
    "message": "message"
  }
}
```

## Lists

List endpoints use:

```json
{
  "items": [],
  "total": 0
}
```

Some existing Phase 1A endpoints return `items` without `total`; later phases standardize all list endpoints.

## Sensitive Values

These values must never appear in API responses or logs:

- Admin password.
- `password_hash`.
- API key hash.
- Telegram `api_hash`.
- Telegram login code.
- Telegram 2FA password.
- Telegram session contents.

API key creation is the only endpoint that returns a plaintext key, and it returns it only once in the creation response.
