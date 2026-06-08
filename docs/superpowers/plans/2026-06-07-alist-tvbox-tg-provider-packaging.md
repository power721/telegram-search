# AList-TVBox tg-provider Packaging Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish `tg-provider` Linux binaries from `telegram-search`, then package the latest provider release into every active `alist-tvbox` Docker image and optionally run it as a same-container helper process.

**Architecture:** `telegram-search` owns provider build, checksum, and GitHub Release publishing. `alist-tvbox` downloads the latest release assets during `build.yaml`, verifies checksums, copies the matching binary into every active Docker image, and uses a shared POSIX shell helper to start/stop `tg-provider` alongside the existing Java/native app. No AList-TVBox API or search adapter is added in this phase.

**Tech Stack:** GitHub Actions, Go, GitHub Releases, Docker Buildx, Alpine/BusyBox `sh`, Spring Boot layered Docker images, GraalVM native Docker images.

---

## File Map

`/home/harold/workspace/telegram-search/.github/workflows/release.yaml`
: New workflow. Builds and tests `tg-provider`, creates `tg-provider-linux-amd64`, `tg-provider-linux-arm64`, and `checksums.txt`, then publishes them to the GitHub Release marked as latest.

`/home/harold/workspace/alist-tvbox/.github/workflows/build.yaml`
: Modify active Docker build workflow. Add one step after `Set APP version` to download the latest `telegram-search` release assets, verify checksums, and place binaries under `build/tg-provider/linux-{amd64,arm64}/tg-provider`.

`/home/harold/workspace/alist-tvbox/scripts/tg-provider-runtime.sh`
: New shared entrypoint helper. Generates provider config using mounted config, user env, or built-in API defaults; starts `tg-provider`; runs the primary app as the critical foreground workload; terminates both processes cleanly.

`/home/harold/workspace/alist-tvbox/scripts/entrypoint.sh`
: Modify alist JVM entrypoint to source the helper and run Java through it.

`/home/harold/workspace/alist-tvbox/scripts/entrypoint-native.sh`
: Modify alist native entrypoint to source the helper and run `./atv` through it.

`/home/harold/workspace/alist-tvbox/entrypoint.sh`
: Modify xiaoya/host JVM entrypoint to preserve httpd/nginx startup, then source the helper and run Java through it.

`/home/harold/workspace/alist-tvbox/entrypoint-native.sh`
: Modify xiaoya/host native entrypoint to preserve httpd/nginx startup, then source the helper and run `./atv` through it.

`/home/harold/workspace/alist-tvbox/docker/Dockerfile`
`/home/harold/workspace/alist-tvbox/docker/Dockerfile-xiaoya`
`/home/harold/workspace/alist-tvbox/docker/Dockerfile-host`
`/home/harold/workspace/alist-tvbox/docker/Dockerfile-alist-native`
`/home/harold/workspace/alist-tvbox/docker/Dockerfile-native`
`/home/harold/workspace/alist-tvbox/docker/Dockerfile-native-host`
: Modify all active Dockerfiles to copy `/usr/local/bin/tg-provider` and `/tg-provider-runtime.sh`.

---

### Task 1: Add `telegram-search` Release Workflow

**Files:**
- Create: `/home/harold/workspace/telegram-search/.github/workflows/release.yaml`

- [ ] **Step 1: Create workflow directory**

Run:

```bash
mkdir -p .github/workflows
```

Expected: `.github/workflows` exists in `/home/harold/workspace/telegram-search`.

- [ ] **Step 2: Add release workflow**

Create `/home/harold/workspace/telegram-search/.github/workflows/release.yaml` with this content:

