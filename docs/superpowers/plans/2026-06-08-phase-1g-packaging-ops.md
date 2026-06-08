# Phase 1G Packaging Ops Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Package `tg-search` for Docker-friendly production use with frontend embedding, Compose, release artifacts, logs, backup/restore documentation, health checks, and smoke tests.

**Architecture:** Build a single Go binary that serves the Vue admin console and REST APIs. Docker runs as a self-contained service using `/data/tg-search` for config, database, sessions, logs, backups, index files, and thumbnails. Operational docs focus on NAS, Raspberry Pi, VPS, and small-disk deployments.

**Tech Stack:** Go 1.25, Vue/Vite build output, Docker multi-stage builds, Docker Compose, GitHub Actions, SQLite, shell smoke scripts.

---

## Prerequisite

Complete Phase 1F first:

[Phase 1F Runtime Reliability Plan](/home/harold/workspace/telegram-search/docs/superpowers/plans/2026-06-08-phase-1f-runtime-reliability.md)

## Scope

In scope:

- Embed or serve built Vue frontend from the Go binary.
- Add production Dockerfile and `.dockerignore`.
- Add Docker Compose example using `/data/tg-search`.
- Add health/readiness endpoints or document existing status endpoint for health checks.
- Add startup directory validation and clear permission errors.
- Add log file configuration and redaction verification.
- Add backup and restore commands/docs.
- Add release workflow that builds Linux artifacts and Docker image metadata.
- Add smoke tests for empty-data startup, setup, login mock, metadata sync mock, search smoke, resource smoke, task smoke, backup smoke, and restart smoke.

Out of scope:

- Kubernetes Helm charts.
- Redis, ElasticSearch, vector database deployment.
- Media proxy or file-drive packaging.
- AList/TVBox/T4/provider compatibility.

## Production Data Layout

Runtime data lives under:

```text
/data/tg-search
├── config.yaml
├── tg-search.db
├── sessions
├── logs
├── uploads
├── backup
├── index
└── thumbnails
```

Default storage quota:

```yaml
storage:
  max_db_size: 10GB
  max_media_cache: 20GB
```

## File Structure

- Modify `cmd/tg-search/main.go`: serve embedded frontend and startup checks.
- Create `internal/web/embed.go`: embedded `web/dist` filesystem.
- Modify `internal/api/router.go`: static frontend fallback and health endpoints.
- Modify `internal/config/config.go`: production path validation and log settings if missing.
- Create `Dockerfile`.
- Create `.dockerignore`.
- Create `compose.yaml`.
- Create `.github/workflows/release.yml`.
- Create `scripts/smoke.sh`.
- Create `scripts/backup.sh`.
- Create `scripts/restore.sh`.
- Modify `docs/production-deployment-checklist.md`.
- Modify `docs/smoke-test-guide.md`.
- Modify `README.md`.
- Add tests for frontend fallback and startup validation.

## Task 1: Frontend Build Serving

**Files:**

- Create: `internal/web/embed.go`
- Modify: `internal/api/router.go`
- Modify: `cmd/tg-search/main.go`
- Test: `internal/api/router_test.go`

- [ ] **Step 1: Write router tests**

Verify:

- `GET /` returns HTML when embedded assets exist.
- `GET /channels` returns the same app shell fallback.
- `GET /api/status` still returns JSON and is not swallowed by frontend fallback.

Run:

```bash
go test ./internal/api -run 'TestFrontendFallback' -v
```

Expected: FAIL until static frontend serving exists.

- [ ] **Step 2: Add embed package**

Create `internal/web/embed.go`:

```go
package web

import "embed"

//go:embed dist/*
var Dist embed.FS
```

Build process copies `web/dist` to `internal/web/dist` before `go build`.

- [ ] **Step 3: Add frontend fallback**

API routes keep `/api/*`. Non-API GET requests serve embedded assets or `index.html`.

- [ ] **Step 4: Verify and commit**

Run:

```bash
npm run web:build
go test ./internal/api
```

Expected: PASS.

Commit:

```bash
git add internal/web/embed.go internal/api/router.go internal/api/router_test.go cmd/tg-search/main.go
git commit -m "feat: serve embedded admin console"
```

