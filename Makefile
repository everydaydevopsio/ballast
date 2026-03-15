SHELL := /bin/bash

ROOT := $(CURDIR)
TS_DIR := packages/ballast-typescript
PY_DIR := packages/ballast-python
GO_DIR := packages/ballast-go
GO_WRAPPER_DIR := cli/ballast
GO_BIN := $(GO_DIR)/ballast-go
GO_WRAPPER_BIN := $(GO_WRAPPER_DIR)/ballast
GO_GOCACHE ?= /tmp/go-build
UV_CACHE_DIR ?= /tmp/uv-cache
PY_RUN_ENV := UV_CACHE_DIR=$(UV_CACHE_DIR) BALLAST_REPO_ROOT=$(ROOT) PYTHONPATH=$(ROOT)/$(PY_DIR) uv run --python 3.12

.PHONY: help build build-all build-typescript build-python build-go build-cli

help:
	@echo "Available targets:"
	@echo "  make build-typescript  Build TypeScript package and print run command"
	@echo "  make build-python      Validate Python package and print run command"
	@echo "  make build-go          Build Go installer CLI (ballast-go) and print run command"
	@echo "  make build-cli         Build wrapper CLI (ballast) and print run command"
	@echo "  make build-all         Build all packages/programs"
	@echo "  make build             Alias for build-all"

build: build-all

build-all: build-typescript build-python build-go build-cli
	@echo ""
	@echo "All builds completed."

build-typescript:
	@echo "==> Building TypeScript package (@everydaydevopsio/ballast)"
	pnpm --filter @everydaydevopsio/ballast run build
	@echo ""
	@echo "Run it with:"
	@echo "  node $(ROOT)/$(TS_DIR)/bin/ballast.js install --target cursor --all"

build-python:
	@echo "==> Validating Python package (ballast-python)"
	UV_CACHE_DIR=$(UV_CACHE_DIR) uv run --python 3.12 python -m py_compile $(PY_DIR)/ballast/cli.py $(PY_DIR)/ballast/__main__.py
	@echo ""
	@echo "Run it from any repo directory with:"
	@echo "  $(PY_RUN_ENV) python -m ballast install --target cursor --all"

build-go:
	@echo "==> Building Go installer CLI (ballast-go)"
	cd $(GO_DIR) && env GOCACHE=$(GO_GOCACHE) go build -o $(ROOT)/$(GO_BIN) ./cmd/ballast-go
	@echo ""
	@echo "Run it with:"
	@echo "  $(ROOT)/$(GO_BIN) install --target cursor --all"

build-cli:
	@echo "==> Building wrapper CLI (ballast)"
	cd $(GO_WRAPPER_DIR) && env GOCACHE=$(GO_GOCACHE) go build -o $(ROOT)/$(GO_WRAPPER_BIN) .
	@echo ""
	@echo "Run it with:"
	@echo "  $(ROOT)/$(GO_WRAPPER_BIN) install --target cursor --all"