```yaml
name: release tg-provider

on:
  push:
    tags:
      - "v*"
  workflow_dispatch:
    inputs:
      tag:
        description: "Release tag, for example v0.1.0"
        required: true
        type: string

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: Test
        run: go test ./...

      - name: Build release assets
        run: |
          set -euo pipefail
          mkdir -p dist
          CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/tg-provider-linux-amd64 ./cmd/tg-provider
          CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o dist/tg-provider-linux-arm64 ./cmd/tg-provider
          cd dist
          sha256sum tg-provider-linux-amd64 tg-provider-linux-arm64 > checksums.txt
          cat checksums.txt

      - name: Resolve release tag
        id: release-tag
        run: |
          set -euo pipefail
          if [ "${{ github.event_name }}" = "workflow_dispatch" ]; then
            tag="${{ inputs.tag }}"
          else
            tag="${GITHUB_REF_NAME}"
          fi
          echo "tag=${tag}" >> "$GITHUB_OUTPUT"
          echo "Release tag: ${tag}"

      - name: Publish GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          tag_name: ${{ steps.release-tag.outputs.tag }}
          target_commitish: ${{ github.sha }}
          make_latest: true
          fail_on_unmatched_files: true
          files: |
            dist/tg-provider-linux-amd64
            dist/tg-provider-linux-arm64
            dist/checksums.txt
```

- [ ] **Step 3: Validate YAML shape**

Run:

```bash
python - <<'PY'
from pathlib import Path
path = Path(".github/workflows/release.yaml")
text = path.read_text()
assert "name: release tg-provider" in text
assert "uses: softprops/action-gh-release@v2" in text
assert "dist/tg-provider-linux-amd64" in text
assert "dist/tg-provider-linux-arm64" in text
assert "dist/checksums.txt" in text
print("release workflow yaml ok")
PY
```

Expected: prints `release workflow yaml ok`.

- [ ] **Step 4: Run provider tests**

Run:

```bash
GOCACHE=/tmp/go-build-cache go test ./...
```

Expected: all packages pass.

- [ ] **Step 5: Build local release assets**

Run:

```bash
rm -rf /tmp/tg-provider-release-test
mkdir -p /tmp/tg-provider-release-test
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o /tmp/tg-provider-release-test/tg-provider-linux-amd64 ./cmd/tg-provider
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o /tmp/tg-provider-release-test/tg-provider-linux-arm64 ./cmd/tg-provider
cd /tmp/tg-provider-release-test
sha256sum tg-provider-linux-amd64 tg-provider-linux-arm64 > checksums.txt
sha256sum -c checksums.txt
```

Expected: both checksum lines end with `OK`.

- [ ] **Step 6: Commit release workflow**

Run:

```bash
git add .github/workflows/release.yaml
git commit -m "ci: publish tg provider release assets"
```

Expected: one commit containing only the new release workflow.

---

### Task 2: Add `alist-tvbox` Latest Release Download Step

**Files:**
- Modify: `/home/harold/workspace/alist-tvbox/.github/workflows/build.yaml`

- [ ] **Step 1: Insert download step after `Set APP version`**

In `/home/harold/workspace/alist-tvbox/.github/workflows/build.yaml`, add this step immediately after the existing `Set APP version` step and before `Build xiaoya docker and push`:

```yaml
      - name: Download tg-provider latest release
        env:
          GH_TOKEN: ${{ github.token }}
        run: |
          set -euo pipefail
          rm -rf build/tg-provider
          mkdir -p build/tg-provider/download build/tg-provider/linux-amd64 build/tg-provider/linux-arm64
          tag="$(gh release view --repo power721/telegram-search --json tagName --jq .tagName)"
          echo "${tag}" > build/tg-provider/version
          gh release download "${tag}" \
            --repo power721/telegram-search \
            --dir build/tg-provider/download \
            --pattern tg-provider-linux-amd64 \
            --pattern tg-provider-linux-arm64 \
            --pattern checksums.txt \
            --clobber
          cd build/tg-provider/download
          sha256sum -c checksums.txt
          cd ../../..
          install -m 0755 build/tg-provider/download/tg-provider-linux-amd64 build/tg-provider/linux-amd64/tg-provider
          install -m 0755 build/tg-provider/download/tg-provider-linux-arm64 build/tg-provider/linux-arm64/tg-provider
          echo "tg-provider release: $(cat build/tg-provider/version)"
          ls -l build/tg-provider/linux-amd64/tg-provider build/tg-provider/linux-arm64/tg-provider
```

