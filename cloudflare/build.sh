#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"
mkdir -p build

GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o build/d2.wasm ..

cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" build/wasm_exec.js
cp "$(go env GOROOT)/LICENSE" build/GO-LICENSE
