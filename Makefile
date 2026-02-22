.PHONY: build test migrate run-dev docker-up docker-down clean lint proto build-import-content

GO := go
GOFLAGS := -trimpath
BIN_DIR := bin

PROTOC := protoc
PROTO_DIR := api/proto
PROTO_GO_OUT := .
PROTO_MODULE := github.com/cory-johannsen/mud

# Build targets
build: proto build-frontend build-gameserver build-migrate build-import-content

build-frontend: proto
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/frontend ./cmd/frontend

build-gameserver: proto
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/gameserver ./cmd/gameserver

build-migrate: proto
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/migrate ./cmd/migrate

build-import-content:
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/import-content ./cmd/import-content

# Protobuf code generation
proto:
	$(PROTOC) --proto_path=$(PROTO_DIR) \
		--go_out=$(PROTO_GO_OUT) --go_opt=module=$(PROTO_MODULE) \
		--go-grpc_out=$(PROTO_GO_OUT) --go-grpc_opt=module=$(PROTO_MODULE) \
		$(PROTO_DIR)/game/v1/game.proto

# Test targets
# NOTE: postgres package uses bcrypt property tests which are slow under -race.
# Use a 10-minute timeout to accommodate the full suite.
test: build
	$(GO) test -race -count=1 -timeout=600s ./...

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
	docker compose -f deployments/docker/docker-compose.yml up --build

docker-down:
	docker compose -f deployments/docker/docker-compose.yml down

# Maintenance
clean:
	rm -rf $(BIN_DIR) coverage.out coverage.html

lint:
	golangci-lint run ./...