- [ ] **Step 2: Validate YAML shape**

Run from `/home/harold/workspace/alist-tvbox`:

```bash
python - <<'PY'
from pathlib import Path
path = Path(".github/workflows/build.yaml")
text = path.read_text()
set_idx = text.index("- name: Set APP version")
download_idx = text.index("- name: Download tg-provider latest release")
build_idx = text.index("- name: Build xiaoya docker and push")
assert set_idx < download_idx < build_idx
assert "gh release view --repo power721/telegram-search" in text
assert "sha256sum -c checksums.txt" in text
print("build workflow yaml ok")
PY
```

Expected: prints `build workflow yaml ok`.

- [ ] **Step 3: Commit workflow change**

Run from `/home/harold/workspace/alist-tvbox`:

```bash
git add .github/workflows/build.yaml
git commit -m "ci: download latest tg provider release"
```

Expected: one commit containing only `build.yaml`.

---

### Task 3: Add Shared Provider Runtime Helper

**Files:**
- Create: `/home/harold/workspace/alist-tvbox/scripts/tg-provider-runtime.sh`

- [ ] **Step 1: Add helper script**

Create `/home/harold/workspace/alist-tvbox/scripts/tg-provider-runtime.sh` with this content:

```sh
#!/bin/sh

TG_PROVIDER_BIN="${TG_PROVIDER_BIN:-/usr/local/bin/tg-provider}"
TG_PROVIDER_DATA="${TG_PROVIDER_DATA:-/data/tg-provider}"
TG_PROVIDER_CONFIG="${TG_PROVIDER_CONFIG:-$TG_PROVIDER_DATA/config.yaml}"
TG_PROVIDER_HOST="${TG_PROVIDER_HOST:-127.0.0.1}"
TG_PROVIDER_PORT="${TG_PROVIDER_PORT:-9900}"
TG_PROVIDER_LOG_DIR="${TG_PROVIDER_LOG_DIR:-$TG_PROVIDER_DATA/logs}"
TG_PROVIDER_PID=""
TG_PROVIDER_STARTED=0
ATV_APP_PID=""

tg_provider_log() {
  echo "[tg-provider] $*"
}

tg_provider_prepare_config() {
  mkdir -p "$TG_PROVIDER_DATA" "$TG_PROVIDER_DATA/sessions" "$TG_PROVIDER_DATA/backup" "$TG_PROVIDER_LOG_DIR"

  if [ -r "$TG_PROVIDER_CONFIG" ]; then
    tg_provider_log "Using config $TG_PROVIDER_CONFIG"
    return 0
  fi

  api_id="${API_ID:-${TG_API_ID:-26375241}}"
  api_hash="${API_HASH:-${TG_API_HASH:-70f574f48a016d683c64f2f7a217d04f}}"

  old_umask="$(umask)"
  umask 077
  cat > "$TG_PROVIDER_CONFIG" <<EOF
telegram:
  api_id: ${api_id}
  api_hash: ${api_hash}
server:
  host: ${TG_PROVIDER_HOST}
  port: ${TG_PROVIDER_PORT}
sync:
  workers: 5
  history_batch_size: 100
storage:
  path: ${TG_PROVIDER_DATA}
EOF
  umask "$old_umask"
  tg_provider_log "Generated config $TG_PROVIDER_CONFIG"
}

tg_provider_start() {
  if [ ! -x "$TG_PROVIDER_BIN" ]; then
    tg_provider_log "Binary not found or not executable: $TG_PROVIDER_BIN"
    return 1
  fi

  tg_provider_prepare_config || return 1

  "$TG_PROVIDER_BIN" -config "$TG_PROVIDER_CONFIG" >> "$TG_PROVIDER_LOG_DIR/stdout.log" 2>> "$TG_PROVIDER_LOG_DIR/stderr.log" &
  TG_PROVIDER_PID="$!"
  TG_PROVIDER_STARTED=1
  tg_provider_log "Started pid $TG_PROVIDER_PID on ${TG_PROVIDER_HOST}:${TG_PROVIDER_PORT}"
}

tg_provider_stop_pid() {
  pid="$1"
  name="$2"
  if [ -z "$pid" ]; then
    return 0
  fi
  if ! kill -0 "$pid" 2>/dev/null; then
    return 0
  fi

  tg_provider_log "Stopping $name pid $pid"
  kill "$pid" 2>/dev/null || true
  for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
    if ! kill -0 "$pid" 2>/dev/null; then
      return 0
    fi
    sleep 1
  done
  tg_provider_log "Force stopping $name pid $pid"
  kill -9 "$pid" 2>/dev/null || true
}

tg_provider_shutdown() {
  status="${1:-0}"
  tg_provider_stop_pid "$ATV_APP_PID" "alist-tvbox"
  tg_provider_stop_pid "$TG_PROVIDER_PID" "tg-provider"
  exit "$status"
}

tg_provider_run_app() {
  "$@" &
  ATV_APP_PID="$!"
  trap 'tg_provider_shutdown 143' INT TERM

  while true; do
    if ! kill -0 "$ATV_APP_PID" 2>/dev/null; then
      wait "$ATV_APP_PID"
      status="$?"
      tg_provider_stop_pid "$TG_PROVIDER_PID" "tg-provider"
      exit "$status"
    fi

    if [ "$TG_PROVIDER_STARTED" = "1" ] && ! kill -0 "$TG_PROVIDER_PID" 2>/dev/null; then
      wait "$TG_PROVIDER_PID" 2>/dev/null
      status="$?"
      tg_provider_log "Exited with status $status"
      tg_provider_stop_pid "$ATV_APP_PID" "alist-tvbox"
      exit 1
    fi

    sleep 2
  done
}
```

