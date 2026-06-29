#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -f "$SCRIPT_DIR/env.sh" ]; then
  # shellcheck disable=SC1091
  . "$SCRIPT_DIR/env.sh"
elif [ -f "$SCRIPT_DIR/../../.multica/local-dev/bin/env.sh" ]; then
  # shellcheck disable=SC1091
  . "$SCRIPT_DIR/../../.multica/local-dev/bin/env.sh"
else
  echo "Missing Simultica local-dev env.sh" >&2
  exit 1
fi

cd "$REPO_ROOT"

TRAE_BIN="${MULTICA_TRAEX_PATH:-/Users/bytedance/.local/bin/traex}"
INSTALL_CMD="${SIMULTICA_TRAEX_INSTALL_CMD:-}"
DAEMON_HEALTH_PORT="${SIMULTICA_DAEMON_HEALTH_PORT:-19596}"

usage() {
  cat <<'USAGE'
Usage: update-traex.sh [--verify-only]

By default this script verifies the configured TraeX binary. To actually update
TraeX, provide SIMULTICA_TRAEX_INSTALL_CMD, for example:

  SIMULTICA_TRAEX_INSTALL_CMD='brew upgrade traex' .multica/local-dev/bin/update-traex.sh

The install command is intentionally external because TraeX distribution can
vary by machine. After a successful update, the script restarts only the
Simultica desktop daemon by restarting the Simultica desktop app.
USAGE
}

VERIFY_ONLY=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --verify-only)
      VERIFY_ONLY=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

version_of() {
  local bin="$1"
  if [ ! -x "$bin" ]; then
    printf 'missing'
    return 0
  fi
  "$bin" --version 2>/dev/null | head -1 || true
}

before="$(version_of "$TRAE_BIN")"
echo "TraeX binary:  $TRAE_BIN"
echo "Before:        ${before:-unknown}"

if [ "$VERIFY_ONLY" != "1" ]; then
  if [ -z "$INSTALL_CMD" ]; then
    cat >&2 <<MSG
No TraeX install command configured.

Set SIMULTICA_TRAEX_INSTALL_CMD to the command that updates your TraeX binary,
or run with --verify-only.
MSG
    exit 1
  fi

  echo "==> Updating TraeX"
  sh -c "$INSTALL_CMD"
fi

after="$(version_of "$TRAE_BIN")"
echo "After:         ${after:-unknown}"

if [ ! -x "$TRAE_BIN" ]; then
  echo "TraeX binary is not executable: $TRAE_BIN" >&2
  exit 1
fi

if [ "$VERIFY_ONLY" != "1" ]; then
  echo "==> Restarting Simultica desktop daemon through desktop launcher"
  "$LOCAL_DEV_DIR/bin/desktop.sh"
fi

echo "==> Checking daemon health"
curl -fsS "http://127.0.0.1:${DAEMON_HEALTH_PORT}/health" >/dev/null

echo "==> Checking TraeX runtime row"
if command -v psql >/dev/null 2>&1; then
  psql "$DATABASE_URL" -c "select id, provider, status, device_info, last_seen_at from agent_runtime where provider = 'traex' order by last_seen_at desc limit 1;"
fi

"$LOCAL_DEV_DIR/bin/write-version.sh" 2>/dev/null || true
