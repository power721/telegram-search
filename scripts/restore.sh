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
