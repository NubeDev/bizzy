## github-plugin — bizzy plugin for GitHub

.PHONY: build run

build:
	go build -o github-plugin .

run:
	@if [ -z "$$GITHUB_TOKEN" ]; then echo "ERROR: GITHUB_TOKEN is not set"; exit 1; fi
	go run .
