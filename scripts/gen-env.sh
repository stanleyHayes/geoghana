#!/usr/bin/env bash
#
# Generates .env and .env.production for every app and service.
#
# SAFE TO RE-RUN. Any value you have already pasted is preserved: the script
# only fills in blanks and generates the secrets it can generate itself.
# Nothing it writes is ever committed — .gitignore covers .env and .env.*.
#
#   ./scripts/gen-env.sh            # fill blanks, keep everything existing
#   ./scripts/gen-env.sh --rotate   # regenerate the generated secrets too
#
set -euo pipefail
cd "$(dirname "$0")/.."

ROTATE=false
[[ "${1:-}" == "--rotate" ]] && ROTATE=true

# ---------------------------------------------------------------- helpers

gen()   { openssl rand -base64 "${1:-32}" | tr -d '\n=+/' | cut -c1-"${2:-40}"; }
genhex(){ openssl rand -hex "${1:-16}"; }

# read_existing <file> <KEY> — echoes the current value, unquoted.
#
# Surrounding quotes are STRIPPED. The templates add their own, so returning a
# value with quotes attached made every re-run wrap it again: "v" became ""v""
# became """v""", and the file stopped parsing. sed is used rather than bash
# parameter expansion because the escaping there is easy to get wrong.
read_existing() {
  [[ -f "$1" ]] || return 0
  grep -E "^${2}=" "$1" 2>/dev/null \
    | head -1 \
    | cut -d= -f2- \
    | sed -e 's/^"*//' -e 's/"*$//' -e "s/^'*//" -e "s/'*$//" \
    || true
}

# keep <file> <KEY> <fallback> — preserves a pasted value unless --rotate and
# the value is one we generated (never rotates something you pasted by hand).
keep() {
  local file="$1" key="$2" fallback="$3"
  local current
  current="$(read_existing "$file" "$key")"
  if [[ -n "$current" && "$current" != PASTE_* ]]; then
    if [[ "$ROTATE" == true && "${4:-}" == "generated" ]]; then
      echo "$fallback"
    else
      echo "$current"
    fi
  else
    echo "$fallback"
  fi
}

write() {
  local path="$1"; shift
  mkdir -p "$(dirname "$path")"
  printf '%s\n' "$@" > "$path"
  chmod 600 "$path"
  echo "  wrote $path"
}

# ------------------------------------------------- generated shared secrets
# One value per secret, shared across the files that genuinely need the same
# secret (the API and worker must agree on the internal token, for instance).

ROOT_ENV=".env"
MONGO_PASSWORD="$(keep "$ROOT_ENV" MONGO_PASSWORD "$(gen 24 32)" generated)"
REDIS_PASSWORD="$(keep "$ROOT_ENV" REDIS_PASSWORD "$(gen 24 32)" generated)"
TYPESENSE_API_KEY="$(keep "$ROOT_ENV" TYPESENSE_API_KEY "$(gen 32 48)" generated)"
INTERNAL_SERVICE_TOKEN="$(keep "$ROOT_ENV" INTERNAL_SERVICE_TOKEN "$(gen 32 48)" generated)"
AUTH_SECRET="$(keep "$ROOT_ENV" AUTH_SECRET "$(gen 32 44)" generated)"
SESSION_SECRET="$(keep "$ROOT_ENV" SESSION_SECRET "$(gen 32 44)" generated)"

# Connection strings. Pasted values survive re-runs; the defaults point at the
# local Docker stack.
MONGO_URI="$(keep "$ROOT_ENV" MONGO_URI "mongodb://localhost:27117/ghanageo?replicaSet=rs0&directConnection=true")"
MONGO_DB="$(keep "$ROOT_ENV" MONGO_DB "ghanageo")"
REDIS_URL="$(keep "$ROOT_ENV" REDIS_URL "redis://localhost:6679")"
TYPESENSE_URL="$(keep "$ROOT_ENV" TYPESENSE_URL "http://localhost:8108")"

