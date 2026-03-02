SHELL := /bin/bash

ROOT := $(CURDIR)
TS_DIR := packages/ballast-typescript
PY_DIR := packages/ballast-python
GO_DIR := packages/ballast-go
GO_BIN := $(GO_DIR)/ballast
GO_GOCACHE ?= /tmp/go-build
PY_RUN_ENV := BALLAST_REPO_ROOT=$(ROOT) PYTHONPATH=$(ROOT)/$(PY_DIR)

.PHONY: help build build-all build-typescript build-python build-go

help:
	@echo "Available targets:"
	@echo "  make build-typescript  Build TypeScript package and print run command"
	@echo "  make build-python      Validate Python package and print run command"
	@echo "  make build-go          Build Go program and print run command"
	@echo "  make build-all         Build all three packages/programs"
	@echo "  make build             Alias for build-all"

build: build-all

build-all: build-typescript build-python build-go
	@echo ""
	@echo "All three builds completed."

build-typescript:
	@echo "==> Building TypeScript package (@everydaydevopsio/ballast)"
	pnpm --filter @everydaydevopsio/ballast run build
	@echo ""
	@echo "Run it with:"
	@echo "  node $(ROOT)/$(TS_DIR)/bin/ballast.js install --target cursor --all"

build-python:
	@echo "==> Validating Python package (ballast-python)"
	python3 -m py_compile $(PY_DIR)/ballast/cli.py $(PY_DIR)/ballast/__main__.py
	@echo ""
	@echo "Run it from any repo directory with:"
	@echo "  $(PY_RUN_ENV) python3 -m ballast install --target cursor --all"

build-go:
	@echo "==> Building Go program (ballast-go)"
	cd $(GO_DIR) && env GOCACHE=$(GO_GOCACHE) go build ./cmd/ballast
	@echo ""
	@echo "Run it with:"
	@echo "  $(ROOT)/$(GO_BIN) install --target cursor --all"
