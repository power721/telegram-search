# End-to-End Smoke Test Guide

Use this guide after building a local binary or container image.

## Prerequisites

- Valid Telegram `api_id` and `api_hash`.
- A local `config.yaml` or `/data/tg-provider/config.yaml`.
- Service listening on `127.0.0.1:6000`.

## Steps

1. Start the service:

   ```bash
   go run ./cmd/tg-provider -config config.yaml
   ```

2. Check status:

   ```bash
   curl -s http://127.0.0.1:6000/api/status
   ```

   Expected: JSON with `"service":"ok"`.

3. Send login code:

   ```bash
   curl -s -X POST http://127.0.0.1:6000/api/login/send-code \
     -H 'content-type: application/json' \
     -d '{"phone":"+123456789"}'
   ```

4. Sign in:

   ```bash
   curl -s -X POST http://127.0.0.1:6000/api/login/sign-in \
     -H 'content-type: application/json' \
     -d '{"phone":"+123456789","code":"12345"}'
   ```

   If the response includes `"password_required":true`, call:

   ```bash
   curl -s -X POST http://127.0.0.1:6000/api/login/password \
     -H 'content-type: application/json' \
     -d '{"phone":"+123456789","password":"your-2fa-password"}'
   ```

5. Sync channel list for the account:

   ```bash
   curl -s -X POST http://127.0.0.1:6000/api/accounts/1/channels/sync
   ```

6. List channels:

   ```bash
   curl -s http://127.0.0.1:6000/api/channels
   ```

7. Sync one channel history:

   ```bash
   curl -s -X POST http://127.0.0.1:6000/api/channels/1/sync
   ```

8. Search for a keyword:

   ```bash
   curl -s 'http://127.0.0.1:6000/api/search?q=keyword&limit=20'
   ```

9. Verify extracted links:

   ```bash
   curl -s 'http://127.0.0.1:6000/api/links?limit=20'
   ```

10. Restart the service and check session recovery:

    Stop the process, start it again with the same storage path, then call:

    ```bash
    curl -s http://127.0.0.1:6000/api/accounts
    curl -s http://127.0.0.1:6000/api/status
    ```

    Expected: the account is still present and the service starts without requiring a new login.

## Failure Checks

- Invalid query parameters should return the standard error envelope documented in `docs/api-response-contract.md`.
- Telegram network errors and FloodWait responses should not crash the service.
- Log files should be under `/data/tg-provider/logs`.
