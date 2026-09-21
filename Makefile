# Cluster Vision — the Go side (cmd/, internal/, mcp/).
#
# The web/ side keeps its own npm scripts; see CLAUDE.md for that table. This
# file exists so the Go side has the same thing: one place that says which
# toolchain and which linter a workstation uses, and it is the one CI uses.

# Pin the Go toolchain to go.mod's `go` line for every target below. This is
# Go's equivalent of a virtualenv: with GOTOOLCHAIN set to an exact version the
# go command downloads that release once (into the module cache) and uses it,
# whatever `go` on PATH happens to be. Without the pin a NEWER local go is used
# silently — `go 1.25.7` in go.mod only forbids OLDER ones — so a workstation on
# 1.27 tested code that CI (setup-go, go-version-file: go.mod) builds on 1.25.
# Exported, so it also reaches anything the targets below shell out to.
GO_VERSION := $(shell awk '/^go /{print $$2}' go.mod)
export GOTOOLCHAIN := go$(GO_VERSION)

LOCALBIN ?= $(shell pwd)/bin

.PHONY: all
all: lint test build ## Lint, test and build, all under the pinned toolchain.

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: go-version
go-version: ## Print the pinned toolchain and what it resolves to.
	@echo "GOTOOLCHAIN=$(GOTOOLCHAIN)"
	@go version

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: lint
lint: golangci-lint ## Run golangci-lint at the version CI pins.
	$(GOLANGCI_LINT) run --timeout 5m

.PHONY: test
test: ## Run the Go tests, the same invocation CI runs.
	go test ./...

.PHONY: build
build: ## Build every package, the same invocation CI runs.
	go build ./...

.PHONY: tidy
tidy: ## Tidy go.mod/go.sum under the pinned toolchain.
	go mod tidy

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint

## Tool Versions
# The one place the linter version is pinned. .github/workflows/ci.yaml reads
# it back out of this file (see the "Resolve golangci-lint version" step), so a
# workstation and CI cannot drift apart. A golangci-lint release is built
# against one Go minor and must not lag go.mod's: feeding packages loaded by a
# newer toolchain to a linter built for an older one panics inside go/types.
# When the `go` line above moves to a new minor, bump this alongside it.
GOLANGCI_LINT_VERSION ?= v2.10.1

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary (the release binary, as its authors recommend over `go install`).
$(GOLANGCI_LINT): $(LOCALBIN)
	@test -s $(GOLANGCI_LINT) && $(GOLANGCI_LINT) --version | grep -q "version $(GOLANGCI_LINT_VERSION:v%=%) " || \
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(LOCALBIN) $(GOLANGCI_LINT_VERSION)
