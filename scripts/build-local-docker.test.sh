#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$ROOT_DIR/scripts/build-local-docker.sh"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

mkdir -p "$TMP_DIR/bin" "$TMP_DIR/data"

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

cat > "$TMP_DIR/bin/ip" <<'MOCK_IP'
#!/usr/bin/env bash
set -euo pipefail

cat <<'IP_OUTPUT'
2: eth0    inet 192.168.1.20/24 brd 192.168.1.255 scope global eth0
IP_OUTPUT
MOCK_IP
chmod +x "$TMP_DIR/bin/ip"

OUTPUT="$(
  API_ID=12345 API_HASH=test_hash PATH="$TMP_DIR/bin:$PATH" "$SCRIPT" -d "$TMP_DIR/data" -p 7000 2>&1
)"

assert_contains() {
  local needle="$1"
  if [[ "$OUTPUT" != *"$needle"* ]]; then
    printf 'expected output to contain: %s\n' "$needle" >&2
    printf 'actual output:\n%s\n' "$OUTPUT" >&2
    exit 1
  fi
}

assert_contains "docker build -f $ROOT_DIR/Dockerfile --build-arg VERSION="
assert_contains "--tag=haroldli/tg-search:latest $ROOT_DIR"
assert_contains "=== generate $TMP_DIR/data/config.yaml ==="
assert_contains "docker rm -f tg-search"
assert_contains "docker run -d -p 7000:9900 -e TZ=Asia/Shanghai -v $TMP_DIR/data:/data/tg-search --restart=unless-stopped --name=tg-search haroldli/tg-search:latest"
assert_contains "docker logs -f tg-search"
assert_contains "http://192.168.1.20:7000/"

if ! grep -q "api_id: 12345" "$TMP_DIR/data/config.yaml"; then
  printf 'generated config missing API_ID\n' >&2
  cat "$TMP_DIR/data/config.yaml" >&2
  exit 1
fi

if ! grep -q "host: 0.0.0.0" "$TMP_DIR/data/config.yaml"; then
  printf 'generated config must bind to 0.0.0.0 for Docker port publishing\n' >&2
  cat "$TMP_DIR/data/config.yaml" >&2
  exit 1
fi

EMPTY_DATA="$TMP_DIR/empty-data"
mkdir -p "$EMPTY_DATA"
OUTPUT_WITH_IMAGE_DEFAULT="$(
  PATH="$TMP_DIR/bin:$PATH" "$SCRIPT" -d "$EMPTY_DATA" -p 7001 2>&1
)"

if [[ "$OUTPUT_WITH_IMAGE_DEFAULT" != *"=== use image default /app/config.yaml ==="* ]]; then
  printf 'expected script to continue with image default config\n' >&2
  printf 'actual output:\n%s\n' "$OUTPUT_WITH_IMAGE_DEFAULT" >&2
  exit 1
fi

if [[ -e "$EMPTY_DATA/config.yaml" ]]; then
  printf 'script should not create data config when API credentials are absent\n' >&2
  cat "$EMPTY_DATA/config.yaml" >&2
  exit 1
fi
