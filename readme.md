# Nube iO Developer Tools

A collection of developer tools for Nube iO plugin development and documentation workflows.

## Tools

### 📄 doc2pdf
Converts Markdown documents into branded PDF output with support for Mermaid diagrams, tables, and custom themes.

**Location:** `cmd/doc2pdf`

**Documentation:**
- [Usage Guide](docs/pdf-generator/usage.md)
- [Development Guide](docs/pdf-generator/devlopment.md)

### 🔌 fake-plugin-generator
Interactive TUI for scaffolding Nube iO plugin projects with a beautiful terminal interface.

**Location:** `cmd/fake-plugin-generator`

**Documentation:**
- [Usage Guide](docs/fake-plugin-generator/usage.md)
- [Development Guide](docs/fake-plugin-generator/development.md)

## Building

### Quick Start

Build all tools:
```bash
make build
```

Binaries will be output to `bin/`:
- `bin/doc2pdf`
- `bin/fake-plugin-generator`

### Make Targets

```bash
make build                    # Build all binaries
make clean                    # Remove build artifacts
make install                  # Install to GOPATH/bin
make test                     # Run tests
make fmt                      # Format code
make vet                      # Run go vet
make tidy                     # Tidy go.mod
make help                     # Show all targets
```

### Build Individual Tools

```bash
make bin/doc2pdf              # Build doc2pdf only
make bin/fake-plugin-generator # Build fake-plugin-generator only
```

### Manual Build

```bash
# Build to bin/ directory
go build -o bin/doc2pdf ./cmd/doc2pdf
go build -o bin/fake-plugin-generator ./cmd/fake-plugin-generator

# Or use go install
go install ./cmd/doc2pdf
go install ./cmd/fake-plugin-generator
```

## Usage

### doc2pdf

Convert a markdown file to PDF:
```bash
./bin/doc2pdf docs/input.md --output dist/output.pdf
```

See [docs/pdf-generator/usage.md](docs/pdf-generator/usage.md) for full documentation.

### fake-plugin-generator

Launch the interactive plugin generator:
```bash
./bin/fake-plugin-generator
```

See [docs/fake-plugin-generator/usage.md](docs/fake-plugin-generator/usage.md) for full documentation.

## Development

### Prerequisites

- Go 1.24.0 or later
- For doc2pdf Mermaid support: `pnpm install -g @mermaid-js/mermaid-cli`

### Project Structure

```
.
├── cmd/
│   ├── doc2pdf/              # PDF generator
│   └── fake-plugin-generator/
│       ├── main.go           # Entry point
│       └── internal/
│           ├── config/       # Configuration & constants
│           ├── generator/    # Plugin generation logic
│           └── ui/           # Bubble Tea UI
├── docs/                     # Documentation
├── bin/                      # Build output (gitignored)
├── Makefile                  # Build automation
└── go.mod                    # Go module definition
```

### Testing

Run all tests:
```bash
make test
```

Run tests for a specific package:
```bash
go test ./cmd/doc2pdf/...
go test ./cmd/fake-plugin-generator/...
```

### Contributing

1. Format your code: `make fmt`
2. Run linter: `make vet`
3. Run tests: `make test`
4. Build: `make build`

## License

Copyright © 2026 Nube iO
