#!/usr/bin/env bash
# Build the SIMD kernel module. Freestanding: no libc, no start function, no
# memory of its own -- it imports the host's (i.e. Go's) memory instead.
set -euo pipefail
cd "$(dirname "$0")"
clang --target=wasm32 -msimd128 -O3 -nostdlib -ffreestanding \
	-Wl,--no-entry -Wl,--import-memory \
	-o ../build/kernel.wasm kernel.c
