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

PROFILE="${SIMULTICA_PROFILE:-desktop-localhost-18083}"
WORKSPACE_ID="${SIMULTICA_WORKSPACE_ID:-}"
if [ -z "$WORKSPACE_ID" ] && [ -f "$HOME/.multica/profiles/$PROFILE/config.json" ]; then
  WORKSPACE_ID="$(jq -r '.workspace_id // empty' "$HOME/.multica/profiles/$PROFILE/config.json" 2>/dev/null || true)"
fi
if [ -z "$WORKSPACE_ID" ]; then
  WORKSPACE_ID="7e77c48d-dd0b-481b-ac36-a77f7b0967f5"
fi

TOKEN=""
if [ -f "$HOME/.multica/profiles/$PROFILE/config.json" ]; then
  TOKEN="$(jq -r '.token // empty' "$HOME/.multica/profiles/$PROFILE/config.json" 2>/dev/null || true)"
fi

has_secret="false"
if [ -n "${MULTICA_LARK_SECRET_KEY:-}" ]; then
  has_secret="true"
fi

api_json=""
if [ -n "$TOKEN" ]; then
  api_json="$(curl -fsS "http://localhost:${PORT}/api/workspaces/${WORKSPACE_ID}/lark/installations" \
    -H "Authorization: Bearer $TOKEN" \
    -H "X-Workspace-ID: $WORKSPACE_ID" 2>/dev/null || true)"
fi

configured="unknown"
install_supported="unknown"
install_count="unknown"
if [ -n "$api_json" ] && command -v jq >/dev/null 2>&1; then
  configured="$(printf '%s' "$api_json" | jq -r '.configured // "unknown"')"
  install_supported="$(printf '%s' "$api_json" | jq -r '.install_supported // "unknown"')"
  install_count="$(printf '%s' "$api_json" | jq -r '.installations | length')"
fi

printf 'Simultica Lark Status\n\n'
printf 'Repo:              %s\n' "$REPO_ROOT"
printf 'Profile:           %s\n' "$PROFILE"
printf 'Workspace:         %s\n' "$WORKSPACE_ID"
printf 'API:               http://localhost:%s\n' "$PORT"
printf 'Secret configured: %s\n' "$has_secret"
printf 'API configured:    %s\n' "$configured"
printf 'Install supported: %s\n' "$install_supported"
printf 'Installations:     %s\n' "$install_count"

if command -v psql >/dev/null 2>&1; then
  printf '\nActive installations:\n'
  psql "$DATABASE_URL" -c "select id, agent_id, app_id, status, region, installed_at from lark_installation order by installed_at desc limit 20;" 2>/dev/null || true

  printf '\nRecent inbound audit drops:\n'
  psql "$DATABASE_URL" -c "select created_at, drop_reason, lark_chat_id, lark_message_id from lark_inbound_audit order by created_at desc limit 10;" 2>/dev/null || true
fi

if [ -n "$api_json" ]; then
  printf '\nAPI response:\n%s\n' "$api_json"
fi
