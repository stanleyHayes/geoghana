#!/bin/sh
set -eu

SDK=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
REPO=$(CDPATH='' cd -- "$SDK/../.." && pwd)
cd "$SDK"

php_version=$(php -r 'echo PHP_MAJOR_VERSION.".".PHP_MINOR_VERSION;')
case "$php_version" in 8.2|8.3|8.4|8.5) ;; *) echo "PHP 8.2-8.5 required, found $php_version" >&2; exit 1 ;; esac
composer validate --strict --no-check-publish
composer install --no-interaction --prefer-dist
vendor/bin/phpunit
vendor/bin/phpstan analyse --memory-limit=1G
vendor/bin/pint --test

(cd "$REPO" && shasum -a 256 -c sdks/php/contract.lock)
(cd "$REPO" && ruby tools/conformance/generate.rb --check && ruby tools/conformance/validate.rb)

artifacts=$(mktemp -d "${TMPDIR:-/tmp}/ghanageo-php-artifacts.XXXXXX")
consumer=$(mktemp -d "${TMPDIR:-/tmp}/ghanageo-php-consumer.XXXXXX")
cleanup() { rm -rf "$artifacts" "$consumer"; }
trap cleanup EXIT HUP INT TERM
composer archive --format=zip --dir="$artifacts" >/dev/null
archive=$(find "$artifacts" -maxdepth 1 -name '*.zip' -print -quit)
test -n "$archive"
unzip -l "$archive" | grep -q 'src/Client.php'
if unzip -l "$archive" | grep -Eq '(^|/)(vendor|tests|conformance|consumer-smoke)/'; then echo 'development files leaked into release archive' >&2; exit 1; fi
mkdir "$artifacts/package"
unzip -q "$archive" -d "$artifacts/package"

cp consumer-smoke/composer.json consumer-smoke/smoke.php "$consumer/"
sed "s#\"url\": \"\.\.\"#\"url\": \"$artifacts/package\"#" "$consumer/composer.json" > "$consumer/composer.tmp"
mv "$consumer/composer.tmp" "$consumer/composer.json"
(cd "$consumer" && composer install --no-dev --no-interaction --prefer-dist && php smoke.php)

scan_files=$(find src config examples -type f -print)
# Paths are whitespace-free. The canonical scanner checks source and unpacked consumer artifact.
# shellcheck disable=SC2086
"$REPO/tools/conformance/scan-secrets" $scan_files README.md CONFORMANCE.md composer.json composer.lock "$archive" "$artifacts/package"

if [ -n "${CI:-}" ] && [ -z "${GHANAGEO_CONFORMANCE_URL:-}" ]; then
  echo 'GHANAGEO_CONFORMANCE_URL is required in CI.' >&2
  exit 1
fi
if [ -n "${GHANAGEO_CONFORMANCE_URL:-}" ]; then
  ruby "$REPO/tools/conformance/export_cases.rb" > "$artifacts/cases.json"
  php conformance/runner.php "$artifacts/cases.json" "$GHANAGEO_CONFORMANCE_URL" "$artifacts/report.json"
  (cd "$REPO" && ruby tools/conformance/validate.rb --report "$artifacts/report.json")
fi

echo 'PHP SDK verification passed (including Packagist archive metadata dry run).'
