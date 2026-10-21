GO ?= go
DOCKER ?= docker
APP_IMAGE ?= oya:integration
TEST_IMAGE ?= oya-integration:latest
APP_BINARY ?= bin/oya
DOCKER_SOCKET ?= /var/run/docker.sock
TESTCONTAINERS_HOST ?= host.docker.internal
MOCKERY_VERSION ?= v3.5.5
MOCKERY ?= $(GO) run github.com/vektra/mockery/v3@$(MOCKERY_VERSION)
SQLC_VERSION ?= v1.27.0
SQLC ?= $(GO) run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)
FLYWAY_IMAGE ?= flyway/flyway:latest
FLYWAY_URL ?= jdbc:postgresql://localhost:15432/oya?sslmode=disable
FLYWAY_USER ?= postgres
FLYWAY_PASSWORD ?= postgres

.PHONY: help fmt vet test test-race generate-mocks sqlc integration-test check build run migrate dev-db dev-db-stop docker-build

help:
	@echo "Available commands:"
	@echo "  make dev-db          Start local PostgreSQL container using Docker Compose"
	@echo "  make dev-db-stop     Stop local PostgreSQL container"
	@echo "  make fmt              Format Go source files"
	@echo "  make vet              Run go vet"
	@echo "  make test             Run non-containerized tests"
	@echo "  make test-race        Run non-containerized tests with the race detector"
	@echo "  make generate-mocks   Regenerate test mocks using pinned Mockery"
	@echo "  make sqlc             Run sqlc generate"
	@echo "  make integration-test Run Docker and Testcontainers integration tests"
	@echo "  make check            Run formatting checks, vet, and tests"
	@echo "  make build            Build the application binary"
	@echo "  make run              Run the application locally"
	@echo "  make migrate          Apply pending database migrations using Flyway"
	@echo "  make docker-build     Build the application image"

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

generate-mocks:
	GOTOOLCHAIN="$$($(GO) env GOVERSION)" $(MOCKERY) --config .mockery.yml

sqlc:
	$(SQLC) generate

integration-test:
	$(DOCKER) build -t $(APP_IMAGE) .
	$(DOCKER) build -f Dockerfile.test -t $(TEST_IMAGE) .
	$(DOCKER) run --rm \
		-v "$(DOCKER_SOCKET)":/var/run/docker.sock \
		--add-host=host.docker.internal:host-gateway \
		-e TESTCONTAINERS_HOST_OVERRIDE=$(TESTCONTAINERS_HOST) \
		-e TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock \
		-e OYA_TEST_IMAGE=$(APP_IMAGE) \
		$(TEST_IMAGE) \
		go test -count=1 -tags=integration ./tests/integration/...

check:
	@unformatted="$$(gofmt -l cmd internal tests)" || exit $$?; \
		test -z "$$unformatted" || { printf '%s\n' "Go files are not formatted:" "$$unformatted"; exit 1; }
	$(GO) vet ./...
	$(GO) test ./...

build:
	mkdir -p $(dir $(APP_BINARY))
	$(GO) build -o $(APP_BINARY) ./cmd/oya

run:
	$(GO) run ./cmd/oya

migrate:
	$(DOCKER) run --rm --net=host -v "$$(pwd)/migrations":/flyway/sql $(FLYWAY_IMAGE) -url="$(FLYWAY_URL)" -user=$(FLYWAY_USER) -password=$(FLYWAY_PASSWORD) migrate

dev-db:
	$(DOCKER) compose up -d postgres

dev-db-stop:
	$(DOCKER) compose down

docker-build:
	$(DOCKER) build -t $(APP_IMAGE) .