## Task 2: Docker Build

**Files:**

- Create: `Dockerfile`
- Create: `.dockerignore`
- Create: `compose.yaml`
- Modify: `README.md`

- [ ] **Step 1: Add Dockerfile**

Use a multi-stage build:

```dockerfile
FROM node:22-alpine AS web
WORKDIR /src
COPY package.json ./
COPY web/package.json web/package-lock.json ./web/
RUN npm ci --prefix web
COPY web ./web
RUN npm run web:build

FROM golang:1.25-alpine AS go-build
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -o /out/tg-search ./cmd/tg-search

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=go-build /out/tg-search /usr/local/bin/tg-search
EXPOSE 6000
VOLUME ["/data/tg-search"]
ENTRYPOINT ["tg-search"]
```

- [ ] **Step 2: Add `.dockerignore`**

Include:

```text
.git
web/node_modules
web/dist
internal/web/dist
tmp
*.db
*.log
```

- [ ] **Step 3: Add Compose**

Create `compose.yaml`:

```yaml
services:
  tg-search:
    build: .
    image: tg-search:local
    container_name: tg-search
    restart: unless-stopped
    ports:
      - "6000:6000"
    volumes:
      - ./data:/data/tg-search
    environment:
      - TZ=Asia/Shanghai
```

- [ ] **Step 4: Verify and commit**

Run:

```bash
docker build -t tg-search:local .
docker compose config
```

Expected: image builds and Compose config is valid.

Commit:

```bash
git add Dockerfile .dockerignore compose.yaml README.md
git commit -m "build: add docker packaging"
```

## Task 3: Startup Validation And Health

**Files:**

- Modify: `internal/config/config.go`
- Modify: `internal/storage/usage.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/handlers.go`
- Test: `internal/config/config_test.go`
- Test: `internal/api/handlers_test.go`

- [ ] **Step 1: Write startup validation tests**

Verify missing or unwritable `sessions`, `logs`, `backup`, `index`, and `thumbnails` directories return clear errors.

Run:

```bash
go test ./internal/config -run 'TestRuntimeDirectoryValidation' -v
```

Expected: FAIL until validation exists.

- [ ] **Step 2: Implement runtime directory validation**

On startup:

- Create missing directories.
- Fail if a path exists as a file.
- Fail if write test cannot create and remove a small probe file.

- [ ] **Step 3: Add health endpoints**

Add:

```text
GET /api/health
GET /api/ready
```

`health` returns process health. `ready` verifies database access and runtime directory writeability.

- [ ] **Step 4: Verify and commit**

Run:

```bash
go test ./internal/config ./internal/api ./internal/storage
```

Expected: PASS.

Commit:

```bash
git add internal/config/config.go internal/config/config_test.go internal/storage/usage.go internal/api/router.go internal/api/handlers.go internal/api/handlers_test.go
git commit -m "feat: add health checks and startup validation"
```

## Task 4: Backup And Restore Operations

**Files:**

- Create: `scripts/backup.sh`
- Create: `scripts/restore.sh`
- Modify: `docs/production-deployment-checklist.md`
- Modify: `README.md`

- [ ] **Step 1: Add backup script**

Create `scripts/backup.sh`:

```sh
#!/bin/sh
set -eu
DATA_DIR="${DATA_DIR:-/data/tg-search}"
BACKUP_DIR="${BACKUP_DIR:-$DATA_DIR/backup}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$BACKUP_DIR"
sqlite3 "$DATA_DIR/tg-search.db" ".backup '$BACKUP_DIR/tg-search-$STAMP.db'"
tar -C "$DATA_DIR" -czf "$BACKUP_DIR/tg-search-$STAMP-sessions.tgz" sessions config.yaml
printf '%s\n' "$BACKUP_DIR/tg-search-$STAMP.db"
```

- [ ] **Step 2: Add restore script**

Create `scripts/restore.sh`:

```sh
#!/bin/sh
set -eu
if [ "$#" -lt 1 ]; then
  printf 'usage: %s /path/to/tg-search-backup.db\n' "$0" >&2
  exit 2
fi
DATA_DIR="${DATA_DIR:-/data/tg-search}"
mkdir -p "$DATA_DIR"
cp "$1" "$DATA_DIR/tg-search.db"
printf 'restored %s to %s\n' "$1" "$DATA_DIR/tg-search.db"
```