# ---------------------------------------------- generated PRODUCTION secrets
# Deliberately DIFFERENT values from local. Reusing a development secret in
# production means a leaked dev machine is a production compromise, and it is
# the single most common way a small team gets breached.
PROD_ENV=".env.production"
PROD_INTERNAL_SERVICE_TOKEN="$(keep "$PROD_ENV" INTERNAL_SERVICE_TOKEN "$(gen 32 48)" generated)"
PROD_AUTH_SECRET="$(keep "$PROD_ENV" AUTH_SECRET "$(gen 32 44)" generated)"
PROD_SESSION_SECRET="$(keep "$PROD_ENV" SESSION_SECRET "$(gen 32 44)" generated)"

# Production connections — yours to paste, preserved once set.
PROD_MONGO_URI="$(keep "$PROD_ENV" MONGO_URI PASTE_MONGODB_ATLAS_URI)"
PROD_MONGO_DB="$(keep "$PROD_ENV" MONGO_DB ghanageo)"
PROD_REDIS_URL="$(keep "$PROD_ENV" REDIS_URL PASTE_REDIS_URL)"
PROD_TYPESENSE_URL="$(keep "$PROD_ENV" TYPESENSE_URL PASTE_TYPESENSE_URL)"
PROD_TYPESENSE_API_KEY="$(keep "$PROD_ENV" TYPESENSE_API_KEY PASTE_TYPESENSE_API_KEY)"
PROD_SENTRY_DSN="$(keep "$PROD_ENV" SENTRY_DSN PASTE_SENTRY_DSN)"
PROD_RESEND_API_KEY="$(keep "$PROD_ENV" RESEND_API_KEY PASTE_RESEND_API_KEY)"

echo "Generating environment files…"
echo

# ------------------------------------------------------------------- root
write "$ROOT_ENV" \
"# GhanaGeo — local development. NEVER COMMIT. Regenerate: ./scripts/gen-env.sh" \
"# Docker Compose reads this file automatically." \
"" \
"GHANAGEO_ENV=local" \
"" \
"# --- Connections. These are the single source of truth: services/api and" \
"# --- services/worker inherit them unless their own .env overrides." \
"#" \
"# --- Paste your own MongoDB URI here to use Atlas (or any remote cluster)" \
"# --- instead of the local container. Include the database name, and keep" \
"# --- directConnection=true ONLY for the local single-node replica set —" \
"# --- Atlas needs it omitted so the driver can discover the whole set." \
"MONGO_URI=\"$MONGO_URI\"" \
"MONGO_DB=\"$MONGO_DB\"" \
"REDIS_URL=\"$REDIS_URL\"" \
"TYPESENSE_URL=\"$TYPESENSE_URL\"" \
"" \
"# --- Generated secrets (safe to rotate with --rotate) ---" \
"MONGO_PASSWORD=$MONGO_PASSWORD" \
"REDIS_PASSWORD=$REDIS_PASSWORD" \
"TYPESENSE_API_KEY=$TYPESENSE_API_KEY" \
"INTERNAL_SERVICE_TOKEN=$INTERNAL_SERVICE_TOKEN" \
"AUTH_SECRET=$AUTH_SECRET" \
"SESSION_SECRET=$SESSION_SECRET" \
"" \
"# --- Local service ports. This machine runs other projects, so GhanaGeo" \
"# --- claims a dedicated block. Change here and in docker-compose.yml together." \
"MONGO_PORT=27117" \
"REDIS_PORT=6679" \
"TYPESENSE_PORT=8108" \
"API_HTTP_PORT=8180" \
"API_GRPC_PORT=9190"

