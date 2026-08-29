#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_DB="${MONGO_DB:-ghanageo}"
DRILL_SUFFIX="$(date -u +%Y%m%dT%H%M%SZ)-$$"
SCRATCH_DB="ghanageo_restore_drill_${DRILL_SUFFIX}"
STAGE="$(mktemp -d)"
ARCHIVE="$STAGE/${SOURCE_DB}.archive"

if [[ ! "$SOURCE_DB" =~ ^[A-Za-z0-9_-]+$ ]]; then
  echo "MONGO_DB contains unsupported characters" >&2
  exit 2
fi
if [[ ! "$SCRATCH_DB" =~ ^ghanageo_restore_drill_[A-Za-z0-9_-]+$ ]]; then
  echo "refusing unsafe scratch database name" >&2
  exit 2
fi

mongo() {
  docker compose -f "$ROOT/docker-compose.yml" exec -T mongo "$@"
}

cleanup() {
  mongo mongosh --quiet --eval "db.getSiblingDB('$SCRATCH_DB').dropDatabase()" >/dev/null 2>&1 || true
  rm -rf "$STAGE"
}
trap cleanup EXIT

mongo mongodump --quiet --db "$SOURCE_DB" --archive > "$ARCHIVE"
[[ -s "$ARCHIVE" ]] || { echo "backup archive is empty" >&2; exit 1; }

ARCHIVE_SHA="$(shasum -a 256 "$ARCHIVE" | awk '{print $1}')"
ARCHIVE_BYTES="$(wc -c < "$ARCHIVE" | tr -d ' ')"

mongo mongorestore --quiet --archive \
  --nsFrom "${SOURCE_DB}.*" --nsTo "${SCRATCH_DB}.*" < "$ARCHIVE"

COUNTS="$(mongo mongosh --quiet --eval "
const source = db.getSiblingDB('$SOURCE_DB');
const restored = db.getSiblingDB('$SCRATCH_DB');
const names = source.getCollectionNames().sort();
const result = names.map(name => ({name, source: source[name].countDocuments({}), restored: restored[name].countDocuments({})}));
if (result.some(item => item.source !== item.restored)) { print(JSON.stringify(result)); quit(3); }
print(JSON.stringify(result));
")"

VALIDATORS="$(mongo mongosh --quiet --eval "
const source = db.getSiblingDB('$SOURCE_DB').getCollectionInfos().filter(item => !item.name.startsWith('system.'));
const restored = db.getSiblingDB('$SCRATCH_DB').getCollectionInfos().filter(item => !item.name.startsWith('system.'));
const withValidators = restored.filter(item => item.options && item.options.validator && Object.keys(item.options.validator).length > 0).length;
if (source.length !== restored.length) quit(4);
print(JSON.stringify({collections: restored.length, validators: withValidators}));
")"

echo "restore_drill=passed"
echo "source_database=$SOURCE_DB"
echo "scratch_database=$SCRATCH_DB"
echo "archive_bytes=$ARCHIVE_BYTES"
echo "archive_sha256=$ARCHIVE_SHA"
echo "collection_counts=$COUNTS"
echo "schema_validation=$VALIDATORS"
