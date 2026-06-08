#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${BASH_SOURCE[0]:-$0}"
SCRIPT_DIR="$(cd "$(dirname "$SCRIPT_PATH")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

IMAGE="haroldli/tg-search:latest"
CONTAINER_NAME="tg-search"
DATA_DIR="$ROOT_DIR/data"
PORT=6000
TZ_VALUE="${TZ:-Asia/Shanghai}"
CONFIG_SOURCE=""
EXTRA_MOUNTS=()

usage() {
  cat <<EOF
Usage: $0 [-d data_dir] [-p port] [-c config.yaml] [-v host_path:container_path]

Build haroldli/tg-search:latest locally, restart the tg-search container,
and follow container logs.

Options:
  -d  Host data directory mounted to /data/tg-search. Default: $ROOT_DIR/data
  -p  Host port mapped to container port 6000. Default: 6000
  -c  Config file to copy into the data directory before starting.
  -v  Extra Docker volume mount. Can be specified multiple times.
  -h  Show this help.
EOF
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
  local api_id="${API_ID:-${TG_API_ID:-}}"
  local api_hash="${API_HASH:-${TG_API_HASH:-}}"

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

  if [[ -z "$api_id" || -z "$api_hash" ]]; then
    echo "=== use image default /app/config.yaml ==="
    return
  fi

  echo "=== generate $target ==="
  cat > "$target" <<EOF
telegram:
  api_id: $api_id
  api_hash: $api_hash
server:
  host: 0.0.0.0
  port: 6000
sync:
  workers: 5
  history_batch_size: 100
storage:
  path: /data/tg-search
  max_db_size: 10GB
  max_media_cache: 20GB
EOF
  chmod 600 "$target"
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

echo "=== build $IMAGE ==="
docker build -f "$ROOT_DIR/Dockerfile" --tag="$IMAGE" "$ROOT_DIR"

echo "=== restart $CONTAINER_NAME ==="
docker rm -f "$CONTAINER_NAME" 2>/dev/null || true
docker run -d \
  -p "$PORT:6000" \
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
