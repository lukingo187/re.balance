# re.balance — build and install the `rebal` binary.

BINARY    := rebal
CMD       := ./cmd/rebal
BINARY_SC := rebal-sc
CMD_SC    := ./cmd/rebal-sc
GOBIN     := $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN  := $(shell go env GOPATH)/bin
endif

# Stamp the version from git (falls back to "dev" outside a checkout).
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build install test vet fmt clean run build-sc install-sc snapshot

## build: compile ./rebal in the repo root
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

## install: install `rebal` into $(GOBIN)
install:
	go install -ldflags "$(LDFLAGS)" $(CMD)
	@echo "installed to $(GOBIN)/$(BINARY)"
	@case ":$$PATH:" in *":$(GOBIN):"*) ;; \
	  *) echo "note: $(GOBIN) is not on your PATH — add it to run 'rebal' directly";; esac

## build-sc: compile the optional Scalable-sync addon (./rebal-sc) — not part
## of `build`; rebal itself never depends on this. Requires the `sc` CLI
## (https://github.com/ScalableCapital/scalable-cli) on PATH to actually run.
build-sc:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_SC) $(CMD_SC)

## install-sc: install `rebal-sc` into $(GOBIN)
install-sc:
	go install -ldflags "$(LDFLAGS)" $(CMD_SC)
	@echo "installed to $(GOBIN)/$(BINARY_SC)"

## test: run the full test suite
test:
	go test ./... -count=1

## vet: run go vet
vet:
	go vet ./...

## fmt: format all Go files
fmt:
	go fmt ./...

## snapshot: build all release archives into ./dist without publishing
snapshot:
	goreleaser release --snapshot --clean

## clean: remove the built binaries and release output
clean:
	rm -rf $(BINARY) $(BINARY_SC) dist

## run: build and run (pass args with ARGS=..., e.g. make run ARGS="plan --months 6")
run: build
	./$(BINARY) $(ARGS)
