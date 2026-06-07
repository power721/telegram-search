# Production Deployment Checklist

## Configuration

- `telegram.api_id` is set.
- `telegram.api_hash` is set through a private config file or protected environment workflow.
- `server.host` remains `127.0.0.1` unless there is a deliberate local-network deployment.
- `server.port` does not conflict with AList-TVBox or other sidecar processes.
- `sync.workers` is appropriate for the host CPU and Telegram account limits.
- `sync.history_batch_size` is small enough to avoid long transactions.
- `storage.path` points to persistent storage mounted at `/data/tg-provider`.

## Data Directory

- `/data/tg-provider` exists and is writable.
- `/data/tg-provider/sessions` exists and is not world-readable.
- `/data/tg-provider/logs` exists and has enough disk space.
- `/data/tg-provider/backup` exists and is included in operational backup retention.
- `telegram.db` is on reliable local storage.

## Logs

- `app.log`, `sync.log`, `telegram.log`, and `error.log` are created under `/data/tg-provider/logs`.
- Log rotation is enabled.
- Logs are not shipped to public services without redaction.
- Login codes, passwords, API hashes, and session file contents are not logged.

## Backup

- Trigger a backup after first successful login and channel sync:

  ```bash
  curl -s -X POST http://127.0.0.1:6000/api/maintenance/backup
  ```

- Verify the returned backup file exists under `/data/tg-provider/backup`.
- Keep backup retention bounded by external cleanup policy.
- Do not run filesystem copies of `telegram.db` without SQLite backup or `VACUUM INTO` while the service is running.

## Startup

- Startup order is config load, runtime directory creation, logger creation, SQLite open, migrations, account/session restore, scheduler start, API start.
- Failure at any startup step exits the process.
- AList-TVBox container startup supervises both the Java/native app and `tg-provider`.

## Shutdown

- Stop HTTP acceptance first through process termination.
- Stop scheduler jobs.
- Wait for queued sync jobs to finish or hit shutdown timeout.
- Stop account/update runtime.
- Close SQLite after runtime components have stopped using it.
- Keep shutdown timeout long enough for small active batches to finish.

## Health Check

- Use:

  ```bash
  curl -fsS http://127.0.0.1:6000/api/status
  ```

- Alert if the API is unavailable.
- Alert if account states remain `RECONNECTING`, `FLOOD_WAIT`, or `DISCONNECTED` unexpectedly.
- Check `error.log` after restart loops.

## Security

- Do not publish port `6000`.
- Restrict filesystem permissions for config and sessions.
- Keep Telegram credentials out of shell history and public logs.
- Delete accounts through `DELETE /api/accounts/{id}` so runtime state and session files are cleaned together.
- Review backup files before moving them outside the host.
