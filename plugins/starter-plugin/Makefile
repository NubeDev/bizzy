ROOT_DIR := $(abspath ../..)
BIN_DIR := $(ROOT_DIR)/bin
BINARY := $(BIN_DIR)/starter-plugin

.PHONY: build run

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BINARY) .

run:
	go run .
