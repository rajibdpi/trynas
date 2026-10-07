#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

# Give each server its own process group so cleanup also stops its children.
set -m
server_pids=()
cleanup() {
  trap - EXIT INT TERM
  for pid in "${server_pids[@]}"; do
    kill -TERM -- "-$pid" 2>/dev/null || true
  done
  for pid in "${server_pids[@]}"; do
    wait "$pid" 2>/dev/null || true
  done
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

./run-backend.sh &
server_pids+=("$!")
./run-frontend.sh &
server_pids+=("$!")

echo "Starting TryNAS development servers. Press Ctrl+C to stop both."
echo "Frontend: http://localhost:3000"
echo "Backend:  http://localhost:8080 (unless ADDR is configured differently)"

# Stop the other server if either exits, preserving the exit status.
while true; do
  for pid in "${server_pids[@]}"; do
    if ! kill -0 "$pid" 2>/dev/null; then
      status=0
      wait "$pid" || status=$?
      exit "$status"
    fi
  done
  sleep 1
done