- [ ] **Step 2: Validate shell syntax**

Run from `/home/harold/workspace/alist-tvbox`:

```bash
/bin/sh -n scripts/tg-provider-runtime.sh
```

Expected: no output and exit code `0`.

- [ ] **Step 3: Commit helper script**

Run from `/home/harold/workspace/alist-tvbox`:

```bash
git add scripts/tg-provider-runtime.sh
git commit -m "feat: add tg provider runtime helper"
```

Expected: one commit containing only the helper script.

---

### Task 4: Wire Helper Into Entrypoints

**Files:**
- Modify: `/home/harold/workspace/alist-tvbox/scripts/entrypoint.sh`
- Modify: `/home/harold/workspace/alist-tvbox/scripts/entrypoint-native.sh`
- Modify: `/home/harold/workspace/alist-tvbox/entrypoint.sh`
- Modify: `/home/harold/workspace/alist-tvbox/entrypoint-native.sh`

- [ ] **Step 1: Update alist JVM entrypoint**

In `/home/harold/workspace/alist-tvbox/scripts/entrypoint.sh`, replace the final Java line:

```sh
/jre/bin/java "$MEM_OPT" -cp BOOT-INF/classes:BOOT-INF/lib/* cn.har01d.alist_tvbox.AListApplication "$@"
```

with:

```sh
. /tg-provider-runtime.sh
tg_provider_start || exit 1
tg_provider_run_app /jre/bin/java "$MEM_OPT" -cp BOOT-INF/classes:BOOT-INF/lib/* cn.har01d.alist_tvbox.AListApplication "$@"
```

- [ ] **Step 2: Update alist native entrypoint**

In `/home/harold/workspace/alist-tvbox/scripts/entrypoint-native.sh`, replace the final native app line:

```sh
./atv "$@"
```

with:

```sh
. /tg-provider-runtime.sh
tg_provider_start || exit 1
tg_provider_run_app ./atv "$@"
```

- [ ] **Step 3: Update xiaoya JVM entrypoint**

In `/home/harold/workspace/alist-tvbox/entrypoint.sh`, keep the existing `httpd`, `nginx`, and `shift` lines. Replace the final Java line:

```sh
/jre/bin/java "$MEM_OPT" -cp BOOT-INF/classes:BOOT-INF/lib/* cn.har01d.alist_tvbox.AListApplication "$@"
```

with:

