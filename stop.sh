#!/usr/bin/env bash
set -euo pipefail

PORT=8080

PIDS=$(lsof -ti:${PORT} 2>/dev/null || true)
if [ -z "$PIDS" ]; then
    echo "==> nothing on :${PORT}"
    exit 0
fi

for pid in $PIDS; do
    if ps -p "$pid" -o command= 2>/dev/null | grep -q multicaa-server; then
        echo "==> killing multicaa-server pid=$pid"
        kill "$pid" || true
    fi
done

for _ in 1 2 3 4 5; do
    REMAINING=$(lsof -ti:${PORT} 2>/dev/null | xargs -I{} sh -c 'ps -p {} -o command= 2>/dev/null | grep -q multicaa-server && echo {}' || true)
    [ -z "$REMAINING" ] && break
    sleep 1
done

echo "==> stopped"
