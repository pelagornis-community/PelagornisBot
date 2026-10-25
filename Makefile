SHELL := /bin/sh
GOCACHE ?= /tmp/communityos-gocache
GOMODCACHE ?= /tmp/communityos-gomod
GO := GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go
COMPOSE := docker compose -f deployments/compose.yaml

.DEFAULT_GOAL := help
.PHONY: help fmt test lint check build build-arm64 run migrate compose-config up down logs smoke clean

help:
	@printf '%s\n' 'CommunityOS targets:' '  make fmt             Format Go source.' '  make test            Run all unit tests.' '  make lint            Run go vet.' '  make check           Check formatting, tests, and vet.' '  make build           Build bin/communityos.' '  make build-arm64     Cross-build Linux ARM64.' '  make migrate         Apply PostgreSQL migrations only.' '  make run             Start CommunityOS.' '  make compose-config  Validate Compose.' '  make up|down|logs    Manage Compose.' '  make smoke           Offline smoke checks.'

fmt:
	gofmt -w $$(find cmd internal -name '*.go' -type f)
test:
	$(GO) test ./...
lint:
	$(GO) vet ./...
check:
	@unformatted="$$(gofmt -l $$(find cmd internal -name '*.go' -type f))"; if [ -n "$$unformatted" ]; then printf 'Unformatted Go files:\n%s\n' "$$unformatted"; exit 1; fi
	$(MAKE) test
	$(MAKE) lint
build:
	mkdir -p bin
	$(GO) build -trimpath -o bin/communityos ./cmd/communityos
build-arm64:
	mkdir -p bin
	GOOS=linux GOARCH=arm64 $(GO) build -trimpath -o bin/communityos-linux-arm64 ./cmd/communityos
run:
	$(GO) run ./cmd/communityos
migrate:
	$(GO) run ./cmd/migrate
compose-config:
	$(COMPOSE) config
up:
	$(COMPOSE) up -d --build
down:
	$(COMPOSE) down
logs:
	$(COMPOSE) logs -f --tail=100
smoke:
	$(GO) test ./internal/... ./cmd/...
	$(MAKE) build-arm64
clean:
	rm -f bin/communityos bin/communityos-linux-arm64