```sh
. /tg-provider-runtime.sh
tg_provider_start || exit 1
tg_provider_run_app /jre/bin/java "$MEM_OPT" -cp BOOT-INF/classes:BOOT-INF/lib/* cn.har01d.alist_tvbox.AListApplication "$@"
```

- [ ] **Step 4: Update xiaoya native entrypoint**

In `/home/harold/workspace/alist-tvbox/entrypoint-native.sh`, keep the existing `httpd`, `nginx`, and `shift` lines. Replace the final native app line:

```sh
./atv "$@"
```

with:

```sh
. /tg-provider-runtime.sh
tg_provider_start || exit 1
tg_provider_run_app ./atv "$@"
```

- [ ] **Step 5: Validate shell syntax**

Run from `/home/harold/workspace/alist-tvbox`:

```bash
/bin/sh -n scripts/entrypoint.sh
/bin/sh -n scripts/entrypoint-native.sh
/bin/sh -n entrypoint.sh
/bin/sh -n entrypoint-native.sh
```

Expected: no output and exit code `0`.

- [ ] **Step 6: Commit entrypoint wiring**

Run from `/home/harold/workspace/alist-tvbox`:

```bash
git add scripts/entrypoint.sh scripts/entrypoint-native.sh entrypoint.sh entrypoint-native.sh
git commit -m "feat: run tg provider from entrypoints"
```

Expected: one commit containing only the four entrypoint edits.

---

### Task 5: Package Provider Into Active Dockerfiles

**Files:**
- Modify: `/home/harold/workspace/alist-tvbox/docker/Dockerfile`
- Modify: `/home/harold/workspace/alist-tvbox/docker/Dockerfile-xiaoya`
- Modify: `/home/harold/workspace/alist-tvbox/docker/Dockerfile-host`
- Modify: `/home/harold/workspace/alist-tvbox/docker/Dockerfile-alist-native`
- Modify: `/home/harold/workspace/alist-tvbox/docker/Dockerfile-native`
- Modify: `/home/harold/workspace/alist-tvbox/docker/Dockerfile-native-host`

- [ ] **Step 1: Add `ARG TARGETARCH` to each active Dockerfile**

In each Dockerfile listed above, add this line after the `LABEL MAINTAINER="Har01d"` line:

```dockerfile
ARG TARGETARCH
```

- [ ] **Step 2: Copy provider binary and helper to each active Dockerfile**

In each Dockerfile listed above, add these lines after the existing entrypoint/init copy lines:

```dockerfile
COPY build/tg-provider/linux-${TARGETARCH}/tg-provider /usr/local/bin/tg-provider
COPY scripts/tg-provider-runtime.sh /tg-provider-runtime.sh
RUN chmod +x /usr/local/bin/tg-provider /tg-provider-runtime.sh
```

For `docker/Dockerfile`, place the block after:

```dockerfile
COPY scripts/entrypoint.sh /
COPY scripts/init.sh /
```

For `docker/Dockerfile-xiaoya` and `docker/Dockerfile-host`, place the block after:

```dockerfile
COPY entrypoint.sh /
COPY init.sh /
```

For `docker/Dockerfile-alist-native`, place the block after:

```dockerfile
COPY scripts/entrypoint-native.sh /entrypoint.sh
COPY scripts/index.sh /
COPY scripts/init.sh /
```

For `docker/Dockerfile-native` and `docker/Dockerfile-native-host`, place the block after:

```dockerfile
COPY entrypoint-native.sh /entrypoint.sh
COPY init.sh /
```

- [ ] **Step 3: Verify all active Dockerfiles reference the provider**

Run from `/home/harold/workspace/alist-tvbox`:

```bash
for file in docker/Dockerfile docker/Dockerfile-xiaoya docker/Dockerfile-host docker/Dockerfile-alist-native docker/Dockerfile-native docker/Dockerfile-native-host; do
  grep -q 'ARG TARGETARCH' "$file"
  grep -q 'build/tg-provider/linux-${TARGETARCH}/tg-provider' "$file"
  grep -q 'scripts/tg-provider-runtime.sh' "$file"
done
echo "active dockerfiles package tg-provider"
```

