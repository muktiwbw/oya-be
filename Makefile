GO ?= go
MOCKERY_VERSION ?= v3.5.5
MOCKERY ?= $(GO) run github.com/vektra/mockery/v3@$(MOCKERY_VERSION)
DOCKER ?= $(shell if command -v podman >/dev/null 2>&1; then echo "podman"; elif command -v docker >/dev/null 2>&1; then echo "docker"; else echo "docker"; fi)
APP_IMAGE ?= oya:integration
TEST_IMAGE ?= oya-integration:latest
APP_BINARY ?= bin/oya
DOCKER_SOCKET ?= $(shell \
	if [ -n "$$DOCKER_HOST" ]; then \
		s=$$(printf '%s\n' "$$DOCKER_HOST" | sed -e 's|^unix://||'); \
		if [ -S "$$s" ]; then echo "$$s"; exit 0; fi; \
	fi; \
	if command -v podman >/dev/null 2>&1; then \
		for t in '{{.ConnectionInfo.PodmanSocket.Path}}' '{{(index . 0).ConnectionInfo.PodmanSocket.Path}}'; do \
			s=$$(podman machine inspect --format "$$t" 2>/dev/null | sed -e 's|^unix://||'); \
			if [ -n "$$s" ] && [ -S "$$s" ]; then echo "$$s"; exit 0; fi; \
		done; \
	fi; \
	if [ -S "$$HOME/.local/share/containers/podman/machine/podman.sock" ]; then \
		echo "$$HOME/.local/share/containers/podman/machine/podman.sock"; \
	elif [ -S "$$HOME/.docker/run/docker.sock" ]; then \
		echo "$$HOME/.docker/run/docker.sock"; \
	elif [ -S "/var/run/docker.sock" ]; then \
		echo "/var/run/docker.sock"; \
	else \
		echo "/var/run/docker.sock"; \
	fi)
ACT_SOCKET := $(patsubst unix://%,%,$(DOCKER_SOCKET))
ACT ?= DOCKER_HOST="unix://$(ACT_SOCKET)" act --container-architecture linux/amd64 -P ubuntu-latest=catthehacker/ubuntu:act-latest --container-daemon-socket $(ACT_SOCKET)

.PHONY: help fmt vet test test-race generate-mocks sqlc integration-test check build run migrate dev-db dev-db-stop docker-build act-list act-dryrun act-test

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
	@echo "  make migrate          Apply pending database migrations using Goose"
	@echo "  make docker-build     Build the application image"
	@echo "  make act-dryrun       Dry-run and validate both workflows locally (-n)"
	@echo "  make act-list         List workflow jobs recognized by act"

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
		--network host \
		-v "$(DOCKER_SOCKET)":/var/run/docker.sock \
		-e TESTCONTAINERS_HOST_OVERRIDE=$(TESTCONTAINERS_HOST) \
		-e TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock \
		-e TESTCONTAINERS_RYUK_DISABLED=true \
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
	$(DOCKER) compose run --rm migrate

dev-db:
	$(DOCKER) compose up -d postgres

dev-db-stop:
	$(DOCKER) compose down postgres

docker-build:
	$(DOCKER) build -t $(APP_IMAGE) .

act-list:
	$(ACT) -l

act-dryrun:
	@echo "==> Validating build-and-deploy workflow (push event)..."
	$(ACT) push -W .github/workflows/build-and-deploy.yaml -n
	@echo "==> Validating run-migration workflow (workflow_dispatch event)..."
	$(ACT) workflow_dispatch -W .github/workflows/run-migration.yaml -n

act-test: act-dryrun
