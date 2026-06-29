.PHONY: build test clean install install-system uninstall lint fmt vet run help

# Build variables
BINARY_NAME=geoffrussy
VERSION?=0.1.0
BUILD_DIR=bin
GO=go
GOFLAGS=-v
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"

# Default target
all: build

## help: Display this help message
help:
	@echo "Available targets:"
	@grep -E '^##' Makefile | sed 's/##//'

## build: Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/geoffrussy

## test: Run all tests
test:
	@echo "Running tests..."
	$(GO) test -v -race -coverprofile=coverage.txt ./...

## test-unit: Run unit tests only
test-unit:
	@echo "Running unit tests..."
	$(GO) test -v -race -short ./...

## test-property: Run property-based tests
test-property:
	@echo "Running property tests..."
	$(GO) test -v -tags=property ./test/properties/...

## test-integration: Run integration tests
test-integration:
	@echo "Running integration tests..."
	$(GO) test -v -tags=integration ./test/integration/...

## test-pipeline: Run full pipeline integration test with ZAI provider
test-pipeline:
	@echo "Running full pipeline test with ZAI provider (glm-4.7)..."
	@echo "This will use real API calls and may take several minutes..."
	INTEGRATION_TEST=1 $(GO) test -v -tags=integration -run TestFullPipelineZAI ./test/integration/... -timeout 10m

## test-pipeline-simple: Run simple devplan execution test
test-pipeline-simple:
	@echo "Running simple devplan test..."
	INTEGRATION_TEST=1 $(GO) test -v -tags=integration -run TestSimpleDevPlanExecution ./test/integration/... -timeout 5m

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.txt
	@$(GO) clean

## install: Install binary to GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME) to GOPATH/bin..."
	$(GO) install $(LDFLAGS) ./cmd/geoffrussy

## install-system: Install binary to system PATH (requires sudo)
install-system:
	@echo "Installing $(BINARY_NAME) to system..."
	@./install.sh

## uninstall: Remove from system PATH
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	@rm -f /usr/local/bin/$(BINARY_NAME) 2>/dev/null || true
	@rm -f $(HOME)/bin/$(BINARY_NAME) 2>/dev/null || true
	@echo "$(BINARY_NAME) removed from system"

## lint: Run linters
lint:
	@echo "Running linters..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run ./...

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GO) vet ./...

## run: Build and run the binary
run: build
	@echo "Running $(BINARY_NAME)..."
	@$(BUILD_DIR)/$(BINARY_NAME)

## build-all: Build for all platforms
build-all:
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/geoffrussy
	GOOS=linux GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/geoffrussy
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/geoffrussy
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/geoffrussy
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/geoffrussy
	GOOS=windows GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe ./cmd/geoffrussy

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):$(VERSION) .

## docker-run: Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker run -it --rm $(BINARY_NAME):$(VERSION)
