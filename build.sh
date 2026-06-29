#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="${OUTPUT_DIR:-"$ROOT_DIR/dist/tce"}"

VERSION="${VERSION:-$(git -C "$ROOT_DIR" describe --tags --always --dirty 2>/dev/null || echo dev)}"
COMMIT="${COMMIT:-$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || echo unknown)}"
DATE="${DATE:-$(date -u '+%Y-%m-%dT%H:%M:%SZ')}"

GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-amd64}"
CGO_ENABLED="${CGO_ENABLED:-0}"

echo "==> Building campaign_insight backend"
echo "    version: ${VERSION}"
echo "    commit:  ${COMMIT}"
echo "    date:    ${DATE}"
echo "    target:  ${GOOS}/${GOARCH}, CGO_ENABLED=${CGO_ENABLED}"
echo "    output:  ${OUTPUT_DIR}"
echo "    go:      $(go version)"

rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR" "$ROOT_DIR/server/bin"

(
    cd "$ROOT_DIR/server"
    go mod download

    env GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED="$CGO_ENABLED" \
        go build -trimpath -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
        -o "$OUTPUT_DIR/server" ./cmd/server

    env GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED="$CGO_ENABLED" \
        go build -trimpath -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
        -o "$OUTPUT_DIR/multica" ./cmd/multica

    env GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED="$CGO_ENABLED" \
        go build -trimpath -ldflags "-s -w" \
        -o "$OUTPUT_DIR/migrate" ./cmd/migrate

    env GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED="$CGO_ENABLED" \
        go build -trimpath -ldflags "-s -w" \
        -o "$OUTPUT_DIR/backfill_task_usage_hourly" ./cmd/backfill_task_usage_hourly
)

cp -R "$ROOT_DIR/server/migrations" "$OUTPUT_DIR/migrations"

cat > "$OUTPUT_DIR/bootstrap.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

echo "Running database migrations..."
./migrate up

echo "Starting server..."
exec ./server
EOF

chmod +x \
    "$OUTPUT_DIR/server" \
    "$OUTPUT_DIR/multica" \
    "$OUTPUT_DIR/migrate" \
    "$OUTPUT_DIR/backfill_task_usage_hourly" \
    "$OUTPUT_DIR/bootstrap.sh"

cat > "$OUTPUT_DIR/build-info.env" <<EOF
VERSION=${VERSION}
COMMIT=${COMMIT}
DATE=${DATE}
GOOS=${GOOS}
GOARCH=${GOARCH}
CGO_ENABLED=${CGO_ENABLED}
EOF

echo "==> Build complete"
find "$OUTPUT_DIR" -maxdepth 1 -type f | sort
echo "    migrations: $(find "$OUTPUT_DIR/migrations" -type f | wc -l | tr -d ' ') files"
