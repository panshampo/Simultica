#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
LOCAL_BIN_DIR="$REPO_ROOT/.multica/local-dev/bin"

if [ ! -d "$LOCAL_BIN_DIR" ]; then
  echo "Missing local-dev bin directory: $LOCAL_BIN_DIR" >&2
  echo "Run the Simultica local-dev setup first." >&2
  exit 1
fi

tools=(
  check-update.sh
  lark-status.sh
  update-simultica.sh
  update-traex.sh
  write-version.sh
)

for tool in "${tools[@]}"; do
  src="$SCRIPT_DIR/$tool"
  dst="$LOCAL_BIN_DIR/$tool"
  if [ ! -f "$src" ]; then
    echo "Missing source tool: $src" >&2
    exit 1
  fi
  cp "$src" "$dst"
  chmod +x "$dst"
  echo "Installed $dst"
done

echo "Simultica local-dev tools installed."
