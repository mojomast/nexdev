.PHONY: build test clean install install-user install-system uninstall lint fmt vet run help generate pi-ext-deps pi-ext-check pi-ext-build pi-ext-clean pi-ext-install-dev

# Build variables
BINARY_NAME=nexdev
VERSION?=0.1.0
BUILD_DIR=bin
GO=go
GOFLAGS=-v
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"
CMD_DIR=./cmd/nexdev
USER_PREFIX?=$(HOME)/.local
USER_BIN_DIR?=$(USER_PREFIX)/bin
USER_SHARE_DIR?=$(USER_PREFIX)/share/nexdev
PI_EXT_DIR=extensions/nexdev
PI_EXT_DIST=$(BUILD_DIR)/pi-extension

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
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

## test: Run all tests
test:
	@echo "Running tests..."
	$(GO) test -v -race -coverprofile=coverage.txt ./...

## generate: Regenerate checked-in contract code
generate:
	@echo "Generating OpenAPI contract code..."
	@mkdir -p api/generated
	$(GO) tool oapi-codegen -generate types -package generated -o api/generated/nexdev_api.gen.go api/openapi.yaml

pi-ext-deps:
	@if [ -f $(PI_EXT_DIR)/package-lock.json ]; then \
		npm ci --ignore-scripts --prefix $(PI_EXT_DIR); \
	else \
		npm install --ignore-scripts --prefix $(PI_EXT_DIR); \
	fi

## pi-ext-check: Compile-check the Pi extension
pi-ext-check: pi-ext-deps
	@echo "Checking Pi extension..."
	npm --prefix $(PI_EXT_DIR) run check

## pi-ext-build: Build/prep the Pi extension distribution
pi-ext-build: pi-ext-check
	@echo "Preparing Pi extension distribution..."
	@rm -rf $(PI_EXT_DIST)
	@mkdir -p $(PI_EXT_DIST)
	@cp $(PI_EXT_DIR)/index.ts $(PI_EXT_DIST)/index.ts
	@cp $(PI_EXT_DIR)/client.ts $(PI_EXT_DIST)/client.ts
	@cp $(PI_EXT_DIR)/menu.ts $(PI_EXT_DIST)/menu.ts
	@cp $(PI_EXT_DIR)/steer.ts $(PI_EXT_DIST)/steer.ts
	@cp $(PI_EXT_DIR)/types.ts $(PI_EXT_DIST)/types.ts
	@cp $(PI_EXT_DIR)/widgets.ts $(PI_EXT_DIST)/widgets.ts
	@cp $(PI_EXT_DIR)/package.json $(PI_EXT_DIST)/package.json
	@cp $(PI_EXT_DIR)/tsconfig.json $(PI_EXT_DIST)/tsconfig.json

## pi-ext-clean: Remove Pi extension build artifacts
pi-ext-clean:
	@echo "Cleaning Pi extension artifacts..."
	@rm -rf $(PI_EXT_DIST)
	@rm -rf $(PI_EXT_DIR)/node_modules
	@rm -f $(PI_EXT_DIR)/tsconfig.tsbuildinfo

## pi-ext-install-dev: Explicitly install the Pi extension into PI_EXTENSION_DEV_DIR
pi-ext-install-dev: pi-ext-build
	@test -n "$(PI_EXTENSION_DEV_DIR)" || (echo "Set PI_EXTENSION_DEV_DIR to an explicit Pi dev extension directory" >&2; exit 2)
	@case "$(PI_EXTENSION_DEV_DIR)" in "/"|"."|".."|"" ) echo "Refusing unsafe PI_EXTENSION_DEV_DIR=$(PI_EXTENSION_DEV_DIR)" >&2; exit 2;; esac
	@mkdir -p "$(PI_EXTENSION_DEV_DIR)"
	@rm -rf "$(PI_EXTENSION_DEV_DIR)/nexdev"
	@ln -s "$(abspath $(PI_EXT_DIR))" "$(PI_EXTENSION_DEV_DIR)/nexdev"
	@echo "Installed dev Pi extension symlink: $(PI_EXTENSION_DEV_DIR)/nexdev -> $(abspath $(PI_EXT_DIR))"

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

## install: Build and install Nexdev plus Pi extension to ~/.local
install: install-user

## install-user: Build and install Nexdev plus Pi extension to ~/.local
install-user: build pi-ext-build
	@echo "Installing $(BINARY_NAME) to $(USER_BIN_DIR)..."
	@mkdir -p $(USER_BIN_DIR)
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(USER_BIN_DIR)/$(BINARY_NAME)
	@echo "Installing Pi extension to $(USER_SHARE_DIR)/pi-extension..."
	@mkdir -p $(USER_SHARE_DIR)
	@rm -rf $(USER_SHARE_DIR)/pi-extension
	@cp -R $(PI_EXT_DIST) $(USER_SHARE_DIR)/pi-extension

## install-system: Install binary to system PATH (requires sudo)
install-system: build pi-ext-build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin and Pi extension to /usr/local/share/nexdev/pi-extension..."
	@install -d /usr/local/bin /usr/local/share/nexdev
	@install -m 0755 $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@rm -rf /usr/local/share/nexdev/pi-extension
	@cp -R $(PI_EXT_DIST) /usr/local/share/nexdev/pi-extension

## uninstall: Remove from system PATH
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	@rm -f /usr/local/bin/$(BINARY_NAME) 2>/dev/null || true
	@rm -f $(USER_BIN_DIR)/$(BINARY_NAME) 2>/dev/null || true
	@rm -rf $(USER_SHARE_DIR)/pi-extension 2>/dev/null || true
	@rm -rf /usr/local/share/nexdev/pi-extension 2>/dev/null || true
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
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)
	GOOS=linux GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_DIR)
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR)
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)
	GOOS=windows GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe $(CMD_DIR)

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):$(VERSION) .

## docker-run: Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker run -it --rm $(BINARY_NAME):$(VERSION)
