#!/bin/sh
set -eu

CP_PORT="${CP_PORT:-8080}"
DP_PORT="${DP_PORT:-8081}"
stop_requested=0

stop_children() {
  stop_requested=1
  kill -TERM "$CP_PID" "$DP_PID" 2>/dev/null || true
}

trap stop_children TERM INT

PORT="$CP_PORT" /app/cp &
CP_PID=$!

PORT="$DP_PORT" /app/dp &
DP_PID=$!

while kill -0 "$CP_PID" 2>/dev/null && kill -0 "$DP_PID" 2>/dev/null; do
  sleep 1
done

if [ "$stop_requested" -eq 1 ]; then
  kill -TERM "$CP_PID" "$DP_PID" 2>/dev/null || true
  wait "$CP_PID" 2>/dev/null || true
  wait "$DP_PID" 2>/dev/null || true
  exit 0
fi

if ! kill -0 "$CP_PID" 2>/dev/null; then
  wait "$CP_PID" || STATUS=$?
  kill -TERM "$DP_PID" 2>/dev/null || true
else
  wait "$DP_PID" || STATUS=$?
  kill -TERM "$CP_PID" 2>/dev/null || true
fi
wait 2>/dev/null || true
exit "${STATUS:-1}"
