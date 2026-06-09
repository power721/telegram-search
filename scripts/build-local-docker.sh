#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE[0]:-$0}"
SCRIPT_DIR="$(cd "$(dirname "$SCRIPT_PATH")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

IMAGE="haroldli/tg-search:latest"
CONTAINER_NAME="tg-search"
DATA_DIR="$ROOT_DIR/data"
PORT=9900
TZ_VALUE="${TZ:-Asia/Shanghai}"
CONFIG_SOURCE=""
EXTRA_MOUNTS=()
LOCAL_DOCKERFILE="$ROOT_DIR/Dockerfile.local"
LOCAL_BINARY="$ROOT_DIR/dist/local/tg-search"
BUILD_DIR=""

cleanup() {
  if [[ -n "$BUILD_DIR" ]]; then
    rm -rf "$BUILD_DIR"
  fi
}
trap cleanup EXIT

usage() {
  cat <<EOF
Usage: $0 [-d data_dir] [-p port] [-c config.yaml] [-v host_path:container_path]

Build haroldli/tg-search:latest locally, restart the tg-search container,
and follow container logs.

Options:
  -d  Host data directory mounted to /data/tg-search. Default: $ROOT_DIR/data
  -p  Host port mapped to container port 9900. Default: 9900
  -c  Config file to copy into the data directory before starting.
  -v  Extra Docker volume mount. Can be specified multiple times.
  -h  Show this help.
EOF
}

require_cmd() {
  local cmd="$1"

  if ! command -v "$cmd" >/dev/null 2>&1; then
    printf 'required command not found: %s\n' "$cmd" >&2
    exit 1
  fi
}

