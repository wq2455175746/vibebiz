#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

env_value() {
  local file=$1 key=$2 fallback=$3 value
  if [[ -f "$file" ]]; then
    value=$(set -a; . "$file" >/dev/null 2>&1; printf '%s' "${!key:-}")
  fi
  printf '%s' "${value:-$fallback}"
}

CP_PORT=$(env_value "$ROOT/packages/backend/.env" CP_PORT 8080)
DP_PORT=$(env_value "$ROOT/packages/backend/.env" DP_PORT 8081)
SANDBOX_PORT=$(env_value "$ROOT/packages/sandbox/.env" SANDBOX_PORT 8000)
PORTS=("$SANDBOX_PORT" "$CP_PORT" "$DP_PORT" 5173 5174)

is_vibebiz_cwd() {
  local pid=$1 cwd
  cwd=$(lsof -a -p "$pid" -d cwd -Fn 2>/dev/null | sed -n 's/^n//p' | head -1)
  [[ -n "$cwd" && "$cwd" == "$ROOT"* ]]
}

matches_vibebiz_process() {
  local cmd=$1
  case "$cmd" in
    *uvicorn\ app.main:app* | *"go run ./cmd/cp"* | *"go run ./cmd/dp"* | *"/exe/cp"* | *"/exe/dp"* | *"node_modules/vite/bin/vite"* | *"npm run dev"* | *"scripts/dev.sh"* | *"make --no-print-directory -C"*dev*)
      return 0
      ;;
  esac
  return 1
}

pids=()
for port in "${PORTS[@]}"; do
  while IFS= read -r pid; do
    [[ -z "$pid" ]] && continue
    if is_vibebiz_cwd "$pid"; then
      pids+=("$pid")
    fi
  done < <(lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || true)
done

while read -r pid _ cmd; do
  [[ -z "$pid" ]] && continue
  if matches_vibebiz_process "$cmd" && is_vibebiz_cwd "$pid"; then
    pids+=("$pid")
    ppid=$(ps -o ppid= -p "$pid" | tr -d ' ')
    if [[ -n "$ppid" && "$ppid" != "1" ]]; then
      pcmd=$(ps -o command= -p "$ppid" 2>/dev/null || true)
      if matches_vibebiz_process "$pcmd" && is_vibebiz_cwd "$ppid"; then
        pids+=("$ppid")
      fi
    fi
  fi
done < <(ps -axo pid=,ppid=,command=)

if [[ ${#pids[@]} -eq 0 ]]; then
  echo "no vibebiz processes found"
  exit 0
fi

unique=()
while IFS= read -r pid; do
  unique+=("$pid")
done < <(printf '%s\n' "${pids[@]}" | awk 'NF' | sort -u)
pids=("${unique[@]}")
if [[ ${#pids[@]} -eq 0 ]]; then
  echo "no vibebiz processes found"
  exit 0
fi

echo "stopping vibebiz processes: ${pids[*]}"
kill -TERM "${pids[@]}" 2>/dev/null || true
for _ in {1..20}; do
  alive=()
  for pid in "${pids[@]}"; do
    if kill -0 "$pid" 2>/dev/null; then
      alive+=("$pid")
    fi
  done
  [[ ${#alive[@]} -eq 0 ]] && break
  sleep 0.25
done

alive=()
for pid in "${pids[@]}"; do
  if kill -0 "$pid" 2>/dev/null; then
    alive+=("$pid")
  fi
done
if [[ ${#alive[@]} -gt 0 ]]; then
  echo "force stopping: ${alive[*]}"
  kill -KILL "${alive[@]}" 2>/dev/null || true
fi

echo "vibebiz stopped"