# ------------------------------------------------------------ root production
write "$PROD_ENV" \
"# GhanaGeo — PRODUCTION. NEVER COMMIT. Paste your values here." \
"#" \
"# This is the single place to paste production configuration. The per-service" \
"# .env.production files inherit these when you re-run ./scripts/gen-env.sh." \
"#" \
"# In production, prefer loading these into the platform (Vercel, Render) over" \
"# shipping a file. A .env.production on a server is a file that can leak." \
"" \
"GHANAGEO_ENV=production" \
"" \
"# ── PASTE: MongoDB Atlas ────────────────────────────────────────────────" \
"# Atlas → Database → Connect → Drivers → Go. Include the database name." \
"# Do NOT carry over directConnection=true: that is only correct for the" \
"# local single-node replica set. Atlas needs it omitted so the driver can" \
"# discover the whole set and fail over." \
"MONGO_URI=\"$PROD_MONGO_URI\"" \
"MONGO_DB=\"$PROD_MONGO_DB\"" \
"" \
"# ── PASTE: Render Key Value (Redis) ─────────────────────────────────────" \
"# Render dashboard → your Key Value instance → Connect." \
"#   Internal URL  redis://...  use this when the API also runs on Render:" \
"#                 it stays on Render private networking and is not exposed." \
"#   External URL  rediss://... TLS, needed only from outside Render." \
"# Prefer the internal URL. It is faster and never leaves their network." \
"REDIS_URL=\"$PROD_REDIS_URL\"" \
"" \
"# ── PASTE: Typesense Cloud, or your own node ────────────────────────────" \
"TYPESENSE_URL=\"$PROD_TYPESENSE_URL\"" \
"TYPESENSE_API_KEY=$PROD_TYPESENSE_API_KEY" \
"" \
"# ── PASTE: observability and transactional mail ─────────────────────────" \
"SENTRY_DSN=\"$PROD_SENTRY_DSN\"" \
"RESEND_API_KEY=$PROD_RESEND_API_KEY" \
"RESEND_FROM_EMAIL=noreply@digitalghana.dev" \
"" \
"# --- Generated for production. DIFFERENT from your local values, on purpose:" \
"# --- reusing a development secret in production turns a leaked laptop into a" \
"# --- production compromise. Rotate with: ./scripts/gen-env.sh --rotate" \
"INTERNAL_SERVICE_TOKEN=$PROD_INTERNAL_SERVICE_TOKEN" \
"AUTH_SECRET=$PROD_AUTH_SECRET" \
"SESSION_SECRET=$PROD_SESSION_SECRET" \
"" \
"# --- Public hosts ---" \
"API_ALLOWED_ORIGINS=\"https://geo.digitalghana.dev,https://sandbox.geo.digitalghana.dev,https://console.geo.digitalghana.dev,https://admin.geo.digitalghana.dev\"" \
"API_HTTP_PORT=8080" \
"API_GRPC_PORT=9090" \
"API_LOG_LEVEL=info"

# -------------------------------------------------------------- services/api
API_ENV="services/api/.env"
write "$API_ENV" \
"# GhanaGeo API — local. NEVER COMMIT." \
"GHANAGEO_ENV=local" \
"API_LOG_LEVEL=debug" \
"API_HTTP_PORT=8180" \
"API_GRPC_PORT=9190" \
"" \
"# Local Mongo runs without auth, so no credentials appear here. directConnection" \
"# is required for a single-node replica set reached from outside Docker." \
"MONGO_URI=\"$MONGO_URI\"" \
"MONGO_DB=\"$MONGO_DB\"" \
"" \
"REDIS_URL=\"$REDIS_URL\"" \
"" \
"TYPESENSE_URL=\"$TYPESENSE_URL\"" \
"TYPESENSE_API_KEY=ghanageo_local_dev_only" \
"" \
"# Origins allowed to call the API from a browser. An unlisted origin gets no" \
"# CORS header at all, so the browser blocks it. Never use a wildcard." \
"API_ALLOWED_ORIGINS=\"http://localhost:3100,http://localhost:3101,http://localhost:3102,http://localhost:3103\"" \
"" \
"INTERNAL_SERVICE_TOKEN=$INTERNAL_SERVICE_TOKEN"

write "services/api/.env.production" \
"# GhanaGeo API — production. NEVER COMMIT. Load these into Render, not a file." \
"GHANAGEO_ENV=production" \
"API_LOG_LEVEL=info" \
"API_HTTP_PORT=8080" \
"API_GRPC_PORT=9090" \
"" \
"# ── PASTE: MongoDB Atlas connection string ──────────────────────────────" \
"# Atlas → Database → Connect → Drivers → Go. Include the database name." \
"MONGO_URI=\"$PROD_MONGO_URI\"" \
"MONGO_DB=\"$PROD_MONGO_DB\"" \
"" \
"REDIS_URL=\"$PROD_REDIS_URL\"" \
"" \
"TYPESENSE_URL=\"$PROD_TYPESENSE_URL\"" \
"TYPESENSE_API_KEY=$PROD_TYPESENSE_API_KEY" \
"" \
"API_ALLOWED_ORIGINS=\"https://geo.digitalghana.dev,https://sandbox.geo.digitalghana.dev,https://console.geo.digitalghana.dev,https://admin.geo.digitalghana.dev\"" \
"" \
"INTERNAL_SERVICE_TOKEN=$INTERNAL_SERVICE_TOKEN" \
"" \
"# ── PASTE: observability and mail ───────────────────────────────────────" \
"SENTRY_DSN=\"$PROD_SENTRY_DSN\"" \
"RESEND_API_KEY=$PROD_RESEND_API_KEY" \
"RESEND_FROM_EMAIL=noreply@digitalghana.dev"

