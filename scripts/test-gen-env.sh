#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
fixture_root="$(mktemp -d "${TMPDIR:-/tmp}/ghanageo-gen-env.XXXXXX")"
trap 'rm -rf "$fixture_root"' EXIT

mkdir -p "$fixture_root/scripts"
cp "$repo_root/scripts/gen-env.sh" "$fixture_root/scripts/gen-env.sh"
(
  cd "$fixture_root"
  ./scripts/gen-env.sh >/dev/null
)

value() {
  local file="$1" key="$2"
  sed -n "s/^${key}=//p" "$file" | head -1 | sed -e 's/^"//' -e 's/"$//'
}

prod_token="$(value "$fixture_root/.env.production" INTERNAL_SERVICE_TOKEN)"
prod_auth="$(value "$fixture_root/.env.production" AUTH_SECRET)"
prod_session="$(value "$fixture_root/.env.production" SESSION_SECRET)"
local_token="$(value "$fixture_root/.env" INTERNAL_SERVICE_TOKEN)"

[[ -n "$prod_token" && -n "$prod_auth" && -n "$prod_session" ]]
[[ "$prod_token" != "$(value "$fixture_root/.env" INTERNAL_SERVICE_TOKEN)" ]]

for file in services/api/.env.production services/worker/.env.production; do
  [[ "$(value "$fixture_root/$file" INTERNAL_SERVICE_TOKEN)" == "$prod_token" ]]
done
[[ -z "$(value "$fixture_root/services/api/.env.production" AUTH_SECRET)" ]]
[[ -z "$(value "$fixture_root/services/api/.env.production" SESSION_SECRET)" ]]
[[ "$(value "$fixture_root/services/api/.env.production" API_TRUSTED_PROXY_CIDRS)" == "PASTE_RENDER_EDGE_PROXY_CIDRS" ]]
[[ "$(value "$fixture_root/services/api/.env.production" API_PASSKEY_RPID)" == "digitalghana.dev" ]]
[[ "$(value "$fixture_root/services/api/.env.production" API_PASSKEY_ORIGINS)" == "https://console-geo.digitalghana.dev,https://admin-geo.digitalghana.dev" ]]
[[ "$(value "$fixture_root/services/api/.env.production" GHANAGEO_SERVE_MODE)" == "http" ]]
[[ "$(value "$fixture_root/services/api/.env.production" OTEL_EXPORTER_OTLP_ENDPOINT)" == "PASTE_OTEL_EXPORTER_OTLP_ENDPOINT" ]]
[[ "$(value "$fixture_root/services/api/.env.production" OTEL_EXPORTER_OTLP_HEADERS)" == "PASTE_OTEL_EXPORTER_OTLP_HEADERS" ]]
[[ "$(value "$fixture_root/services/worker/.env.production" OTEL_EXPORTER_OTLP_ENDPOINT)" == "PASTE_OTEL_EXPORTER_OTLP_ENDPOINT" ]]
[[ "$(value "$fixture_root/services/worker/.env.production" OTEL_EXPORTER_OTLP_HEADERS)" == "PASTE_OTEL_EXPORTER_OTLP_HEADERS" ]]

for app in web sandbox portal admin; do
  file="$fixture_root/apps/$app/.env.production"
  local_file="$fixture_root/apps/$app/.env"
  [[ -n "$(value "$local_file" AUTH_SECRET)" ]]
  [[ -n "$(value "$local_file" SESSION_SECRET)" ]]
  [[ "$(value "$local_file" AUTH_SECRET)" != "$prod_auth" ]]
  [[ "$(value "$local_file" SESSION_SECRET)" != "$prod_session" ]]
  [[ "$(value "$file" AUTH_SECRET)" == "$prod_auth" ]]
  [[ "$(value "$file" SESSION_SECRET)" == "$prod_session" ]]
  [[ "$(value "$file" INTERNAL_SERVICE_TOKEN)" == "$prod_token" ]]
  [[ "$(value "$file" NEXT_PUBLIC_GHANAGEO_WEB_URL)" == "https://geo.digitalghana.dev" ]]
  [[ "$(value "$file" NEXT_PUBLIC_GHANAGEO_SANDBOX_URL)" == "https://sandbox-geo.digitalghana.dev" ]]
  [[ "$(value "$file" NEXT_PUBLIC_GHANAGEO_PORTAL_URL)" == "https://console-geo.digitalghana.dev" ]]
done

[[ -z "$(value "$fixture_root/.env" AUTH_SECRET)" ]]
[[ -z "$(value "$fixture_root/.env" SESSION_SECRET)" ]]

[[ "$(value "$fixture_root/apps/web/.env.production" NEXT_PUBLIC_GHANAGEO_INDEXABLE)" == true ]]
for app in sandbox portal admin; do
  [[ "$(value "$fixture_root/apps/$app/.env.production" NEXT_PUBLIC_GHANAGEO_INDEXABLE)" == false ]]
done

(
  cd "$fixture_root"
  ./scripts/gen-env.sh --rotate-local >/dev/null
)
[[ "$(value "$fixture_root/.env.production" INTERNAL_SERVICE_TOKEN)" == "$prod_token" ]]
[[ "$(value "$fixture_root/.env.production" AUTH_SECRET)" == "$prod_auth" ]]
[[ "$(value "$fixture_root/.env.production" SESSION_SECRET)" == "$prod_session" ]]
[[ "$(value "$fixture_root/.env" INTERNAL_SERVICE_TOKEN)" != "$local_token" ]]

if (cd "$fixture_root" && ./scripts/gen-env.sh --rotate >/dev/null 2>&1); then
  echo "deprecated unscoped --rotate unexpectedly succeeded" >&2
  exit 1
fi
[[ "$(value "$fixture_root/.env.production" INTERNAL_SERVICE_TOKEN)" == "$prod_token" ]]

echo "gen-env regression checks passed (secret values withheld)"
