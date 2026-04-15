# Binaries will be built into the ./bin directory
all:: bin/openapi-mcp bin/openmcp

bin/openapi-mcp:: $(shell find . -path ./bin -prune -o -name '*.go' -print)
	@mkdir -p bin
	go build -o bin/openapi-mcp ./cmd/openapi-mcp

bin/openmcp:: $(shell find . -path ./bin -prune -o -name '*.go' -print)
	@mkdir -p bin
	go build -o bin/openmcp ./cmd/openmcp

test::
	go test ./...

clean::
	rm -f bin/openapi-mcp bin/openmcp
