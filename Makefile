.PHONY: build test test-fast test-postgres test-cover migrate run-dev docker-up docker-down clean lint proto build-import-content build-devserver kind-up kind-down docker-push helm-install helm-upgrade helm-uninstall k8s-up k8s-down k8s-redeploy k8s-metallb deps wire wire-check

deps:
	$(GO) mod tidy
	$(GO) install github.com/google/wire/cmd/wire

wire:
	wire ./cmd/gameserver/... ./cmd/devserver/... ./cmd/frontend/...

wire-check:
	wire ./cmd/gameserver/... ./cmd/devserver/... ./cmd/frontend/...
	git diff --exit-code cmd/gameserver/wire_gen.go cmd/devserver/wire_gen.go cmd/frontend/wire_gen.go

GO := go
VERSION := $(shell cat VERSION 2>/dev/null || echo dev)-$(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
VERSION_PKG := github.com/cory-johannsen/mud/internal/version
LDFLAGS := -X $(VERSION_PKG).Version=$(VERSION)
GOFLAGS := -trimpath -ldflags "$(LDFLAGS)"
BIN_DIR := bin

PROTOC := protoc
PROTO_DIR := api/proto
PROTO_GO_OUT := .
PROTO_MODULE := github.com/cory-johannsen/mud

# Build targets
build: proto build-frontend build-gameserver build-devserver build-migrate build-import-content build-setrole

build-devserver: proto
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/devserver ./cmd/devserver

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
	DOCKER_HOST=unix:///var/run/docker.sock $(GO) test -race -count=1 -timeout=300s $(POSTGRES_PKG) -args -rapid.checks=3

test-cover: build
	$(GO) test -race -count=1 -timeout=600s -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# Database
CONFIG ?= configs/dev.yaml

migrate: build-migrate
	$(BIN_DIR)/migrate -config $(CONFIG)

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

# Kubernetes / Kind
REGISTRY    := registry.johannsen.cloud:5000
DB_USER     := mud
DB_PASSWORD := mud
HELM_NAMESPACE := mud
IMAGE_TAG   := $(shell git rev-parse --short HEAD 2>/dev/null || echo latest)
HELM_CHART  := deployments/k8s/mud
HELM_RELEASE := mud
HELM_VALUES := $(HELM_CHART)/values-prod.yaml

kind-up:
	./deployments/k8s/mud/scripts/cluster-up.sh

k8s-metallb:
	kubectl apply -f deployments/k8s/metallb/metallb-native.yaml
	kubectl rollout status deployment/controller -n metallb-system --timeout=120s
	kubectl rollout status daemonset/speaker -n metallb-system --timeout=120s
	kubectl apply -f deployments/k8s/metallb/ipaddresspool.yaml
	kubectl apply -f deployments/k8s/metallb/l2advertisement.yaml

kind-down:
	./deployments/k8s/mud/scripts/cluster-down.sh

docker-push:
	docker build --build-arg VERSION=$(VERSION) -t $(REGISTRY)/mud-gameserver:$(IMAGE_TAG) -f deployments/docker/Dockerfile.gameserver .
	docker push $(REGISTRY)/mud-gameserver:$(IMAGE_TAG)
	docker build --build-arg VERSION=$(VERSION) -t $(REGISTRY)/mud-frontend:$(IMAGE_TAG) -f deployments/docker/Dockerfile.frontend .
	docker push $(REGISTRY)/mud-frontend:$(IMAGE_TAG)

helm-install:
	helm install $(HELM_RELEASE) $(HELM_CHART) \
		--namespace $(HELM_NAMESPACE) \
		--create-namespace \
		--values $(HELM_VALUES) \
		--set db.user=$(DB_USER) \
		--set db.password=$(DB_PASSWORD) \
		--set image.tag=$(IMAGE_TAG)

helm-upgrade:
	helm upgrade $(HELM_RELEASE) $(HELM_CHART) \
		--namespace $(HELM_NAMESPACE) \
		--values $(HELM_VALUES) \
		--set db.user=$(DB_USER) \
		--set db.password=$(DB_PASSWORD) \
		--set image.tag=$(IMAGE_TAG)

helm-uninstall:
	helm uninstall $(HELM_RELEASE) --namespace $(HELM_NAMESPACE)

k8s-up: kind-up k8s-metallb docker-push helm-install

k8s-down: helm-uninstall kind-down

k8s-redeploy: docker-push helm-upgrade

# Maintenance
clean:
	rm -rf $(BIN_DIR) coverage.out coverage.html

lint:
	golangci-lint run ./...
