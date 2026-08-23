#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p build
./kernel/build.sh
GOOS=wasip1 GOARCH=wasm go build -tags gosimd -o build/app-simd.wasm ./
GOOS=wasip1 GOARCH=wasm go build            -o build/app-plain.wasm ./
GOOS=js     GOARCH=wasm go build -tags gosimd -o build/app-simd-js.wasm ./
ls -l build/
