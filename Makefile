.PHONY: build test migrate run-dev docker-up docker-down clean lint

GO := go
GOFLAGS := -trimpath
BIN_DIR := bin

# Build targets
build: build-devserver build-migrate

build-devserver:
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/devserver ./cmd/devserver

build-migrate:
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/migrate ./cmd/migrate

# Test targets
test:
	$(GO) test -race -count=1 ./...

test-cover:
	$(GO) test -race -count=1 -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# Database
migrate:
	$(BIN_DIR)/migrate -config configs/dev.yaml

# Run
run-dev:
	$(GO) run ./cmd/devserver -config configs/dev.yaml

# Docker
docker-up:
	docker compose -f deployments/docker/docker-compose.yml up -d

docker-down:
	docker compose -f deployments/docker/docker-compose.yml down

# Maintenance
clean:
	rm -rf $(BIN_DIR) coverage.out coverage.html

lint:
	golangci-lint run ./...
