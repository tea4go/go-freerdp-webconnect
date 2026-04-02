#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FREERDP_INSTALL="${PROJECT_ROOT}/install"
DEFAULT_WEBKIT_TAG="webkit2_40"

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

detect_webkit_tag() {
    local output

    if ! command -v pkg-config >/dev/null 2>&1; then
        echo "WARN: pkg-config not found, defaulting to ${DEFAULT_WEBKIT_TAG}" >&2
        echo "${DEFAULT_WEBKIT_TAG}"
        return
    fi

    output="$(pkg-config --list-all 2>/dev/null | grep webkit || true)"
    if [[ "${output}" == *"webkit2gtk-4.1"* ]]; then
        echo "Detected webkit2gtk-4.1 via pkg-config" >&2
        echo "webkit2_41"
        return
    fi
    if [[ "${output}" == *"webkit2gtk-4.0"* ]]; then
        echo "Detected webkit2gtk-4.0 via pkg-config" >&2
        echo "webkit2_40"
        return
    fi

    echo "WARN: webkit package not detected via pkg-config, defaulting to ${DEFAULT_WEBKIT_TAG}" >&2
    echo "${DEFAULT_WEBKIT_TAG}"
}

has_wails_tags_arg() {
    local arg
    for arg in "$@"; do
        case "${arg}" in
            -tags|--tags|-tags=*|--tags=*)
                return 0
                ;;
        esac
    done
    return 1
}

resolve_wails_tags() {
    if [[ -n "${WAILS_GO_TAGS:-}" ]]; then
        echo "Using WAILS_GO_TAGS from environment: ${WAILS_GO_TAGS}" >&2
        echo "${WAILS_GO_TAGS}"
        return
    fi
    if [[ -n "${WEBKIT_TAG:-}" ]]; then
        echo "Using WEBKIT_TAG from environment: ${WEBKIT_TAG}" >&2
        echo "${WEBKIT_TAG}"
        return
    fi
    detect_webkit_tag
}

for required in libfreerdp3.so libfreerdp-client3.so libwinpr3.so; do
    if ! find "${FREERDP_INSTALL}" -type f -name "${required}*" -print -quit 2>/dev/null | grep -q .; then
        echo "ERROR: ${required} not found under ${FREERDP_INSTALL}"
        echo "Please run: ./lib_build_linux.sh"
        exit 1
    fi
done

LIB_PATHS=()
for dir in \
    "${FREERDP_INSTALL}/lib" \
    "${FREERDP_INSTALL}/lib64" \
    "${FREERDP_INSTALL}/lib/x86_64-linux-gnu" \
    "${FREERDP_INSTALL}/lib/aarch64-linux-gnu"; do
    [[ -d "${dir}" ]] && LIB_PATHS+=("${dir}")
done

if [[ ${#LIB_PATHS[@]} -eq 0 ]]; then
    echo "ERROR: FreeRDP runtime libraries not found in ${FREERDP_INSTALL}"
    echo "Please run: ./lib_build_linux.sh"
    exit 1
fi

CGO_LDFLAGS_EXTRA=()
for dir in "${LIB_PATHS[@]}"; do
    CGO_LDFLAGS_EXTRA+=("-L${dir}" "-Wl,-rpath,${dir}")
done

export LD_LIBRARY_PATH="$(IFS=:; echo "${LIB_PATHS[*]}"):${LD_LIBRARY_PATH:-}"
export CGO_LDFLAGS="${CGO_LDFLAGS_EXTRA[*]} ${CGO_LDFLAGS:-}"

cd "${PROJECT_ROOT}"
echo "Using FreeRDP lib paths: $(IFS=:; echo "${LIB_PATHS[*]}")"
echo "Starting Wails dev (Linux)..."
if has_wails_tags_arg "$@"; then
    echo "Using Wails tags from CLI args"
    exec wails dev "$@"
fi

WAILS_TAGS="$(resolve_wails_tags)"
echo "Using Wails tags: ${WAILS_TAGS}"
exec wails dev -tags "${WAILS_TAGS}" "$@"
