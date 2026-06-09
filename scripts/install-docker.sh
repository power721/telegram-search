#!/usr/bin/env bash
set -euo pipefail

IMAGE="haroldli/tg-search:latest"
CONTAINER_NAME="tg-search"
HOST_PORT=9900
CONTAINER_PORT=9900
DATA_VOLUME="tg-search-data"
OPEN_BROWSER=1

usage() {
  cat <<EOF
Usage: $0 [-p host_port] [--no-open]

Pull and start tg-search with Docker, then open the admin shell.

Options:
  -p, --port      Host port mapped to container port 9900. Default: 9900
  --no-open      Do not open a browser after starting the container.
  -h, --help     Show this help.
EOF
}

require_cmd() {
  local cmd="$1"

  if ! command -v "$cmd" >/dev/null 2>&1; then
    printf 'required command not found: %s\n' "$cmd" >&2
    exit 1
  fi
}

open_browser() {
  local url="$1"

  case "$(uname -s 2>/dev/null || printf unknown)" in
    Darwin)
      if command -v open >/dev/null 2>&1; then
        open "$url" >/dev/null 2>&1 &
        return
      fi
      ;;
    MINGW* | MSYS* | CYGWIN*)
      if command -v cmd.exe >/dev/null 2>&1; then
        cmd.exe /c start "" "$url" >/dev/null 2>&1 &
        return
      fi
      ;;
    Linux)
      if command -v xdg-open >/dev/null 2>&1; then
        xdg-open "$url" >/dev/null 2>&1 &
        return
      fi

      if command -v wslview >/dev/null 2>&1; then
        wslview "$url" >/dev/null 2>&1 &
        return
      fi

      if command -v cmd.exe >/dev/null 2>&1; then
        cmd.exe /c start "" "$url" >/dev/null 2>&1 &
        return
      fi
      ;;
  esac

  printf 'browser command not found; open this URL manually: %s\n' "$url"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -p | --port)
      if [[ $# -lt 2 ]]; then
        usage >&2
        exit 2
      fi
      HOST_PORT="$2"
      shift 2
      ;;
    --no-open)
      OPEN_BROWSER=0
      shift
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      usage >&2
      exit 2
      ;;
  esac
done

require_cmd docker

URL="http://127.0.0.1:$HOST_PORT"

docker pull "$IMAGE"
docker rm -f "$CONTAINER_NAME" 2>/dev/null || true
docker run -d \
  --name "$CONTAINER_NAME" \
  --restart unless-stopped \
  -p "$HOST_PORT:$CONTAINER_PORT" \
  -v "$DATA_VOLUME:/data/tg-search" \
  "$IMAGE"

printf 'tg-search is running: %s\n' "$URL"

if [[ "$OPEN_BROWSER" -eq 1 ]]; then
  open_browser "$URL"
fi
