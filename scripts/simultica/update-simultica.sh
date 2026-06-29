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
UPDATE_PREFIX="${SIMULTICA_UPDATE_PREFIX:-simultica-update}"
HISTORY_DIR="$LOCAL_DEV_DIR/update-history"
BACKUP_DIR="$LOCAL_DEV_DIR/backups"
MODE="prepare"
ALLOW_DIRTY=0
SKIP_FETCH=0
SKIP_DB_BACKUP=0

usage() {
  cat <<'USAGE'
Usage: update-simultica.sh [--prepare|--apply] [options]

Modes:
  --prepare       Fetch upstream, create an update branch, and merge upstream.
                  This is the default. It does not rebuild or restart services.
  --apply         Run prepare, then install/build/migrate/restart/verify.

Options:
  --allow-dirty   Allow running with existing local changes.
  --skip-fetch    Do not run git fetch before reading the upstream ref.
  --skip-db-backup
                  Do not dump the local database before applying.
  -h, --help      Show this help.

Environment:
  SIMULTICA_UPSTREAM_REMOTE  Default: origin
  SIMULTICA_UPSTREAM_BRANCH  Default: master
  SIMULTICA_UPSTREAM_REF     Default: $SIMULTICA_UPSTREAM_REMOTE/$SIMULTICA_UPSTREAM_BRANCH
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --prepare)
      MODE="prepare"
      ;;
    --apply)
      MODE="apply"
      ;;
    --allow-dirty)
      ALLOW_DIRTY=1
      ;;
    --skip-fetch)
      SKIP_FETCH=1
      ;;
    --skip-db-backup)
      SKIP_DB_BACKUP=1
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

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Missing required command: $cmd" >&2
    exit 1
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

ensure_expected_workspace() {
  if [ ! -f "$ENV_FILE" ] || [ ! -d "$LOCAL_DEV_DIR/bin" ]; then
    echo "Refusing to update: local-dev environment is incomplete." >&2
    exit 1
  fi
}

ensure_git_ready() {
  require_cmd git

  if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    echo "Refusing to update outside a git worktree." >&2
    exit 1
  fi

  if [ "$ALLOW_DIRTY" != "1" ] && [ -n "$(git status --porcelain)" ]; then
    cat >&2 <<MSG
Refusing to update with local changes.

Commit or stash your changes, or rerun with --allow-dirty after reviewing:
$(git status --short)
MSG
    exit 1
  fi
}

fetch_upstream() {
  if [ "$SKIP_FETCH" = "1" ]; then
    return 0
  fi
  echo "==> Fetching $UPSTREAM_REMOTE"
  git fetch "$UPSTREAM_REMOTE"
}

ensure_upstream_ref() {
  if ! git rev-parse --verify "$UPSTREAM_REF" >/dev/null 2>&1; then
    echo "Missing upstream ref: $UPSTREAM_REF" >&2
    echo "Set SIMULTICA_UPSTREAM_REF or SIMULTICA_UPSTREAM_REMOTE/SIMULTICA_UPSTREAM_BRANCH." >&2
    exit 1
  fi
}

write_history() {
  local phase="$1"
  local file="$2"
  local branch commit upstream_commit traex_version
  branch="$(git branch --show-current 2>/dev/null || true)"
  commit="$(git rev-parse HEAD 2>/dev/null || true)"
  upstream_commit="$(git rev-parse "$UPSTREAM_REF" 2>/dev/null || true)"
  traex_version="$("${MULTICA_TRAEX_PATH:-/Users/bytedance/.local/bin/traex}" --version 2>/dev/null | head -1 || true)"
  mkdir -p "$HISTORY_DIR"
  cat > "$file" <<JSON
{
  "phase": $(json_string "$phase"),
  "mode": $(json_string "$MODE"),
  "repo": $(json_string "$REPO_ROOT"),
  "branch": $(json_string "$branch"),
  "commit": $(json_string "$commit"),
  "upstream_ref": $(json_string "$UPSTREAM_REF"),
  "upstream_commit": $(json_string "$upstream_commit"),
  "traex_version": $(json_string "$traex_version"),
  "recorded_at": $(json_string "$(date -Iseconds)")
}
JSON
}

backup_database() {
  if [ "$SKIP_DB_BACKUP" = "1" ]; then
    echo "==> Skipping database backup"
    return 0
  fi
  require_cmd pg_dump
  mkdir -p "$BACKUP_DIR"
  local file="$BACKUP_DIR/pre-update-$(date +%Y%m%d-%H%M%S).sql"
  echo "==> Backing up local database to $file"
  pg_dump "$DATABASE_URL" > "$file"
}

prepare_update_branch() {
  local branch
  branch="${UPDATE_PREFIX}-$(date +%Y%m%d-%H%M%S)"
  echo "==> Creating update branch $branch"
  git checkout -b "$branch"
  echo "==> Merging $UPSTREAM_REF"
  if ! git merge --no-edit "$UPSTREAM_REF"; then
    cat >&2 <<MSG

Merge stopped with conflicts.
Resolve conflicts on branch '$branch', then run the build/restart steps manually:
  $LOCAL_DEV_DIR/bin/update-simultica.sh --apply --skip-fetch --allow-dirty
MSG
    exit 1
  fi
}

ensure_pnpm() {
  require_cmd node
  if ! command -v pnpm >/dev/null 2>&1 || [ "$(pnpm -v 2>/dev/null || true)" != "10.28.2" ]; then
    echo "==> Activating pnpm 10.28.2 for this workspace"
    corepack enable --install-directory "$PNPM_HOME"
    corepack prepare pnpm@10.28.2 --activate
    export PATH="$PNPM_HOME:$PATH"
  fi
}

apply_update() {
  backup_database
  ensure_pnpm

  echo "==> Installing dependencies"
  pnpm install

  echo "==> Running desktop typecheck"
  pnpm --filter @multica/desktop exec tsc --noEmit

  echo "==> Rebuilding and restarting local Simultica services"
  "$LOCAL_DEV_DIR/bin/dev.sh"
  "$LOCAL_DEV_DIR/bin/desktop.sh"

  echo "==> Verifying local API"
  curl -fsS "http://localhost:${PORT}/health" >/dev/null

  echo "==> Verifying local web"
  curl -fsS "http://localhost:${FRONTEND_PORT}" >/dev/null

  "$LOCAL_DEV_DIR/bin/write-version.sh" 2>/dev/null || true
}

ensure_expected_workspace
ensure_git_ready

timestamp="$(date +%Y%m%d-%H%M%S)"
write_history "before" "$HISTORY_DIR/$timestamp-before.json"
fetch_upstream
ensure_upstream_ref
prepare_update_branch

if [ "$MODE" = "apply" ]; then
  apply_update
  write_history "after" "$HISTORY_DIR/$timestamp-after.json"
  echo "==> Simultica update applied."
else
  echo "==> Simultica update branch prepared."
  echo "    Branch: $(git branch --show-current)"
  echo "    Next:   review changes, then run with --apply when ready."
fi
