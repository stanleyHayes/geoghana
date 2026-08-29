.PHONY: dev up down logs seed test lint api

up:      ## Start MongoDB (rs0), Redis and Typesense
	docker compose up -d --wait
down:
	docker compose down
logs:
	docker compose logs -f
seed:    ## Import the bootstrap dataset (16 regions / 261 districts / 16 places)
	cd services/api && go run ./cmd/ghanageo data seed --file ../../data/seed-data/manifest.json --environment local
validate:
	cd services/api && go run ./cmd/ghanageo data validate --dataset seed
api:
	cd services/api && go run ./cmd/api
test:
	cd services/api && go test ./... && pnpm test
lint:
	cd services/api && go vet ./... && pnpm lint
dev: up seed api
