#!/usr/bin/env bash
#
# Starts the whole GhanaGeo stack and reports what is listening where.
#
#   ./scripts/dev.sh          start everything
#   ./scripts/dev.sh --stop   stop the apps and the API (containers keep running)
#
set -uo pipefail
cd "$(dirname "$0")/.."

APPS=("web:3100" "sandbox:3101" "portal:3102" "admin:3103")
LOGS=".dev-logs"

stop_all() {
  echo "Stopping…"
  pkill -f "gg-api" 2>/dev/null
  # `next start` runs as `next-server`, which is why a naive
  # pkill -f "next start" misses it and leaves a stale build serving.
  pgrep -f "next-server" | while read -r pid; do kill "$pid" 2>/dev/null; done
  echo "  stopped (containers left running — use 'docker compose down' for those)"
}

if [[ "${1:-}" == "--stop" ]]; then stop_all; exit 0; fi

mkdir -p "$LOGS"
stop_all >/dev/null 2>&1
sleep 1

echo "1/3  Containers"
docker compose up -d --wait >/dev/null 2>&1 \
  && docker compose ps --format "     {{.Name}}: {{.Status}}" \
  || { echo "     ✗ docker compose failed"; exit 1; }

echo
echo "2/3  API"
( cd services/api && set -a && . ./.env && set +a && go build -o /tmp/gg-api ./cmd/api ) || exit 1
( cd services/api && set -a && . ./.env && set +a && nohup /tmp/gg-api > "../../$LOGS/api.log" 2>&1 & )
for _ in $(seq 1 30); do
  curl -s -m1 localhost:8180/health >/dev/null 2>&1 && break
  sleep 1
done
if curl -s -m1 localhost:8180/health >/dev/null 2>&1; then
  echo "     ✓ http://localhost:8180  $(curl -s localhost:8180/health)"
else
  echo "     ✗ API did not start — see $LOGS/api.log"; exit 1
fi

echo
echo "3/3  Apps"
for entry in "${APPS[@]}"; do
  name="${entry%%:*}"; port="${entry##*:}"
  ( cd "apps/$name" && nohup pnpm start > "../../$LOGS/$name.log" 2>&1 & )
done
for entry in "${APPS[@]}"; do
  name="${entry%%:*}"; port="${entry##*:}"
  ok=""
  for _ in $(seq 1 40); do
    if curl -s -m1 -o /dev/null "http://localhost:$port/"; then ok="yes"; break; fi
    sleep 1
  done
  if [[ -n "$ok" ]]; then
    printf "     ✓ %-9s http://localhost:%s\n" "$name" "$port"
  else
    printf "     ✗ %-9s failed — see %s/%s.log\n" "$name" "$LOGS" "$name"
  fi
done

echo
echo "Open:"
for entry in "${APPS[@]}"; do echo "  http://localhost:${entry##*:}   ${entry%%:*}"; done
echo "  http://localhost:8180/health   api"