abs_path() {
  local path="$1"
  local dir
  local base

  if [[ "$path" = /* ]]; then
    printf '%s\n' "$path"
    return
  fi

  dir="$(dirname "$path")"
  base="$(basename "$path")"
  printf '%s/%s\n' "$(cd "$dir" && pwd)" "$base"
}

prepare_config() {
  local target="$DATA_DIR/config.yaml"

  if [[ -f "$target" ]]; then
    echo "=== use existing $target ==="
    return
  fi

  if [[ -n "$CONFIG_SOURCE" ]]; then
    if [[ ! -f "$CONFIG_SOURCE" ]]; then
      printf 'config file not found: %s\n' "$CONFIG_SOURCE" >&2
      exit 1
    fi
    echo "=== copy $CONFIG_SOURCE to $target ==="
    cp "$CONFIG_SOURCE" "$target"
    return
  fi

  if [[ -f "$ROOT_DIR/config.yaml" ]]; then
    echo "=== copy $ROOT_DIR/config.yaml to $target ==="
    cp "$ROOT_DIR/config.yaml" "$target"
    return
  fi

  echo "=== use image default /app/config.yaml ==="
}

target_arch() {
  local platform="${DOCKER_PLATFORM:-${DOCKER_DEFAULT_PLATFORM:-}}"

  case "$platform" in
    linux/amd64 | linux/amd64/*)
      printf 'amd64\n'
      return
      ;;
    linux/arm64 | linux/arm64/*)
      printf 'arm64\n'
      return
      ;;
    "" )
      go env GOARCH
      return
      ;;
    * )
      printf 'unsupported Docker platform for local Go build: %s\n' "$platform" >&2
      printf 'set TARGETARCH=amd64 or TARGETARCH=arm64 to override.\n' >&2
      exit 1
      ;;
  esac
}

ensure_frontend_deps() {
  if [[ -d "$ROOT_DIR/web/node_modules" ]]; then
    return
  fi

  echo "=== install frontend dependencies locally ==="
  npm ci --prefix "$ROOT_DIR/web"
}

build_frontend() {
  require_cmd npm
  ensure_frontend_deps

  echo "=== build frontend locally ==="
  (
    cd "$ROOT_DIR"
    npm run web:build
  )
}

stage_go_source() {
  BUILD_DIR="$(mktemp -d "${TMPDIR:-/tmp}/tg-search-local-build.XXXXXX")"

  tar -C "$ROOT_DIR" \
    --exclude='.git' \
    --exclude='.worktrees' \
    --exclude='data' \
    --exclude='dist' \
    --exclude='web/node_modules' \
    --exclude='web/dist' \
    --exclude='web/.vite' \
    --exclude='web/*.tsbuildinfo' \
    -cf - . | tar -C "$BUILD_DIR" -xf -

  rm -rf "$BUILD_DIR/internal/web/dist"
  mkdir -p "$BUILD_DIR/internal/web"
  cp -R "$ROOT_DIR/web/dist" "$BUILD_DIR/internal/web/dist"
}

build_local_binary() {
  local arch="${TARGETARCH:-}"

  require_cmd go

  if [[ -z "$arch" ]]; then
    arch="$(target_arch)"
  fi

  if [[ "$arch" != "amd64" && "$arch" != "arm64" ]]; then
    printf 'unsupported TARGETARCH for Docker image: %s\n' "$arch" >&2
    exit 1
  fi

  build_frontend
  stage_go_source

  mkdir -p "$(dirname "$LOCAL_BINARY")"

  echo "=== build linux/$arch binary locally ==="
  (
    cd "$BUILD_DIR"
    CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -buildvcs=false -ldflags "-s -w -X 'tg-search/internal/build.Version=${VERSION}'" -o "$LOCAL_BINARY" ./cmd/tg-search
  )
}

while getopts ":d:p:c:v:h" arg; do
  case "$arg" in
    d)
      DATA_DIR="$OPTARG"
      ;;
    p)
      PORT="$OPTARG"
      ;;
    c)
      CONFIG_SOURCE="$OPTARG"
      ;;
    v)
      EXTRA_MOUNTS+=("-v" "$OPTARG")
      ;;
    h)
      usage
      exit 0
      ;;
    *)
      usage >&2
      exit 2
      ;;
  esac
done

shift $((OPTIND - 1))

if [[ $# -gt 0 ]]; then
  DATA_DIR="$1"
fi

if [[ $# -gt 1 ]]; then
  PORT="$2"
fi

mkdir -p "$DATA_DIR"
DATA_DIR="$(abs_path "$DATA_DIR")"
if [[ -n "$CONFIG_SOURCE" ]]; then
  CONFIG_SOURCE="$(abs_path "$CONFIG_SOURCE")"
fi

prepare_config

VERSION="${VERSION:-$(git -C "$ROOT_DIR" describe --tags --always)}"

build_local_binary

echo "=== build $IMAGE ==="
docker build -f "$LOCAL_DOCKERFILE" --tag="$IMAGE" "$ROOT_DIR"

echo "=== restart $CONTAINER_NAME ==="
docker rm -f "$CONTAINER_NAME" 2>/dev/null || true
docker run -d \
  -p "$PORT:9900" \
  -e "TZ=$TZ_VALUE" \
  -v "$DATA_DIR:/data/tg-search" \
  "${EXTRA_MOUNTS[@]}" \
  --restart=unless-stopped \
  --name="$CONTAINER_NAME" \
  "$IMAGE"

sleep 1

IP="$(ip a | awk '{ for (i = 1; i <= NF; i++) if ($i == "inet" && $(i + 1) ~ /^192\.168\./) { split($(i + 1), parts, "/"); print parts[1]; exit } }')"
if [[ -z "$IP" ]]; then
  IP="$(ip a | awk '{ for (i = 1; i <= NF; i++) if ($i == "inet" && $(i + 1) ~ /^10\./) { split($(i + 1), parts, "/"); print parts[1]; exit } }')"
fi

if [[ -n "$IP" ]]; then
  echo ""
  echo -e "\e[32m请用以下地址访问：\e[0m"
  echo -e "    \e[32m管理界面\e[0m： http://$IP:$PORT/"
else
  echo -e "\e[32m云服务器请用公网IP访问，端口：$PORT\e[0m"
fi
echo ""

docker logs -f "$CONTAINER_NAME"
