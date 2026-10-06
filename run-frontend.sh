#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
if [ -f .env ]; then
  API_URL=$(grep '^NEXT_PUBLIC_API_URL=' .env | tail -1 | cut -d= -f2- || true)
  if [ -n "${API_URL:-}" ]; then
    printf 'NEXT_PUBLIC_API_URL=%s\n' "$API_URL" > frontend/.env.local
  fi
fi
cd frontend
if [ ! -d node_modules ]; then pnpm install; fi
exec pnpm dev
