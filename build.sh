#!/usr/bin/env bash

cd "$(dirname "$0")"

optimize_wasm() {
if command -v "wasm-opt" &> /dev/null; then
    echo "Optimizing wasm size with wasm-opt..."
    PREV=$(du ./site/js/main.wasm | cut -f1)
    wasm-opt -Oz ./site/js/main.wasm -o ./site/js/main.wasm --enable-bulk-memory
    echo "Compressed $PREV to $(du ./site/js/main.wasm | cut -f1)"
    return
else 
    echo "You should have wasm-opt installed for release builds. 
Install via one the following:
    sudo apt install binaryen
    brew install binaryen
    cargo install wasm-opt --locked
    npm install -g wasm-opt
If you just want to run locally, use GIN_MODE=debug before your command."
    exit 1
fi
}

build_wasm() {
    if [[ $GIN_MODE == "debug" ]]; then
        API_ORIGIN=""
        MODE="debug"
        GO_COMPILER="go" # use regular compiler because it's much faster
        cp $(go env GOROOT)/lib/wasm/wasm_exec.js ./site/js/wasm_exec.js
    else
        API_ORIGIN="https://api.libble.you"
        MODE="release"
        ARGS="-no-debug"
        GO_COMPILER="tinygo" # much smaller binary sizes
        cp $(tinygo env TINYGOROOT)/targets/wasm_exec.js ./site/js/wasm_exec.js
    fi

    echo "Building with $GO_COMPILER..."
    GOOS="js" GOARCH="wasm" $GO_COMPILER build -ldflags "-X main.APIOrigin=$API_ORIGIN -X main.mode=$MODE" $ARGS -o ./site/js/main.wasm ./wasm 

    if [[ $MODE == "release" ]]; then
        optimize_wasm # Not required, but saves page load time for bad networks
    fi
}


build_server() {
    local server_args=""
    if [[ $MODE == "debug" ]]; then
        local server_args="-race"
    fi
    go build $server_args -o ./main ./server
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


