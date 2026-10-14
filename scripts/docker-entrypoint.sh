#!/bin/sh
set -eu

umask 077

# Print the public address once ready, keeping TryNAS as PID 1 for stop signals.
(
    attempts=0
    while [ "$attempts" -lt 60 ]; do
        if curl -fsS --max-time 2 http://127.0.0.1:8080/api/health >/dev/null 2>&1; then
            printf 'TryNAS is ready. Open: %s\n' "${PUBLIC_URL:-https://nas.tryplusit.com}"
            exit 0
        fi
        attempts=$((attempts + 1))
        sleep 1
    done
) &

exec /usr/local/bin/trynas "$@"
