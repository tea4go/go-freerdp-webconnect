#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FREERDP_PREFIX="${FREERDP_PREFIX:-/usr/local/opt/freerdp}"

if [[ -d "/opt/homebrew/opt/freerdp" && ! -d "${FREERDP_PREFIX}" ]]; then
    FREERDP_PREFIX="/opt/homebrew/opt/freerdp"
fi

check_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "ERROR: missing command '$1'"
        exit 1
    fi
}

check_cmd go
check_cmd node
check_cmd npm
check_cmd wails

if [[ ! -f "${FREERDP_PREFIX}/lib/libfreerdp3.dylib" ]]; then
    echo "ERROR: FreeRDP not found at ${FREERDP_PREFIX}"
    echo "Try: brew install freerdp"
    exit 1
fi

export DYLD_LIBRARY_PATH="${FREERDP_PREFIX}/lib:${DYLD_LIBRARY_PATH:-}"

cd "${PROJECT_ROOT}"
echo "Building Wails package (macOS)..."
wails build -clean "$@"
echo "Done: ${PROJECT_ROOT}/build/bin"
