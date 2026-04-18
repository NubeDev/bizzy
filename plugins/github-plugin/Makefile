## github-plugin — bizzy plugin for GitHub

ROOT_DIR := $(abspath ../..)
BIN_DIR := $(ROOT_DIR)/bin
BINARY := $(BIN_DIR)/github-plugin

.PHONY: build run

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BINARY) .

run:
	@if [ -z "$$GITHUB_TOKEN" ]; then echo "ERROR: GITHUB_TOKEN is not set"; exit 1; fi
	go run .
