#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
if [ -f .env ]; then
  set -a
  . ./.env
  set +a
fi
mkdir -p data storage
# Resolve configured paths before entering the Go module directory.
export DB_PATH="$(realpath -m "${DB_PATH:-./data/trynas.db}")"
export STORAGE_ROOT="$(realpath -m "${STORAGE_ROOT:-./storage}")"
cd backend
exec go run .
