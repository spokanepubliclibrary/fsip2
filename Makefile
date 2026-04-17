.PHONY: build test docker clean install run lint test-unit test-integration test-e2e deps smoke-test docker-verify test-all

# Build variables
BINARY_NAME=fsip2
VERSION=1.0.0
REGISTRY=registry.gitlab.com/spokanepubliclibrary/folio-go-sip2
BUILD_DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.buildDate=$(BUILD_DATE) -X main.gitCommit=$(GIT_COMMIT)"

# Directories
BIN_DIR=bin
LOG_DIR=log
CMD_DIR=cmd/fsip2

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOVET=$(GOCMD) vet
GOFMT=$(GOCMD) fmt

all: deps lint test build

# Install dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Build the application
build: deps
	@echo "Building $(BINARY_NAME) v$(VERSION)..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) ./$(CMD_DIR)

# Install the binary to GOPATH/bin
install: build
	@echo "Installing $(BINARY_NAME)..."
	$(GOCMD) install $(LDFLAGS) ./$(CMD_DIR)

# Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	@mkdir -p $(LOG_DIR)
	./$(BIN_DIR)/$(BINARY_NAME) -config ./documentation/examples/basic.yaml

# Run all tests
test: test-unit

# Run unit tests
test-unit:
	@echo "Running unit tests..."
	$(GOTEST) -v -cover ./internal/... ./cmd/...

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v -tags=integration ./tests/integration/...

# Run e2e tests
test-e2e:
	@echo "Running e2e tests..."
	$(GOTEST) -v -tags=e2e ./tests/e2e/...

# Run all test types
test-all: test-unit test-integration test-e2e
	@echo "All tests passed!"

# Run smoke tests on binary
smoke-test: build
	@echo "Running smoke tests..."
	./$(BIN_DIR)/$(BINARY_NAME) -version
	@echo "Verifying version format..."
ifeq ($(OS),Windows_NT)
	@powershell -Command "if (./$(BIN_DIR)/$(BINARY_NAME).exe -version | Select-String -Pattern '^fsip2 version [0-9]+\.[0-9]+\.[0-9]+') { exit 0 } else { exit 1 }"
else
	@./$(BIN_DIR)/$(BINARY_NAME) -version | grep -E '^fsip2 version [0-9]+\.[0-9]+\.[0-9]+'
endif
	@echo "Smoke tests passed!"

# Verify Docker image
docker-verify: docker
	@echo "Verifying Docker image..."
	docker run --rm $(BINARY_NAME):latest -version
	@echo "Checking user..."
	docker inspect $(BINARY_NAME):latest --format='{{.Config.User}}' | grep 1001
	@echo "Checking exposed ports..."
	docker inspect $(BINARY_NAME):latest --format='{{.Config.ExposedPorts}}' | grep 6443
	docker inspect $(BINARY_NAME):latest --format='{{.Config.ExposedPorts}}' | grep 8081
	@echo "Docker verification passed!"

# Run all tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Lint and format code
lint:
	@echo "Running linters..."
	$(GOVET) ./...
	$(GOFMT) ./...

# Build Docker image
docker: build
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):$(VERSION) .

docker-tag: docker
	@echo "Tagging Docker image with version..."
	docker tag $(BINARY_NAME):$(VERSION) $(REGISTRY):$(VERSION)

docker-latest: docker
	@echo "Tagging Docker image as latest..."
	docker tag $(BINARY_NAME):$(VERSION) $(REGISTRY):latest

docker-push: docker-tag docker-latest
	@echo "Pushing Docker images to registry..."
	docker push $(REGISTRY):$(VERSION)
	docker push $(REGISTRY):latest

# Run Docker image
docker-run:
	@echo "Running Docker container..."
	docker run --rm -p 6443:6443 -p 8081:8081 \
		-v $(PWD)/documentation/examples:/etc/fsip2 \
		-v $(PWD)/log:/app/log \
		$(BINARY_NAME):latest

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)
	rm -rf $(LOG_DIR)/*.log
	rm -f coverage.out coverage.html
	$(GOCMD) clean

# Show help
help:
	@echo "FSIP2 Makefile targets:"
	@echo "  make build              - Build the application"
	@echo "  make install            - Install the binary to GOPATH/bin"
	@echo "  make run                - Run the application with example config"
	@echo "  make test               - Run all unit tests"
	@echo "  make test-unit          - Run unit tests"
	@echo "  make test-integration   - Run integration tests"
	@echo "  make test-e2e           - Run end-to-end tests"
	@echo "  make test-all           - Run all test types (unit, integration, e2e)"
	@echo "  make test-coverage      - Run tests with coverage report"
	@echo "  make smoke-test         - Run smoke tests on built binary"
	@echo "  make lint               - Run linters and format code"
	@echo "  make docker             - Build Docker image"
	@echo "  make docker-run         - Run Docker container"
	@echo "  make docker-verify      - Verify Docker image properties"
	@echo "  make docker-tag         - Tag Docker image with version"
	@echo "  make docker-latest      - Tag Docker image as latest"
	@echo "  make docker-push        - Push Docker images to registry"
	@echo "  make VERSION=1.2.3 docker-push - Build/Tag/Push Docker image with specific version"
	@echo "  make deps               - Download dependencies"
	@echo "  make clean              - Clean build artifacts"
	@echo "  make help               - Show this help message"
