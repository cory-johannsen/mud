.PHONY: build test test-fast test-postgres test-cover migrate run-dev docker-up docker-down clean lint proto build-import-content

GO := go
GOFLAGS := -trimpath
BIN_DIR := bin

PROTOC := protoc
PROTO_DIR := api/proto
PROTO_GO_OUT := .
PROTO_MODULE := github.com/cory-johannsen/mud

# Build targets
build: proto build-frontend build-gameserver build-migrate build-import-content build-setrole

build-frontend: proto
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/frontend ./cmd/frontend

build-gameserver: proto
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/gameserver ./cmd/gameserver

build-migrate: proto
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/migrate ./cmd/migrate

build-import-content:
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/import-content ./cmd/import-content

build-setrole: proto
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/setrole ./cmd/setrole

# Protobuf code generation
proto:
	$(PROTOC) --proto_path=$(PROTO_DIR) \
		--go_out=$(PROTO_GO_OUT) --go_opt=module=$(PROTO_MODULE) \
		--go-grpc_out=$(PROTO_GO_OUT) --go-grpc_opt=module=$(PROTO_MODULE) \
		$(PROTO_DIR)/game/v1/game.proto

# Packages that require Docker (testcontainers).
POSTGRES_PKG := github.com/cory-johannsen/mud/internal/storage/postgres

# All packages except the Docker-dependent one.
FAST_PKGS := $(shell go list ./... | grep -v '$(POSTGRES_PKG)')

# Test targets
#
# test-fast  — unit + integration tests, no Docker required (~5s)
# test-postgres — Docker-dependent postgres tests only; bcrypt property tests
#                 are slow so a 10-minute timeout is used
# test       — both suites; declared order ensures -j parallelism works because
#              both sub-targets depend on build, not on each other
test: test-fast test-postgres

test-fast: build
	$(GO) test -race -count=1 -timeout=300s $(FAST_PKGS)

test-postgres: build
	$(GO) test -race -count=1 -timeout=600s $(POSTGRES_PKG)

test-cover: build
	$(GO) test -race -count=1 -timeout=600s -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# Database
migrate:
	$(BIN_DIR)/migrate -config configs/dev.yaml

# Run
run-frontend:
	$(GO) run ./cmd/frontend -config configs/dev.yaml

run-gameserver:
	$(GO) run ./cmd/gameserver -config configs/dev.yaml

# Docker
docker-up:
	docker compose -f deployments/docker/docker-compose.yml up --build -d

docker-down:
	docker compose -f deployments/docker/docker-compose.yml down

# Maintenance
clean:
	rm -rf $(BIN_DIR) coverage.out coverage.html

lint:
	golangci-lint run ./...
