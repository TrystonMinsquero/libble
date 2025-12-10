#!/usr/bin/env bash

cd "$(dirname "$0")"

build_wasm() {
    GOOS="js" GOARCH="wasm" go build -o ./site/js/main.wasm ./wasm
}

build_server() {
    go build -o ./main ./server
}

# Parse arguments
if [ $# -eq 0 ]; then
    # No arguments, build both
    build_wasm
    build_server
else
    for arg in "$@"; do
        case $arg in
            wasm)
                build_wasm
                ;;
            server)
                build_server
                ;;
            *)
                echo "Unknown argument: $arg"
                echo "Usage: $0 [wasm] [server]"
                echo "  No arguments: build both"
                echo "  wasm: build only WASM"
                echo "  server: build only server"
                exit 1
                ;;
        esac
    done
fi


