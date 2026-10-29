#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SANDBOX="$ROOT/packages/sandbox"
BACKEND="$ROOT/packages/backend"
WEB="$ROOT/packages/web"
OPS_WEB="$ROOT/packages/ops-web"

for dir in "$BACKEND" "$SANDBOX" "$WEB" "$OPS_WEB"; do
  if [[ ! -f "$dir/.env" ]]; then
    cp "$dir/.env.example" "$dir/.env"
    echo "created $dir/.env"
  fi
done

env_value() {
  local file=$1 key=$2 fallback=$3 value
  value=$(set -a; . "$file" >/dev/null 2>&1; printf '%s' "${!key:-}")
  printf '%s' "${value:-$fallback}"
}

CP_PORT=$(env_value "$BACKEND/.env" CP_PORT 8080)
DP_PORT=$(env_value "$BACKEND/.env" DP_PORT 8081)
SANDBOX_PORT=$(env_value "$SANDBOX/.env" SANDBOX_PORT 8000)
SANDBOX_REGISTRY_TOKEN=$(env_value "$BACKEND/.env" SANDBOX_REGISTRY_TOKEN "change-me")
CP_REGISTRY_URL="http://127.0.0.1:${CP_PORT}/cp/api/v1/internal/sandboxes/heartbeat"
CORS_ORIGINS=$(env_value "$BACKEND/.env" CORS_ORIGINS "http://vibebiz-dev.com:5173,http://vibebiz-dev-ops.com:5174")

mkdir -p "$ROOT/data/workspaces"

if [[ ! -x "$SANDBOX/.venv/bin/python" ]]; then
  echo "sandbox venv missing, run: make -C packages/sandbox install"
  exit 1
fi

pids=()
cleanup() {
  for pid in "${pids[@]}"; do
    kill "$pid" 2>/dev/null || true
  done
}
trap cleanup EXIT INT TERM

(
  cd "$SANDBOX"
  set -a
  source ./.env
  set +a
  mkdir -p "$WORKSPACES_DIR"
  WORKSPACES_DIR="$WORKSPACES_DIR" \
    SANDBOX_NAME="${SANDBOX_NAME:-sandbox-local}" \
    SANDBOX_ADVERTISE_HOST="${SANDBOX_ADVERTISE_HOST:-127.0.0.1}" \
    SANDBOX_ADVERTISE_PORT="$SANDBOX_PORT" \
    CP_REGISTRY_URL="$CP_REGISTRY_URL" \
    SANDBOX_REGISTRY_TOKEN="$SANDBOX_REGISTRY_TOKEN" \
    .venv/bin/python -m uvicorn app.main:app --reload --host 127.0.0.1 --no-access-log --port "$SANDBOX_PORT"
) &
pids+=($!)

(
  cd "$BACKEND"
  set -a
  source ./.env
  set +a
  HOST=127.0.0.1 \
    PORT="$CP_PORT" \
    SANDBOX_REGISTRY_TOKEN="$SANDBOX_REGISTRY_TOKEN" \
    CORS_ORIGINS="$CORS_ORIGINS" \
    OTEL_SERVICE_NAME=vibebiz-cp \
    go run ./cmd/cp
) &
pids+=($!)

(
  cd "$BACKEND"
  set -a
  source ./.env
  set +a
  HOST=127.0.0.1 \
    PORT="$DP_PORT" \
    CORS_ORIGINS="$CORS_ORIGINS" \
    OTEL_SERVICE_NAME=vibebiz-dp \
    go run ./cmd/dp
) &
pids+=($!)

(
  cd "$WEB"
  npm run dev
) &
pids+=($!)

(
  cd "$OPS_WEB"
  npm run dev
) &
pids+=($!)

echo "sandbox  http://localhost:${SANDBOX_PORT}/health"
echo "cp       http://vibebiz-dev.com:${CP_PORT}/health"
echo "dp       http://vibebiz-dev.com:${DP_PORT}/health"
echo "web      http://vibebiz-dev.com:5173"
echo "ops-web  http://vibebiz-dev-ops.com:5174"

wait
