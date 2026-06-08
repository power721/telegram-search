#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORKFLOW="$ROOT_DIR/.github/workflows/release.yml"
DOCKERFILE="$ROOT_DIR/Dockerfile.ci"
DOCKERIGNORE="$ROOT_DIR/Dockerfile.ci.dockerignore"
DEFAULT_CONFIG="$ROOT_DIR/docker/config.yaml"

assert_file_contains() {
  local file="$1"
  local needle="$2"

  if [[ ! -f "$file" ]]; then
    printf 'missing file: %s\n' "$file" >&2
    exit 1
  fi

  if ! grep -Fq "$needle" "$file"; then
    printf 'expected %s to contain: %s\n' "$file" "$needle" >&2
    exit 1
  fi
}

assert_file_contains "$WORKFLOW" "dist/linux/amd64/tg-search"
assert_file_contains "$WORKFLOW" "dist/linux/arm64/tg-search"
assert_file_contains "$WORKFLOW" "file: Dockerfile.ci"
assert_file_contains "$DOCKERFILE" "ARG TARGETARCH"
assert_file_contains "$DOCKERFILE" 'COPY --chmod=755 dist/linux/${TARGETARCH}/tg-search /usr/local/bin/tg-search'
assert_file_contains "$DOCKERFILE" "COPY docker/config.yaml /app/config.yaml"
assert_file_contains "$DOCKERIGNORE" "**"
assert_file_contains "$DOCKERIGNORE" "!docker/"
assert_file_contains "$DOCKERIGNORE" "!docker/config.yaml"
assert_file_contains "$DOCKERIGNORE" "!dist/linux/amd64/tg-search"
assert_file_contains "$DOCKERIGNORE" "!dist/linux/arm64/tg-search"
assert_file_contains "$DEFAULT_CONFIG" "host: 0.0.0.0"
assert_file_contains "$DEFAULT_CONFIG" "path: /data/tg-search"
