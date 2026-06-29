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

VERSION_FILE="$LOCAL_DEV_DIR/SIMULTICA_VERSION.json"
TRAE_BIN="${MULTICA_TRAEX_PATH:-/Users/bytedance/.local/bin/traex}"

json_string() {
  local value="${1:-}"
  if command -v jq >/dev/null 2>&1; then
    jq -Rn --arg v "$value" '$v'
  else
    printf '"%s"' "$(printf '%s' "$value" | sed 's/\\/\\\\/g; s/"/\\"/g')"
  fi
}

branch="$(git branch --show-current 2>/dev/null || true)"
commit="$(git rev-parse HEAD 2>/dev/null || true)"
commit_short="$(git rev-parse --short HEAD 2>/dev/null || true)"
dirty="false"
if [ -n "$(git status --porcelain 2>/dev/null || true)" ]; then
  dirty="true"
fi

upstream_ref="${SIMULTICA_UPSTREAM_REF:-${SIMULTICA_UPSTREAM_REMOTE:-origin}/${SIMULTICA_UPSTREAM_BRANCH:-master}}"
upstream_commit=""
if git rev-parse --verify "$upstream_ref" >/dev/null 2>&1; then
  upstream_commit="$(git rev-parse "$upstream_ref")"
fi

traex_version="missing"
if [ -x "$TRAE_BIN" ]; then
  traex_version="$("$TRAE_BIN" --version 2>/dev/null | head -1 || true)"
  if [ -z "$traex_version" ]; then
    traex_version="unknown"
  fi
fi

cli_version=""
if [ -x "$REPO_ROOT/apps/desktop/resources/bin/multica" ]; then
  cli_version="$("$REPO_ROOT/apps/desktop/resources/bin/multica" version 2>/dev/null | head -1 || true)"
fi

cat > "$VERSION_FILE" <<JSON
{
  "simultica_version": $(json_string "local-${commit_short:-unknown}"),
  "branch": $(json_string "$branch"),
  "commit": $(json_string "$commit"),
  "dirty": $dirty,
  "upstream_ref": $(json_string "$upstream_ref"),
  "upstream_commit": $(json_string "$upstream_commit"),
  "traex_path": $(json_string "$TRAE_BIN"),
  "traex_version": $(json_string "$traex_version"),
  "cli_version": $(json_string "$cli_version"),
  "api_url": $(json_string "http://localhost:${PORT}"),
  "web_url": $(json_string "http://localhost:${FRONTEND_PORT}"),
  "updated_at": $(json_string "$(date -Iseconds)")
}
JSON

echo "Wrote $VERSION_FILE"
