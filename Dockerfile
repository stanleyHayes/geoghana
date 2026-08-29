FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.work go.work.sum ./
COPY services/api/go.mod services/api/go.sum ./services/api/
COPY services/worker/go.mod services/worker/go.sum ./services/worker/
RUN cd services/api && go mod download && cd ../worker && go mod download
COPY services/api/ ./services/api/
COPY services/worker/ ./services/worker/
RUN cd services/api \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ghanageo-api ./cmd/api \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ghanageo-admin ./cmd/ghanageo-admin \
    && cd ../worker \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ghanageo-worker ./cmd/worker

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install --yes --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 65532 --home-dir /nonexistent --shell /usr/sbin/nologin ghanageo
WORKDIR /app
COPY --from=build /out/ghanageo-api /ghanageo-api
COPY --from=build /out/ghanageo-admin /ghanageo-admin
COPY --from=build /out/ghanageo-worker /ghanageo-worker
COPY data/seed-data/ /app/data/seed-data/
USER ghanageo:ghanageo
EXPOSE 8180 9190
ENTRYPOINT ["/ghanageo-api"]
