all: build

build:
	go build -o bin/mcpdemo .

run-stdio: build
	./bin/mcpdemo

run-http: build
	./bin/mcpdemo --transport http

test: build
	@echo "Starting demo in HTTP mode..."
	@./bin/mcpdemo --transport http --mcp-addr :8181 --api-addr :9001 & \
		PID=$$!; \
		sleep 1; \
		echo "=== API health ==="; \
		curl -sf http://localhost:9001/api/v1/health; echo; \
		echo "=== List devices ==="; \
		curl -sf http://localhost:9001/api/v1/devices; echo; \
		echo "=== MCP initialize ==="; \
		curl -sf -X POST http://localhost:8181/ \
			-H "Content-Type: application/json" \
			-H "Accept: application/json, text/event-stream" \
			-d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"0.1"}}}'; \
		echo; \
		kill $$PID 2>/dev/null; \
		echo "=== PASSED ==="

clean:
	rm -rf bin
