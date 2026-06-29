#!/usr/bin/env bash
if [ -z "${BASH_VERSION:-}" ]; then
  exec bash "$0" "$@"
fi

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

COMPOSE_FILES=(-f docker-compose.selfhost.yml -f docker-compose.selfhost.build.yml)
ENV_FILE="${ENV_FILE:-.env}"
BIND_HOST="${BIND_HOST:-0.0.0.0}"
BUILD=0

usage() {
  cat <<'EOF'
Usage: scripts/restart-selfhost.sh [options]

Restart the self-hosted Docker Compose services.

Options:
  --build              Rebuild backend/frontend images before restarting.
  --bind-host HOST     Host to bind published ports to. Default: 0.0.0.0.
  --local-only         Shortcut for --bind-host 127.0.0.1.
  -h, --help           Show this help.

Examples:
  scripts/restart-selfhost.sh
  scripts/restart-selfhost.sh --build
  scripts/restart-selfhost.sh --local-only
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --build)
      BUILD=1
      shift
      ;;
    --bind-host)
      if [ "$#" -lt 2 ]; then
        echo "Missing value for --bind-host" >&2
        exit 1
      fi
      BIND_HOST="$2"
      shift 2
      ;;
    --local-only)
      BIND_HOST="127.0.0.1"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is required but was not found in PATH." >&2
  exit 1
fi

DOCKER=(docker)
if ! docker info >/dev/null 2>&1; then
  DOCKER=(sudo docker)
fi

upsert_env() {
  local key="$1"
  local value="$2"

  if [ ! -f "$ENV_FILE" ]; then
    if [ -f .env.example ]; then
      cp .env.example "$ENV_FILE"
    else
      : >"$ENV_FILE"
    fi
  fi

  if grep -q "^${key}=" "$ENV_FILE"; then
    sed -i "s|^${key}=.*|${key}=${value}|" "$ENV_FILE"
  else
    printf '\n%s=%s\n' "$key" "$value" >>"$ENV_FILE"
  fi
}

upsert_env "BIND_HOST" "$BIND_HOST"

echo "==> Restarting self-host services"
echo "    env file:  $ENV_FILE"
echo "    bind host: $BIND_HOST"

if [ "$BUILD" -eq 1 ]; then
  echo "    mode:      rebuild images and restart"
  "${DOCKER[@]}" compose "${COMPOSE_FILES[@]}" up -d --build --force-recreate --remove-orphans
else
  echo "    mode:      restart existing images"
  "${DOCKER[@]}" compose "${COMPOSE_FILES[@]}" up -d --no-build --force-recreate --remove-orphans
fi

echo
echo "==> Container status"
"${DOCKER[@]}" compose "${COMPOSE_FILES[@]}" ps

frontend_port="${FRONTEND_PORT:-3000}"
backend_port="${BACKEND_PORT:-${API_PORT:-${SERVER_PORT:-${PORT:-8080}}}}"
server_host="$BIND_HOST"
if [ "$server_host" = "0.0.0.0" ]; then
  server_host="$(hostname -I 2>/dev/null | awk '{print $1}')"
fi

echo
echo "==> Access URLs"
echo "    frontend: http://${server_host}:${frontend_port}"
echo "    backend:  http://${server_host}:${backend_port}"
