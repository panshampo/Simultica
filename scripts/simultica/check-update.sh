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

UPSTREAM_REMOTE="${SIMULTICA_UPSTREAM_REMOTE:-origin}"
UPSTREAM_BRANCH="${SIMULTICA_UPSTREAM_BRANCH:-master}"
UPSTREAM_REF="${SIMULTICA_UPSTREAM_REF:-$UPSTREAM_REMOTE/$UPSTREAM_BRANCH}"
TRAE_BIN="${MULTICA_TRAEX_PATH:-/Users/bytedance/.local/bin/traex}"
PROFILE="${SIMULTICA_PROFILE:-desktop-localhost-18083}"
DAEMON_HEALTH_PORT="${SIMULTICA_DAEMON_HEALTH_PORT:-19596}"

json_bool() {
  if [ "$1" = "1" ]; then
    printf 'true'
  else
    printf 'false'
  fi
}

json_string() {
  local value="${1:-}"
  if command -v jq >/dev/null 2>&1; then
    jq -Rn --arg v "$value" '$v'
  else
    python3 -c 'import json,sys; print(json.dumps(sys.argv[1]))' "$value"
  fi
}

health_status() {
  local url="$1"
  if curl -fsS "$url" >/dev/null 2>&1; then
    printf 'healthy'
  else
    printf 'unreachable'
  fi
}

current_branch="$(git branch --show-current 2>/dev/null || true)"
current_commit="$(git rev-parse --short HEAD 2>/dev/null || true)"
dirty_count="$(git status --porcelain | wc -l | tr -d ' ')"
dirty=0
if [ "${dirty_count:-0}" != "0" ]; then
  dirty=1
fi

upstream_exists=0
behind="unknown"
ahead="unknown"
if git rev-parse --verify "$UPSTREAM_REF" >/dev/null 2>&1; then
  upstream_exists=1
  counts="$(git rev-list --left-right --count "HEAD...$UPSTREAM_REF" 2>/dev/null || true)"
  if [ -n "$counts" ]; then
    ahead="$(printf '%s' "$counts" | awk '{print $1}')"
    behind="$(printf '%s' "$counts" | awk '{print $2}')"
  fi
fi

traex_version="missing"
if [ -x "$TRAE_BIN" ]; then
  traex_version="$("$TRAE_BIN" --version 2>/dev/null | head -1 || true)"
  if [ -z "$traex_version" ]; then
    traex_version="unknown"
  fi
fi

api_health="$(health_status "http://localhost:${PORT}/health")"
web_health="$(health_status "http://localhost:${FRONTEND_PORT}")"
daemon_health="$(health_status "http://127.0.0.1:${DAEMON_HEALTH_PORT}/health")"
db_health="unreachable"
if command -v psql >/dev/null 2>&1 && psql "$DATABASE_URL" -c "select 1" >/dev/null 2>&1; then
  db_health="healthy"
fi

runtime_status="unknown"
runtime_id=""
if command -v psql >/dev/null 2>&1; then
  runtime_row="$(psql "$DATABASE_URL" -At -F $'\t' -c "select id, status from agent_runtime where provider = 'traex' order by last_seen_at desc limit 1" 2>/dev/null || true)"
  if [ -n "$runtime_row" ]; then
    runtime_id="$(printf '%s' "$runtime_row" | awk -F '\t' '{print $1}')"
    runtime_status="$(printf '%s' "$runtime_row" | awk -F '\t' '{print $2}')"
  fi
fi

printf 'Simultica Update Check\n\n'
printf 'Repo:             %s\n' "$REPO_ROOT"
printf 'Current branch:   %s\n' "${current_branch:-detached}"
printf 'Current commit:   %s\n' "$current_commit"
printf 'Upstream ref:     %s\n' "$UPSTREAM_REF"
printf 'Upstream exists:  %s\n' "$(json_bool "$upstream_exists")"
printf 'Ahead upstream:   %s\n' "$ahead"
printf 'Behind upstream:  %s\n' "$behind"
printf 'Local changes:    %s (%s files)\n' "$(json_bool "$dirty")" "$dirty_count"
printf 'DB:               %s localhost:%s\n' "$db_health" "$POSTGRES_PORT"
printf 'API:              %s http://localhost:%s\n' "$api_health" "$PORT"
printf 'Web:              %s http://localhost:%s\n' "$web_health" "$FRONTEND_PORT"
printf 'Daemon:           %s http://127.0.0.1:%s/health\n' "$daemon_health" "$DAEMON_HEALTH_PORT"
printf 'Profile:          %s\n' "$PROFILE"
printf 'TraeX binary:     %s\n' "$TRAE_BIN"
printf 'TraeX version:    %s\n' "$traex_version"
printf 'TraeX runtime:    %s %s\n' "$runtime_status" "$runtime_id"

if [ "${SIMULTICA_CHECK_JSON:-0}" = "1" ]; then
  printf '\nJSON:\n'
  cat <<JSON
{
  "repo": $(json_string "$REPO_ROOT"),
  "branch": $(json_string "${current_branch:-detached}"),
  "commit": $(json_string "$current_commit"),
  "upstream_ref": $(json_string "$UPSTREAM_REF"),
  "upstream_exists": $(json_bool "$upstream_exists"),
  "ahead": $(json_string "$ahead"),
  "behind": $(json_string "$behind"),
  "dirty": $(json_bool "$dirty"),
  "dirty_count": $dirty_count,
  "db_health": $(json_string "$db_health"),
  "api_health": $(json_string "$api_health"),
  "web_health": $(json_string "$web_health"),
  "daemon_health": $(json_string "$daemon_health"),
  "profile": $(json_string "$PROFILE"),
  "traex_binary": $(json_string "$TRAE_BIN"),
  "traex_version": $(json_string "$traex_version"),
  "traex_runtime_id": $(json_string "$runtime_id"),
  "traex_runtime_status": $(json_string "$runtime_status")
}
JSON
fi
