#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$ROOT_DIR/scripts/install-docker.sh"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

mkdir -p "$TMP_DIR/bin"

cat > "$TMP_DIR/bin/docker" <<'MOCK_DOCKER'
#!/usr/bin/env bash
set -euo pipefail

printf 'docker'
for arg in "$@"; do
  printf ' %s' "$arg"
done
printf '\n'
MOCK_DOCKER
chmod +x "$TMP_DIR/bin/docker"

cat > "$TMP_DIR/bin/xdg-open" <<'MOCK_XDG_OPEN'
#!/usr/bin/env bash
set -euo pipefail

printf 'xdg-open'
for arg in "$@"; do
  printf ' %s' "$arg"
done
printf '\n'

{
  printf 'xdg-open'
  for arg in "$@"; do
    printf ' %s' "$arg"
  done
  printf '\n'
} > "$OPEN_LOG"
MOCK_XDG_OPEN
chmod +x "$TMP_DIR/bin/xdg-open"

OUTPUT="$(
  OPEN_LOG="$TMP_DIR/open.log" PATH="$TMP_DIR/bin:$PATH" "$SCRIPT" 2>&1
)"

assert_contains() {
  local needle="$1"
  if [[ "$OUTPUT" != *"$needle"* ]]; then
    printf 'expected output to contain: %s\n' "$needle" >&2
    printf 'actual output:\n%s\n' "$OUTPUT" >&2
    exit 1
  fi
}

assert_contains "docker pull haroldli/tg-search:latest"
assert_contains "docker rm -f tg-search"
assert_contains "docker run -d --name tg-search --restart unless-stopped -p 9900:9900 -v tg-search-data:/data/tg-search haroldli/tg-search:latest"
assert_contains "tg-search is running: http://127.0.0.1:9900"

for _ in {1..20}; do
  if [[ -e "$TMP_DIR/open.log" ]]; then
    break
  fi
  sleep 0.1
done

if [[ "$(cat "$TMP_DIR/open.log")" != "xdg-open http://127.0.0.1:9900" ]]; then
  printf 'expected browser open command for default URL\n' >&2
  printf 'actual open log:\n%s\n' "$(cat "$TMP_DIR/open.log" 2>/dev/null || true)" >&2
  exit 1
fi

OUTPUT_WITH_CUSTOM_PORT="$(
  rm -f "$TMP_DIR/open.log"
  OPEN_LOG="$TMP_DIR/open.log" PATH="$TMP_DIR/bin:$PATH" "$SCRIPT" -p 9911 --no-open 2>&1
)"

if [[ "$OUTPUT_WITH_CUSTOM_PORT" != *"docker run -d --name tg-search --restart unless-stopped -p 9911:9900 -v tg-search-data:/data/tg-search haroldli/tg-search:latest"* ]]; then
  printf 'expected custom host port in docker run command\n' >&2
  printf 'actual output:\n%s\n' "$OUTPUT_WITH_CUSTOM_PORT" >&2
  exit 1
fi

if [[ -e "$TMP_DIR/open.log" ]]; then
  printf 'expected --no-open to skip opening a browser\n' >&2
  printf 'actual open log:\n%s\n' "$(cat "$TMP_DIR/open.log")" >&2
  exit 1
fi
