#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

BIN=/tmp/multicaa-server
LOG=/tmp/multica-server.log
PORT=8080

echo "==> building server"
(cd server && go build -o "${BIN}.new" ./cmd/server)
mv "${BIN}.new" "${BIN}"

echo "==> stopping any server on :${PORT}"
PIDS=$(lsof -ti:${PORT} 2>/dev/null || true)
for pid in $PIDS; do
    if ps -p "$pid" -o command= 2>/dev/null | grep -q multicaa-server; then
        echo "    killing pid=$pid"
        kill "$pid" || true
    fi
done

# Wait for the port to be free.
for _ in 1 2 3 4 5; do
    if ! lsof -ti:${PORT} 2>/dev/null | xargs -I{} ps -p {} -o command= 2>/dev/null | grep -q multicaa-server; then
        break
    fi
    sleep 1
done

echo "==> starting server"
set -a
source .env
set +a
nohup "${BIN}" >> "${LOG}" 2>&1 < /dev/null &
disown

sleep 2
NEW_PID=$(lsof -ti:${PORT} 2>/dev/null | xargs -I{} sh -c 'ps -p {} -o pid=,command= 2>/dev/null | grep multicaa-server | awk "{print \$1}"' | head -1)
if [ -n "${NEW_PID:-}" ]; then
    echo "==> server up: pid=${NEW_PID} log=${LOG}"
else
    echo "!! server did not bind :${PORT}; check ${LOG}"
    exit 1
fi