# ------------------------------------------------------------ services/worker
write "services/worker/.env" \
"# GhanaGeo worker — local. NEVER COMMIT." \
"GHANAGEO_ENV=local" \
"WORKER_LOG_LEVEL=debug" \
"MONGO_URI=\"$MONGO_URI\"" \
"MONGO_DB=\"$MONGO_DB\"" \
"REDIS_URL=\"$REDIS_URL\"" \
"TYPESENSE_URL=\"$TYPESENSE_URL\"" \
"TYPESENSE_API_KEY=ghanageo_local_dev_only" \
"INTERNAL_SERVICE_TOKEN=$INTERNAL_SERVICE_TOKEN" \
"" \
"# Where ETL source dumps are cached between runs." \
"INGEST_CACHE_DIR=./.cache/ingest"

write "services/worker/.env.production" \
"# GhanaGeo worker — production. NEVER COMMIT." \
"GHANAGEO_ENV=production" \
"WORKER_LOG_LEVEL=info" \
"MONGO_URI=\"$PROD_MONGO_URI\"" \
"MONGO_DB=\"$PROD_MONGO_DB\"" \
"REDIS_URL=\"$PROD_REDIS_URL\"" \
"TYPESENSE_URL=\"$PROD_TYPESENSE_URL\"" \
"TYPESENSE_API_KEY=$PROD_TYPESENSE_API_KEY" \
"INTERNAL_SERVICE_TOKEN=$PROD_INTERNAL_SERVICE_TOKEN" \
"INGEST_CACHE_DIR=/var/cache/ghanageo/ingest" \
"SENTRY_DSN=\"$PROD_SENTRY_DSN\""

# --------------------------------------------------------------------- cli
#
# The CLI is a binary that runs on OTHER PEOPLE'S machines, so it has no
# deployed configuration and no .env.production — there is no environment of
# ours for it to run in.
#
# This file is a convenience for working ON the CLI. The binary deliberately
# does NOT auto-load it: a public tool that silently reads .env from whatever
# directory it is invoked in would pick up unrelated projects' secrets. Source
# it yourself, or use direnv:
#
#     cd cli && set -a && . ./.env && set +a
#
write "cli/.env" \
"# GhanaGeo CLI — local development only. NEVER COMMIT." \
"#" \
"# The binary does not read this automatically, by design. Source it:" \
"#   set -a && . ./.env && set +a" \
"" \
"# Point the CLI at the local API instead of production." \
"GHANAGEO_BASE_URL=\"http://localhost:8180/v1\"" \
"" \
"# Optional. GhanaGeo is free, so the CLI works with no key at all; this only" \
"# identifies heavy use for fair-use accounting. Issue one with:" \
"#   cd services/api && go run ./cmd/ghanageo-admin keys create \\" \
"#     --name 'cli local' --class SERVER --env test" \
"GHANAGEO_API_KEY=$(read_existing cli/.env GHANAGEO_API_KEY)" \
"" \
"# Set to any value to disable colour (https://no-color.org)." \
"NO_COLOR="

# ------------------------------------------------------------------- Next apps
#
# CRITICAL: in Next.js, ONLY variables prefixed NEXT_PUBLIC_ reach the browser,
# and they are baked into the client bundle at build time. Anything secret must
# NOT carry that prefix. A GhanaGeo browser API key is safe to expose because it
# is origin-restricted and scope-limited by construction; a server key is not.

