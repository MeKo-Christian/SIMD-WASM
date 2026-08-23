# SIMD-WASM justfile -- single entry point for build, run, test and checks.

set shell := ["bash", "-euo", "pipefail", "-c"]

# Show available recipes
default:
    @just --list

#################################
# Build
#################################

# Build the SIMD kernel module (freestanding, imports Go's memory)
kernel:
    mkdir -p build
    clang --target=wasm32 -msimd128 -O3 -nostdlib -ffreestanding \
      -Wl,--no-entry -Wl,--import-memory \
      -o build/kernel.wasm kernel/kernel.c

# Build the kernel and all three app variants
build: kernel
    GOOS=wasip1 GOARCH=wasm go build -tags gosimd -o build/app-simd.wasm ./
    GOOS=wasip1 GOARCH=wasm go build -o build/app-plain.wasm ./
    GOOS=js GOARCH=wasm go build -tags gosimd -o build/app-simd-js.wasm ./
    ls -l build/

# Remove all build artifacts
clean:
    rm -rf build/

#################################
# Run
#################################

# Run the wasip1 demo with the SIMD kernels linked
run: build
    node host/run.mjs build/app-simd.wasm

# Run the wasip1 demo on the pure-Go fallback path
run-plain: build
    node host/run.mjs build/app-plain.wasm --no-kernel

# Run the GOOS=js demo under Node
run-js: build
    node host/run-js.mjs

# Serve the browser demo
serve addr="localhost:8080": build
    go run ./cmd/serve -addr {{ addr }}

#################################
# Checks
#################################

# Format everything with treefmt
fmt:
    treefmt --allow-missing-formatter

# Fail if formatting would change any tracked file
check-formatted:
    ./scripts/error-on-diff.sh just fmt

# Run the Go linter
lint:
    golangci-lint run --config ./.golangci.toml --timeout 2m

# Run the Go linter, applying the fixes it can
lint-fix:
    golangci-lint run --config ./.golangci.toml --timeout 2m --fix

# Lint the wasm-only sources, which the native run cannot see
lint-wasm:
    GOOS=wasip1 GOARCH=wasm golangci-lint run --build-tags gosimd --config ./.golangci.toml --timeout 2m

# go vet, native
vet:
    go vet ./...

# go vet under the wasm build tags, which checks the //go:wasmimport pointers
vet-wasm:
    GOOS=wasip1 GOARCH=wasm go vet -tags gosimd ./...

# Run the native tests of the fallback path
test:
    go test ./simd/

# Assert the built modules really are what they claim to be
verify: build
    #!/usr/bin/env bash
    set -euo pipefail
    out="$(mktemp -d)"
    trap 'rm -rf "$out"' EXIT

    grep -qa simd128 build/kernel.wasm \
      || { echo "kernel.wasm lacks the simd128 target feature"; exit 1; }

    node host/run.mjs build/app-simd.wasm | tee "$out/simd.out"
    grep -q "SIMD kernels linked: true" "$out/simd.out"

    node host/run.mjs build/app-plain.wasm --no-kernel | tee "$out/plain.out"
    grep -q "SIMD kernels linked: false" "$out/plain.out"

    if grep -qa gosimd build/app-plain.wasm; then
      echo "untagged build should not import the gosimd module"; exit 1
    fi

    echo "all assertions passed"

# Every static check
check: check-formatted lint lint-wasm vet vet-wasm test

# Everything CI runs
ci: check build verify run-js

# Print the versions of every tool this repo needs
tools:
    go version
    clang --version | head -1
    wasm-ld --version
    node --version
    treefmt --version
    golangci-lint --version

#################################
# Setup
#################################

# Install the formatters and linters used here
setup-deps:
    #!/usr/bin/env bash
    set -euo pipefail

    command -v treefmt >/dev/null 2>&1 || { echo "Installing treefmt..."; curl -fsSL https://github.com/numtide/treefmt/releases/download/v2.1.1/treefmt_2.1.1_linux_amd64.tar.gz | sudo tar -C /usr/local/bin -xz treefmt; }
    command -v gofumpt >/dev/null 2>&1 || { echo "Installing gofumpt..."; go install mvdan.cc/gofumpt@latest; }
    command -v gci >/dev/null 2>&1 || { echo "Installing gci..."; go install github.com/daixiang0/gci@latest; }
    command -v shfmt >/dev/null 2>&1 || { echo "Installing shfmt..."; go install mvdan.cc/sh/v3/cmd/shfmt@latest; }
    command -v prettier >/dev/null 2>&1 || { echo "Installing prettier..."; npm install -g prettier; }
    command -v golangci-lint >/dev/null 2>&1 || { echo "Installing golangci-lint..."; curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go env GOPATH)/bin" v2.6.2; }
    command -v shellcheck >/dev/null 2>&1 || echo "WARNING: shellcheck not found. Install it with your package manager."

    echo "Done. Make sure $(go env GOPATH)/bin is on your PATH."