Expected: prints `active dockerfiles package tg-provider`.

- [ ] **Step 4: Commit Dockerfile changes**

Run from `/home/harold/workspace/alist-tvbox`:

```bash
git add docker/Dockerfile docker/Dockerfile-xiaoya docker/Dockerfile-host docker/Dockerfile-alist-native docker/Dockerfile-native docker/Dockerfile-native-host
git commit -m "feat: package tg provider in active images"
```

Expected: one commit containing only the six Dockerfile edits.

---

### Task 6: End-to-End Verification

**Files:**
- Read/verify only.

- [ ] **Step 1: Verify `telegram-search` remains clean and passing**

Run from `/home/harold/workspace/telegram-search`:

```bash
GOCACHE=/tmp/go-build-cache go test ./...
git status --short
```

Expected: tests pass. `git status --short` is empty after all planned commits.

- [ ] **Step 2: Prepare local provider assets for AList-TVBox Docker build checks**

Run from `/home/harold/workspace/telegram-search`:

```bash
rm -rf /tmp/tg-provider-release-test
mkdir -p /tmp/tg-provider-release-test
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o /tmp/tg-provider-release-test/tg-provider-linux-amd64 ./cmd/tg-provider
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o /tmp/tg-provider-release-test/tg-provider-linux-arm64 ./cmd/tg-provider
```

Expected: both binaries build.

- [ ] **Step 3: Place local assets into AList-TVBox build context**

Run from `/home/harold/workspace/alist-tvbox`:

```bash
rm -rf build/tg-provider
mkdir -p build/tg-provider/linux-amd64 build/tg-provider/linux-arm64
install -m 0755 /tmp/tg-provider-release-test/tg-provider-linux-amd64 build/tg-provider/linux-amd64/tg-provider
install -m 0755 /tmp/tg-provider-release-test/tg-provider-linux-arm64 build/tg-provider/linux-arm64/tg-provider
```

Expected: both provider binaries exist in the Docker build context.

- [ ] **Step 4: Validate shell scripts**

Run from `/home/harold/workspace/alist-tvbox`:

```bash
/bin/sh -n scripts/tg-provider-runtime.sh
/bin/sh -n scripts/entrypoint.sh
/bin/sh -n scripts/entrypoint-native.sh
/bin/sh -n entrypoint.sh
/bin/sh -n entrypoint-native.sh
```

Expected: no output and exit code `0`.

- [ ] **Step 5: Validate workflow YAML**

Run from `/home/harold/workspace/alist-tvbox`:

```bash
python - <<'PY'
from pathlib import Path
path = Path(".github/workflows/build.yaml")
text = path.read_text()
assert "- name: Download tg-provider latest release" in text
assert "gh release download" in text
assert "sha256sum -c checksums.txt" in text
print("alist-tvbox build workflow yaml ok")
PY
```

Expected: prints `alist-tvbox build workflow yaml ok`.

- [ ] **Step 6: Optionally build one JVM image locally**

Run from `/home/harold/workspace/alist-tvbox` if Docker is available and base image access works:

```bash
docker build --build-arg TARGETARCH=amd64 -f docker/Dockerfile -t alist-tvbox:tg-provider-local .
```

Expected: image builds and includes `/usr/local/bin/tg-provider`.

- [ ] **Step 7: Optionally inspect packaged binary**

Run if Step 6 succeeded:

```bash
docker run --rm --entrypoint /bin/sh alist-tvbox:tg-provider-local -c 'test -x /usr/local/bin/tg-provider && /usr/local/bin/tg-provider -h >/dev/null 2>&1 || true; ls -l /usr/local/bin/tg-provider /tg-provider-runtime.sh'
```

Expected: lists executable `/usr/local/bin/tg-provider` and `/tg-provider-runtime.sh`.

- [ ] **Step 8: Check AList-TVBox git status**

Run from `/home/harold/workspace/alist-tvbox`:

```bash
git status --short
```

Expected: only pre-existing unrelated untracked files may remain. The files from this plan are committed.
