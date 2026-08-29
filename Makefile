.PHONY: dev up down logs seed validate test lint api contracts cli cli-release cli-release-verify check-licensed-data restore-drill perf perf-indexes

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
