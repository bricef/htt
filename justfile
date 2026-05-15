# Project task runner. Run `just` to list recipes.
#
# All recipes export GOCACHE to a path inside the repo. The Claude Code
# sandbox blocks writes to ~/Library/Caches/go-build/, so the standard
# cache path doesn't work for in-sandbox test runs. This justfile is the
# single source of truth for that.

export GOCACHE := justfile_directory() + "/.gocache"

# Default: list recipes.
default:
    @just --list

# Run every test suite (e2e, TUI, internal packages).
test:
    go test ./test/... ./internal/... -count=1

# Run just the e2e CLI harness.
test-e2e:
    go test ./test/e2e/ -count=1

# Run just the TUI snapshot harness.
test-tui:
    go test ./test/tui/ -count=1

# Run all internal/* tests (domain, storage, usecase, todo, ...).
test-unit:
    go test ./internal/... -count=1

# Run one package, e.g. `just test-pkg internal/storage`.
test-pkg pkg:
    go test ./{{pkg}}/ -count=1 -v

# Build the htt binary into ./bin/htt.
build:
    mkdir -p bin
    go build -o bin/htt ./cmd/htt

# Install the htt binary into $GOBIN / $GOPATH/bin.
install:
    go install github.com/bricef/htt/cmd/htt

# Static analysis across the module.
vet:
    go vet ./...

# Lint with golangci-lint.
check:
    golangci-lint run

# Populate the module cache. Must be run by the user outside the sandbox
# the first time after a fresh checkout. The sandbox blocks writes to
# ~/go/pkg/mod/cache/download/.
mod-download:
    go mod download

# Remove build artifacts and the in-repo Go cache.
clean:
    rm -rf bin .gocache
