.PHONY: dev up down logs seed validate test lint api contracts docs-sdk-check production-blueprint-check production-preflight conformance conformance-grpc conformance-matrix sdk-python-verify sdk-go-verify sdk-wave1-verify sdk-dart-verify sdk-flutter-verify sdk-java-verify sdk-wave2-verify sdk-dotnet-verify sdk-php-verify sdk-wave3-verify sdk-matrix-verify cli cli-release cli-release-verify check-licensed-data restore-drill perf perf-indexes

up:      ## Start MongoDB (rs0), Redis and Typesense
	docker compose up -d --wait
down:
	docker compose down
logs:
	docker compose logs -f
seed:    ## Import the bootstrap dataset (16 regions / 261 districts / 16 places)
	cd services/api && go run ./cmd/ghanageo-admin data seed --file ../../data/seed-data/manifest.json --environment local
validate:
	cd services/api && go run ./cmd/ghanageo-admin data validate --dataset seed
api:
	cd services/api && go run ./cmd/api
contracts:  ## Regenerate error code artifacts and validate all three contracts
	cd services/api && go run ./cmd/gencontracts ../..
	npx --yes @redocly/cli@latest lint contracts/openapi/v1.yaml
	npx --yes @bufbuild/buf@latest lint
	node -e "const{buildSchema}=require('graphql');buildSchema(require('fs').readFileSync('contracts/graphql/schema.graphql','utf8'));console.log('graphql schema ok')"

docs-sdk-check: ## Reject generated SDK documentation drift and compile every extracted example
	pnpm docs:sdk:check

production-blueprint-check: ## Validate Render Blueprint structure without treating it as a deploy gate
	ruby scripts/production-preflight.rb --blueprint-only
	ruby scripts/test-production-preflight.rb

production-preflight: ## Fail-closed Render gate; API_ENV and WORKER_ENV must be populated production files
	@test -n "$(API_ENV)" || (echo "API_ENV is required"; exit 2)
	@test -n "$(WORKER_ENV)" || (echo "WORKER_ENV is required"; exit 2)
	ruby scripts/production-preflight.rb --env "ghanageo-api=$(API_ENV)" --env "ghanageo-worker=$(WORKER_ENV)"

conformance:  ## Validate the SDK contract, runner kit and TypeScript reference adapter
	./tools/conformance/check.sh

conformance-grpc: ## Run the canonical native gRPC fixture and generated Go client
	REPORT=$(or $(REPORT),/tmp/ghanageo-go-grpc-conformance.json) ./scripts/run-grpc-conformance.sh

conformance-matrix: ## Validate all downloaded GEO-24.1M runner reports
	@test -n "$(REPORT_DIR)" || (echo "REPORT_DIR is required"; exit 2)
	ruby tools/conformance/validate_matrix.rb $(REPORT_DIR)

sdk-python-verify: ## Locked Python release checks plus strict live-fixture conformance
	PYTHON_VERSION=$(or $(PYTHON_VERSION),3.12) REPORT=$(or $(REPORT),$(if $(REPORT_DIR),$(REPORT_DIR)/python-$(or $(PYTHON_VERSION),3.12).json,/tmp/ghanageo-python-conformance.json)) ./scripts/verify-python-wave1.sh

sdk-go-verify: ## Isolated Go release checks plus strict live-fixture conformance
	GOWORK=off GOTOOLCHAIN=local ./sdks/go/verify.sh
	GOWORK=off GOTOOLCHAIN=local ./scripts/run-wave1-conformance.sh go $(or $(REPORT),$(if $(REPORT_DIR),$(REPORT_DIR)/go-rest.json,/tmp/ghanageo-go-conformance.json))
	REPORT=$(or $(GRPC_REPORT),$(if $(REPORT_DIR),$(REPORT_DIR)/go-grpc.json,/tmp/ghanageo-go-grpc-conformance.json)) ./scripts/run-grpc-conformance.sh

