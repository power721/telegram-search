#!/bin/sh
set -eu

DATA_DIR="${DATA_DIR:-/data/tg-search}"
BACKUP_DIR="${BACKUP_DIR:-$DATA_DIR/backup}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"

mkdir -p "$BACKUP_DIR"
sqlite3 "$DATA_DIR/tg-search.db" ".backup '$BACKUP_DIR/tg-search-$STAMP.db'"
tar -C "$DATA_DIR" -czf "$BACKUP_DIR/tg-search-$STAMP-sessions.tgz" sessions config.yaml
printf '%s\n' "$BACKUP_DIR/tg-search-$STAMP.db"
