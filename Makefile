.PHONY: dev up down logs seed test lint api

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
	cd services/api && go test ./... && pnpm test
lint:
	cd services/api && go vet ./... && pnpm lint
cli:      ## Build the public CLI for this machine
	cd cli && go build -o ../bin/ghanageo .
	@echo "built ./bin/ghanageo — try: ./bin/ghanageo search accra"
cli-release: ## Cross-compile the public CLI for every supported platform
	cd cli && ./build.sh $(VERSION)

dev: up seed api
