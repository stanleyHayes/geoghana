#!/bin/sh
set -eu
ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
REPO=$(CDPATH='' cd -- "$ROOT/../.." && pwd)
(cd "$REPO" && shasum -a 256 -c sdks/java/contract.lock)
(cd "$REPO" && ruby tools/conformance/generate.rb --check && ruby tools/conformance/validate.rb)
"$ROOT/mvnw" clean install
FIRST_HASHES=$(mktemp)
SECOND_HASHES=$(mktemp)
find "$ROOT/client/target" "$ROOT/spring-boot-starter/target" -maxdepth 1 -type f -name '*.jar' -print | sort | xargs shasum -a 256 > "$FIRST_HASHES"
ISOLATED_REPO=$(mktemp -d)
"$ROOT/mvnw" -Dmaven.repo.local="$ISOLATED_REPO" clean install -DskipTests
"$ROOT/mvnw" -Dmaven.repo.local="$ISOLATED_REPO" -f "$ROOT/consumer-smoke/pom.xml" clean verify
"$ROOT/mvnw" clean package -DskipTests
find "$ROOT/client/target" "$ROOT/spring-boot-starter/target" -maxdepth 1 -type f -name '*.jar' -print | sort | xargs shasum -a 256 > "$SECOND_HASHES"
diff -u "$FIRST_HASHES" "$SECOND_HASHES"
CENTRAL_DRY_RUN=$(mktemp -d)
"$ROOT/mvnw" deploy -DskipTests -DaltDeploymentRepository="central-dry-run::default::file:$CENTRAL_DRY_RUN"
if find "$CENTRAL_DRY_RUN" -type f | grep -Eq 'ghanageo-java-(examples|conformance)'; then echo 'non-publishable module reached staged repository' >&2; exit 1; fi
test -n "$(find "$CENTRAL_DRY_RUN" -type f -name 'ghanageo-java-*-sources.jar' -print -quit)"
test -n "$(find "$CENTRAL_DRY_RUN" -type f -name 'ghanageo-java-*-javadoc.jar' -print -quit)"
test -n "$(find "$CENTRAL_DRY_RUN" -type f -name 'ghanageo-spring-boot-starter-*-sources.jar' -print -quit)"
test -n "$(find "$CENTRAL_DRY_RUN" -type f -name 'ghanageo-spring-boot-starter-*-javadoc.jar' -print -quit)"
"$ROOT/gradlew" clean test build
if rg -n '(ghg_[A-Za-z0-9_-]{16,}|Bearer [A-Za-z0-9_-]{16,})' "$ROOT" -g '!verify.sh' -g '!target/**' -g '!build/**'; then echo 'key-shaped secret found' >&2; exit 1; fi
ruby "$REPO/tools/conformance/export_cases.rb" > "$ROOT/.build-tools/cases.json"
if [ -n "${GHANAGEO_CONFORMANCE_URL:-}" ]; then
  "$ROOT/mvnw" -q -pl conformance exec:java -Dexec.mainClass=dev.ghanageo.conformance.ConformanceRunner -Dexec.args="$ROOT/.build-tools/cases.json $GHANAGEO_CONFORMANCE_URL $ROOT/java-conformance-report.json"
  (cd "$REPO" && ruby tools/conformance/validate.rb --report sdks/java/java-conformance-report.json)
fi
echo 'Contract lock, generation, unit/build, consumer, Central dry-run, and configured live-conformance checks passed.'