- [ ] **Step 3: Verify scripts**

Run:

```bash
/bin/sh -n scripts/backup.sh
/bin/sh -n scripts/restore.sh
```

Expected: shell syntax is valid.

- [ ] **Step 4: Commit**

```bash
git add scripts/backup.sh scripts/restore.sh docs/production-deployment-checklist.md README.md
git commit -m "ops: add backup and restore scripts"
```

## Task 5: Smoke Tests

**Files:**

- Create: `scripts/smoke.sh`
- Modify: `docs/smoke-test-guide.md`

- [ ] **Step 1: Add smoke script**

Create `scripts/smoke.sh`:

```sh
#!/bin/sh
set -eu
BASE_URL="${BASE_URL:-http://127.0.0.1:6000}"
curl --fail --silent "$BASE_URL/api/health" >/dev/null
curl --fail --silent "$BASE_URL/api/ready" >/dev/null
curl --fail --silent "$BASE_URL/api/setup/status" >/dev/null
curl --fail --silent "$BASE_URL/" >/dev/null
printf 'tg-search smoke checks passed for %s\n' "$BASE_URL"
```

- [ ] **Step 2: Add extended smoke guide**

Document manual smoke flow:

```text
empty data startup
setup wizard
Telegram API config
Telegram login with mock or test account
metadata sync
channel control
Quick history sync
Global Search
Resources
task retry/cancel view
backup
restart
```

- [ ] **Step 3: Verify script**

Run:

```bash
/bin/sh -n scripts/smoke.sh
```

Expected: shell syntax is valid.

- [ ] **Step 4: Commit**

```bash
git add scripts/smoke.sh docs/smoke-test-guide.md
git commit -m "test: add production smoke checks"
```

## Task 6: Release Workflow

**Files:**

- Create: `.github/workflows/release.yml`
- Modify: `README.md`

- [ ] **Step 1: Add release workflow**

Workflow behavior:

- On tag `v*`, run Go tests and frontend tests.
- Build frontend.
- Build Linux amd64 and arm64 binaries.
- Upload artifacts named `tg-search-linux-amd64` and `tg-search-linux-arm64`.
- Build Docker image metadata using image name `tg-search`.

- [ ] **Step 2: Verify workflow YAML**

Run:

```bash
rg -n 'tg-provider|AList|TVBox|T4' .github Dockerfile compose.yaml README.md
```

Expected: no active packaging references to old product/provider names.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/release.yml README.md
git commit -m "ci: add release workflow"
```

## Task 7: Final Production Documentation

**Files:**

- Modify: `README.md`
- Modify: `docs/production-deployment-checklist.md`
- Modify: `docs/api.md`

- [ ] **Step 1: Document production run**

Include:

```bash
docker compose up -d
docker compose logs -f tg-search
curl http://127.0.0.1:6000/api/health
```

- [ ] **Step 2: Document storage quota operations**

Explain `storage.max_db_size` and `storage.max_media_cache`, and state that Phase 1 reports usage, warns, and blocks new Deep/Full sync when the DB quota is exceeded.

- [ ] **Step 3: Run final verification**

Run:

```bash
go test ./...
npm run web:typecheck
npm run web:test
/bin/sh -n scripts/smoke.sh
/bin/sh -n scripts/backup.sh
/bin/sh -n scripts/restore.sh
docker compose config
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add README.md docs/production-deployment-checklist.md docs/api.md
git commit -m "docs: finalize production operations"
```

## Self-Review Checklist

- [ ] Docker image, Compose service, binary, and docs use `tg-search`.
- [ ] Default data directory is `/data/tg-search`.
- [ ] Frontend build is served by the Go binary.
- [ ] Health and readiness checks work without browser access.
- [ ] Backup and restore docs are usable by NAS/VPS operators.
- [ ] Smoke tests cover setup, runtime, search/resources, tasks, backup, and restart.
- [ ] No AList/TVBox/T4/provider compatibility remains in active packaging docs.
