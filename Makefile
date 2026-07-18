# re.balance — build and install the `rebal` binary.

BINARY := rebal
CMD    := ./cmd/rebal
GOBIN  := $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN  := $(shell go env GOPATH)/bin
endif

# Stamp the version from git (falls back to "dev" outside a checkout).
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build install test vet fmt clean run

## build: compile ./rebal in the repo root
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

## install: install `rebal` into $(GOBIN)
install:
	go install -ldflags "$(LDFLAGS)" $(CMD)
	@echo "installed to $(GOBIN)/$(BINARY)"
	@case ":$$PATH:" in *":$(GOBIN):"*) ;; \
	  *) echo "note: $(GOBIN) is not on your PATH — add it to run 'rebal' directly";; esac

## test: run the full test suite
test:
	go test ./... -count=1

## vet: run go vet
vet:
	go vet ./...

## fmt: format all Go files
fmt:
	go fmt ./...

## clean: remove the built binary
clean:
	rm -f $(BINARY)

## run: build and run (pass args with ARGS=..., e.g. make run ARGS="plan --months 6")
run: build
	./$(BINARY) $(ARGS)