next_app() {
  local dir="$1" port="$2" host="$3" desc="$4"

  # Preserve keys already issued for this app. Without this, every re-run
  # blanked them and the app silently lost its credentials — which would have
  # made "safe to re-run" untrue in exactly the way that costs an afternoon.
  local browser_key server_key prod_browser_key prod_server_key
  browser_key="$(read_existing "$dir/.env" NEXT_PUBLIC_GHANAGEO_BROWSER_KEY)"
  server_key="$(read_existing "$dir/.env" GHANAGEO_SERVER_KEY)"
  prod_browser_key="$(keep "$dir/.env.production" NEXT_PUBLIC_GHANAGEO_BROWSER_KEY PASTE_BROWSER_KEY)"
  prod_server_key="$(keep "$dir/.env.production" GHANAGEO_SERVER_KEY PASTE_SERVER_KEY)"

  write "$dir/.env" \
"# $desc — local. NEVER COMMIT." \
"NODE_ENV=development" \
"PORT=$port" \
"" \
"# --- Public: baked into the browser bundle. Nothing secret here. ---" \
"NEXT_PUBLIC_GHANAGEO_API_URL=\"http://localhost:8180/v1\"" \
"NEXT_PUBLIC_GHANAGEO_GRAPHQL_URL=\"http://localhost:8180/graphql\"" \
"NEXT_PUBLIC_SITE_URL=\"http://localhost:$port\"" \
"NEXT_PUBLIC_ENVIRONMENT=local" \
"" \
"# A browser key is origin-restricted and scope-limited, so exposing it is" \
"# safe by construction. Create one with:" \
"#   cd services/api && go run ./cmd/ghanageo-admin keys create \\\\" \
"#     --name '$desc local' --class BROWSER --origins http://localhost:$port" \
"NEXT_PUBLIC_GHANAGEO_BROWSER_KEY=$browser_key" \
"" \
"# --- Server-only: never prefixed NEXT_PUBLIC_. ---" \
"AUTH_SECRET=$AUTH_SECRET" \
"SESSION_SECRET=$SESSION_SECRET" \
"GHANAGEO_SERVER_KEY=$server_key" \
"INTERNAL_SERVICE_TOKEN=$INTERNAL_SERVICE_TOKEN"

  write "$dir/.env.production" \
"# $desc — production. NEVER COMMIT. Load these into Vercel, not a file." \
"NODE_ENV=production" \
"" \
"# --- Public: baked into the browser bundle. Nothing secret here. ---" \
"NEXT_PUBLIC_GHANAGEO_API_URL=\"https://api.geo.digitalghana.dev/v1\"" \
"NEXT_PUBLIC_GHANAGEO_GRAPHQL_URL=\"https://api.geo.digitalghana.dev/graphql\"" \
"NEXT_PUBLIC_SITE_URL=\"https://$host\"" \
"NEXT_PUBLIC_ENVIRONMENT=production" \
"" \
"# ── PASTE: a BROWSER-class key restricted to https://$host ──" \
"NEXT_PUBLIC_GHANAGEO_BROWSER_KEY=$prod_browser_key" \
"" \
"# ── PASTE: analytics and error reporting (optional) ──" \
"NEXT_PUBLIC_SENTRY_DSN=$(keep "$dir/.env.production" NEXT_PUBLIC_SENTRY_DSN PASTE_SENTRY_DSN_OR_LEAVE_BLANK)" \
"" \
"# --- Server-only: never prefixed NEXT_PUBLIC_. ---" \
"AUTH_SECRET=$AUTH_SECRET" \
"SESSION_SECRET=$SESSION_SECRET" \
"# ── PASTE: a SERVER-class key. This must never reach a client bundle. ──" \
"GHANAGEO_SERVER_KEY=$prod_server_key" \
"INTERNAL_SERVICE_TOKEN=$INTERNAL_SERVICE_TOKEN"
}

next_app apps/web      3100 "geo.digitalghana.dev"         "GhanaGeo marketing and docs"
next_app apps/sandbox  3101 "sandbox.geo.digitalghana.dev" "GhanaGeo sandbox"
next_app apps/portal   3102 "console.geo.digitalghana.dev" "GhanaGeo developer portal"
next_app apps/admin    3103 "admin.geo.digitalghana.dev"   "GhanaGeo admin"

echo
echo "Done. Files are chmod 600 and covered by .gitignore."
echo "Search for PASTE_ to find what still needs your values:"
echo "  grep -rn 'PASTE_' --include='.env*' . | grep -v node_modules"
