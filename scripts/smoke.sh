#!/bin/sh
set -eu

BASE_URL="${BASE_URL:-http://127.0.0.1:6000}"

curl --fail --silent "$BASE_URL/api/health" >/dev/null
curl --fail --silent "$BASE_URL/api/ready" >/dev/null
curl --fail --silent "$BASE_URL/api/setup/status" >/dev/null
curl --fail --silent "$BASE_URL/" >/dev/null

printf 'tg-search smoke checks passed for %s\n' "$BASE_URL"
