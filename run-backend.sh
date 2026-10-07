#!/usr/bin/env bash
set -euo pipefail
umask 077
cd "$(dirname "$0")"
if [ -f .env ]; then
  chmod 600 .env
  set -a
  . ./.env
  set +a
fi
mkdir -p data storage
# Resolve configured paths before entering the Go module directory.
DB_PATH="${DB_PATH:-./data/trynas.db}"
case "$DB_PATH" in /*) ;; *) DB_PATH="$PWD/$DB_PATH" ;; esac
export DB_PATH
STORAGE_ROOT="${STORAGE_ROOT:-./storage}"
case "$STORAGE_ROOT" in /*) ;; *) STORAGE_ROOT="$PWD/$STORAGE_ROOT" ;; esac
export STORAGE_ROOT
cd backend
exec go run .