sdk-wave1-verify: sdk-python-verify sdk-go-verify ## Verify both wave-1 SDKs

sdk-dart-verify: ## Verify Dart core and its strict live-fixture report
	cd sdks/dart && dart pub get
	cd sdks/dart && dart run tool/verify.dart
	cd sdks/dart && dart run tool/run_conformance.dart $(or $(REPORT),$(if $(REPORT_DIR),$(REPORT_DIR)/dart.json,/tmp/ghanageo-dart-conformance.json))

sdk-flutter-verify: ## Verify the Flutter companion package
	cd sdks/dart/packages/ghanageo_flutter && dart tool/verify.dart

sdk-java-verify: ## Verify Java 21, contract locks, consumers and strict live conformance
	REPORT=$(or $(REPORT),$(if $(REPORT_DIR),$(REPORT_DIR)/java.json,/tmp/ghanageo-java-conformance.json)) ./scripts/verify-java-wave2.sh

sdk-wave2-verify: sdk-dart-verify sdk-flutter-verify sdk-java-verify ## Verify Dart, Flutter and Java

sdk-dotnet-verify: ## Verify net8 release artifacts and strict live conformance
	$(if $(or $(REPORT),$(REPORT_DIR)),REPORT=$(or $(REPORT),$(REPORT_DIR)/dotnet.json)) ./scripts/verify-wave3-sdk.sh dotnet

sdk-php-verify: ## Verify the current PHP matrix leg and strict live conformance
	$(if $(or $(REPORT),$(REPORT_DIR)),REPORT=$(or $(REPORT),$(REPORT_DIR)/php-$(or $(PHP_VERSION),8.5).json)) ./scripts/verify-wave3-sdk.sh php

sdk-wave3-verify: sdk-dotnet-verify sdk-php-verify ## Verify .NET and PHP

sdk-matrix-verify: sdk-wave1-verify sdk-wave2-verify sdk-wave3-verify ## Verify every GEO-24.1M SDK lane
	$(if $(REPORT_DIR),ruby tools/conformance/validate_matrix.rb $(REPORT_DIR),:)

test:
	cd services/api && go test ./...
	cd services/worker && go test ./...
	pnpm test
lint:
	cd services/api && go vet ./...
	cd services/worker && go vet ./...
	pnpm lint
cli:      ## Build the public CLI for this machine
	cd cli && go build -o ../bin/ghanageo .
	@echo "built ./bin/ghanageo — try: ./bin/ghanageo search accra"
cli-release: ## Cross-compile the public CLI for every supported platform
	cd cli && ./build.sh $(VERSION) $(or $(API_VERSION),v1)

cli-release-verify: ## Dry-run the complete CLI release bundle locally
	./scripts/verify-cli-release.sh $(or $(VERSION),0.0.0-test) $(or $(API_VERSION),v1)

check-licensed-data: ## Reject unlicensed GhanaPostGPS payloads from distributable data
	./scripts/check-ghanapost-exclusion.test.sh
	./scripts/check-ghanapost-exclusion.sh

restore-drill: ## Prove Mongo backup and isolated scratch restoration mechanics
	./scripts/restore-drill.sh

dev: up seed api

perf: ## p95 acceptance gate (Spec 22.4). Needs a running API and an elevated key.
	@test -n "$$PERF_API_KEY" || (echo "PERF_API_KEY is required — anonymous callers get 120 burst units and the gate would measure fair-use limiting, not latency."; exit 1)
	cd services/api && PERF_TARGET_URL=$${PERF_TARGET_URL:-http://localhost:8180} go test ./perf/ -run TestP95Targets -v -timeout 15m

perf-indexes: ## Assert every hot query uses an index, never a collection scan.
	cd services/api && PERF_MONGO_URI=$${PERF_MONGO_URI:-mongodb://127.0.0.1:27117/ghanageo?replicaSet=rs0&directConnection=true} \
		go test ./perf/ -run TestHotQueriesUseIndexes -v
