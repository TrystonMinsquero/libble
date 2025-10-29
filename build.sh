#!/usr/bin/env bash

cd "$(dirname "$0")"
GOOS="js" GOARCH="wasm" go build -o ./views/js/main.wasm ./wasm
go build -o ./main ./server


