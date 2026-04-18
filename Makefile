# Build output directory
BIN_DIR := bin
DATA_DIR := data

# Ports
SERVER_PORT := 8090

# Binary names
DOC2PDF_BIN := $(BIN_DIR)/doc2pdf
FAKE_PLUGIN_GEN_BIN := $(BIN_DIR)/fake-plugin-generator
NUBE_SERVER_BIN := $(BIN_DIR)/nube-server
NUBE_CLI_BIN := $(BIN_DIR)/nube
NATS_SERVER_BIN := $(shell which nats-server)
NATS_PORT := 4225
NATS_STORE_DIR := $(DATA_DIR)/nats

# Go build flags
GO_BUILD := go build
BUILD_FLAGS := -trimpath -ldflags="-s -w"

.PHONY: all build clean install test help
.PHONY: server start stop reset test-api
.PHONY: nats nats-stop
.PHONY: $(NUBE_SERVER_BIN) $(NUBE_CLI_BIN)

all: build

## build: Build all binaries
build: $(NUBE_SERVER_BIN) $(NUBE_CLI_BIN)

## nube-server: Build nube-server binary
$(NUBE_SERVER_BIN):
	@echo "Building nube-server..."
	@mkdir -p $(BIN_DIR)
	$(GO_BUILD) $(BUILD_FLAGS) -o $(NUBE_SERVER_BIN) ./cmd/nube-server

## nube-cli: Build nube CLI binary
$(NUBE_CLI_BIN):
	@echo "Building nube CLI..."
	@mkdir -p $(BIN_DIR)
	$(GO_BUILD) $(BUILD_FLAGS) -o $(NUBE_CLI_BIN) ./cmd/nube

# ---- Server management ----

## server: Start nube-server (foreground)
server: $(NUBE_SERVER_BIN)
	NUBE_ADDR=:$(SERVER_PORT) NUBE_DATA_DIR=$(DATA_DIR) $(NUBE_SERVER_BIN)

## start: Start nube-server (foreground)
start: $(NUBE_SERVER_BIN)
	@fuser -k $(SERVER_PORT)/tcp 2>/dev/null || true
	@mkdir -p $(DATA_DIR)
	@echo "Starting nube-server on :$(SERVER_PORT)..."
	@echo ""
	@echo "  nube-server   → http://localhost:$(SERVER_PORT)"
	@echo "  health check  → http://localhost:$(SERVER_PORT)/health"
	@echo ""
	@echo "Press Ctrl+C to stop."
	@echo ""
	NUBE_ADDR=:$(SERVER_PORT) NUBE_DATA_DIR=$(DATA_DIR) $(NUBE_SERVER_BIN)

## stop: Stop background servers
stop:
	@echo "Stopping servers..."
	@-pkill -f "$(NUBE_SERVER_BIN)" 2>/dev/null || true
	@echo "Stopped."

## nats: Start a standalone NATS server with JetStream on 127.0.0.1:4225
nats:
	@mkdir -p $(NATS_STORE_DIR)
	@echo "Starting NATS server on 127.0.0.1:$(NATS_PORT) (JetStream, store=$(NATS_STORE_DIR))..."
	@echo "Press Ctrl+C to stop."
	$(NATS_SERVER_BIN) --addr 127.0.0.1 --port $(NATS_PORT) --jetstream --store_dir $(NATS_STORE_DIR)

## nats-stop: Stop the standalone NATS server
nats-stop:
	@-pkill -f "nats-server" 2>/dev/null || true
	@echo "NATS stopped."

## reset: Wipe data directory and stop servers
reset: stop
	@echo "Wiping data..."
	@rm -f $(DATA_DIR)/workspaces.json $(DATA_DIR)/users.json $(DATA_DIR)/app_installs.json
	@echo "Clean slate."

## test-api: Run the full API test script against running servers
test-api:
	@bash scripts/test-api.sh $(SERVER_PORT)

## test: Run Go unit + integration tests
test:
	@echo "Running tests..."
	go test -count=1 ./pkg/... ./tests/...

## clean: Remove build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BIN_DIR)

## install: Install binaries to GOPATH/bin
install:
	@echo "Installing binaries..."
	go install ./cmd/nube-server

## fmt: Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

## tidy: Tidy go.mod
tidy:
	@echo "Tidying go.mod..."
	go mod tidy

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'
