.PHONY: all build clean install test help
.PHONY: server fakeserver start stop reset test-api

# Build output directory
BIN_DIR := bin
DATA_DIR := data
APPS_DIR := apps

# Ports
SERVER_PORT := 8090
FAKE_PORT := 9001

# Binary names
DOC2PDF_BIN := $(BIN_DIR)/doc2pdf
FAKE_PLUGIN_GEN_BIN := $(BIN_DIR)/fake-plugin-generator
NUBE_SERVER_BIN := $(BIN_DIR)/nube-server
FAKESERVER_BIN := $(BIN_DIR)/fakeserver
NUBE_CLI_BIN := $(BIN_DIR)/nube

# Go build flags
GO_BUILD := go build
BUILD_FLAGS := -trimpath -ldflags="-s -w"

all: build

## build: Build all binaries
build: $(DOC2PDF_BIN) $(FAKE_PLUGIN_GEN_BIN) $(NUBE_SERVER_BIN) $(FAKESERVER_BIN) $(NUBE_CLI_BIN)

## doc2pdf: Build doc2pdf binary
$(DOC2PDF_BIN):
	@echo "Building doc2pdf..."
	@mkdir -p $(BIN_DIR)
	$(GO_BUILD) $(BUILD_FLAGS) -o $(DOC2PDF_BIN) ./cmd/doc2pdf

## fake-plugin-generator: Build fake-plugin-generator binary
$(FAKE_PLUGIN_GEN_BIN):
	@echo "Building fake-plugin-generator..."
	@mkdir -p $(BIN_DIR)
	$(GO_BUILD) $(BUILD_FLAGS) -o $(FAKE_PLUGIN_GEN_BIN) ./cmd/fake-plugin-generator

## nube-server: Build nube-server binary
$(NUBE_SERVER_BIN):
	@echo "Building nube-server..."
	@mkdir -p $(BIN_DIR)
	$(GO_BUILD) $(BUILD_FLAGS) -o $(NUBE_SERVER_BIN) ./cmd/nube-server

## fakeserver: Build fakeserver binary
$(FAKESERVER_BIN):
	@echo "Building fakeserver..."
	@mkdir -p $(BIN_DIR)
	$(GO_BUILD) $(BUILD_FLAGS) -o $(FAKESERVER_BIN) ./fakeserver

## nube-cli: Build nube CLI binary
$(NUBE_CLI_BIN):
	@echo "Building nube CLI..."
	@mkdir -p $(BIN_DIR)
	@cp api/openapi.yaml cmd/nube/openapi.yaml
	$(GO_BUILD) $(BUILD_FLAGS) -o $(NUBE_CLI_BIN) ./cmd/nube

# ---- Server management ----

## server: Start nube-server (foreground)
server: $(NUBE_SERVER_BIN)
	NUBE_ADDR=:$(SERVER_PORT) NUBE_APPS_DIR=$(APPS_DIR) NUBE_DATA_DIR=$(DATA_DIR) $(NUBE_SERVER_BIN)

## fakeserver: Start fakeserver (foreground)
fakeserver: $(FAKESERVER_BIN)
	$(FAKESERVER_BIN) -addr :$(FAKE_PORT)

## start: Start both servers in background, print PIDs
start: $(NUBE_SERVER_BIN) $(FAKESERVER_BIN)
	@mkdir -p $(DATA_DIR)
	@echo "Starting fakeserver on :$(FAKE_PORT)..."
	@$(FAKESERVER_BIN) -addr :$(FAKE_PORT) &
	@sleep 1
	@echo "Starting nube-server on :$(SERVER_PORT)..."
	@NUBE_ADDR=:$(SERVER_PORT) NUBE_APPS_DIR=$(APPS_DIR) NUBE_DATA_DIR=$(DATA_DIR) $(NUBE_SERVER_BIN) &
	@sleep 1
	@echo ""
	@echo "Servers running:"
	@echo "  nube-server   → http://localhost:$(SERVER_PORT)"
	@echo "  fakeserver    → http://localhost:$(FAKE_PORT)"
	@echo "  health check  → http://localhost:$(SERVER_PORT)/health"
	@echo ""
	@echo "Run 'make test-api' to exercise the full API flow."
	@echo "Run 'make stop' to shut down."

## stop: Stop background servers
stop:
	@echo "Stopping servers..."
	@-pkill -f "$(NUBE_SERVER_BIN)" 2>/dev/null || true
	@-pkill -f "$(FAKESERVER_BIN)" 2>/dev/null || true
	@echo "Stopped."

## reset: Wipe data directory and stop servers
reset: stop
	@echo "Wiping data..."
	@rm -f $(DATA_DIR)/workspaces.json $(DATA_DIR)/users.json $(DATA_DIR)/app_installs.json
	@echo "Clean slate."

## test-api: Run the full API test script against running servers
test-api:
	@bash scripts/test-api.sh $(SERVER_PORT) $(FAKE_PORT)

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
	go install ./cmd/doc2pdf
	go install ./cmd/fake-plugin-generator
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
