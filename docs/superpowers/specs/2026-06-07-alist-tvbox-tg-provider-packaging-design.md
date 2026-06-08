# AList-TVBox tg-provider Packaging Design

## Scope

Package the Go `tg-provider` binary into every currently active Docker image built by `/home/harold/workspace/alist-tvbox/.github/workflows/build.yaml`, and optionally run it as an internal second process in the same container.

This phase does not integrate the AList-TVBox Java or native application with the provider API. Search routing, DTO mapping, and UI entry points remain out of scope.

## Build Targets

The active `build.yaml` Docker publish steps build these images:

- `xiaoya-tvbox:latest` from `docker/Dockerfile-xiaoya`
- `xiaoya-tvbox:hostmode`, `xiaoya-tvbox:host`, `xiaoya-tvbox-hostmode:latest`, and `xiaoya-tvbox-host:latest` from `docker/Dockerfile-host`
- `alist-tvbox:latest` from `docker/Dockerfile`
- `alist-tvbox:native` and `alist-tvbox-native:latest` from `docker/Dockerfile-alist-native`
- `xiaoya-tvbox:native` and `xiaoya-tvbox-native:latest` from `docker/Dockerfile-native`
- `xiaoya-tvbox:native-host` and `xiaoya-tvbox-native-host:latest` from `docker/Dockerfile-native-host`

Every active image gets `/usr/local/bin/tg-provider`.

The commented Python/TG image steps stay disabled. `docker/Dockerfile-tg` and `docker/Dockerfile-xiaoya-tg` are not part of this phase.

## Workflow Packaging

`telegram-search` owns provider compilation and release packaging. Its latest GitHub Release must publish these assets:

- `tg-provider-linux-amd64`
- `tg-provider-linux-arm64`
- `checksums.txt`

`alist-tvbox` does not check out or compile `telegram-search` during its Docker build. `build.yaml` downloads the latest `power721/telegram-search` release before Docker image builds:

1. Resolve the latest release for `power721/telegram-search`.
2. Download `tg-provider-linux-amd64`, `tg-provider-linux-arm64`, and `checksums.txt`.
3. Verify both binaries against `checksums.txt`.
4. Make both binaries executable.
5. Place binaries under the AList-TVBox build context:
   - `build/tg-provider/linux-amd64/tg-provider`
   - `build/tg-provider/linux-arm64/tg-provider`
6. Record the downloaded provider release tag in `build/tg-provider/version` for build logs.

If the latest release is missing an asset or checksum verification fails, `build.yaml` fails the Docker build. It does not fall back to a bundled, stale, or previously cached provider binary.

Each active Dockerfile uses `ARG TARGETARCH` and copies the matching binary:

```dockerfile
ARG TARGETARCH
COPY build/tg-provider/linux-${TARGETARCH}/tg-provider /usr/local/bin/tg-provider
```

This keeps multi-platform Docker builds simple and avoids compiling Go during every AList-TVBox build. It also makes the native AMD64-only images work with the same copy path.

Using the latest provider release is an accepted tradeoff: rebuilding the same AList-TVBox commit may include a newer `tg-provider` binary if `telegram-search` publishes a newer release.

## Runtime Model

The container remains a same-container multi-process runtime. No `supervisord` is added.

Existing entrypoints keep their current responsibilities:

- load `/data/env`
- apply `/data/proxy.txt`
- prepare `/data/log`
- run `/init.sh`
- start `busybox httpd` and `nginx` for xiaoya/host variants
- run the Java or native AList-TVBox application

A shared shell helper, copied to `/tg-provider-runtime.sh`, owns provider startup and shutdown behavior. The four active entrypoint scripts call it instead of duplicating process management logic:

- root `entrypoint.sh`
- root `entrypoint-native.sh`
- `scripts/entrypoint.sh`
- `scripts/entrypoint-native.sh`

The helper starts `tg-provider` by generating or loading provider configuration:

- If `/data/tg-provider/config.yaml` exists, use it.
- Else generate a minimal config at `/data/tg-provider/config.yaml` using environment values first, then built-in defaults.

Credential precedence for generated config:

1. `API_ID` and `API_HASH` supplied by the user.
2. `TG_API_ID` and `TG_API_HASH` supplied by the user, as compatibility aliases.
3. Built-in defaults:
   - `API_ID=26375241`
   - `API_HASH=70f574f48a016d683c64f2f7a217d04f`

The generated minimal config binds to localhost only:

```yaml
telegram:
  api_id: ${API_ID}
  api_hash: ${API_HASH}
server:
  host: 127.0.0.1
  port: 9900
sync:
  workers: 5
  history_batch_size: 100
storage:
  path: /data/tg-provider
```

Port `9900` is not exposed in Dockerfiles and is not intended for host publishing.

## Data Layout

The provider stores all persistent state under `/data/tg-provider`:

- `/data/tg-provider/config.yaml`
- `/data/tg-provider/telegram.db`
- `/data/tg-provider/sessions`
- `/data/tg-provider/logs`
- `/data/tg-provider/backup`

Logs are written to `/data/tg-provider/logs/stdout.log` and `/data/tg-provider/logs/stderr.log`.

The provider uses the Go SQLite driver already built into the binary, so runtime packaging does not require adding a separate SQLite library.

## Process Lifecycle

The entrypoint starts the primary AList-TVBox process in the background and polls both critical processes:

- If AList-TVBox exits, stop `tg-provider` and exit with the AList-TVBox status.
- If `tg-provider` was configured and exits first, stop AList-TVBox and exit non-zero so the container restart policy can recover it.
- On `SIGTERM` or `SIGINT`, stop both processes and then exit.

The helper avoids relying on `wait -n` so it works with BusyBox `sh`.

`busybox httpd` and `nginx` remain non-critical helper processes, matching the existing entrypoint behavior.

## Verification

Provider repository release workflow:

- `go test ./...`
- `GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o tg-provider-linux-amd64 ./cmd/tg-provider`
- `GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o tg-provider-linux-arm64 ./cmd/tg-provider`
- `sha256sum tg-provider-linux-amd64 tg-provider-linux-arm64 > checksums.txt`
- publish all three files to the latest GitHub Release

AList-TVBox repository:

- Maven package still produces `target/application` and `target/atv`.
- `build.yaml` downloads and verifies the latest provider release assets.
- Buildx can build all six active Dockerfiles.
- Runtime smoke test confirms:
  - `/usr/local/bin/tg-provider` exists in each image.
  - container starts provider with generated config when no provider config is mounted, using built-in API defaults.
  - user-provided `API_ID` and `API_HASH` override built-in API defaults.
  - user-provided `/data/tg-provider/config.yaml` overrides both environment values and built-in API defaults.
  - `curl http://127.0.0.1:9900/api/status` succeeds inside the container when provider is configured.

## Explicit Non-Goals

- No Spring or native application API integration.
- No search result DTO mapping.
- No UI pages for Telegram login or provider status.
- No public exposure of provider port `9900`.
- No revival of the old Python/Telethon TG image tags.
