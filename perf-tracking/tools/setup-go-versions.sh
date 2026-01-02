#!/bin/bash
# Download and install Go versions locally for benchmarking

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="${SCRIPT_DIR}/../.go-versions"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Map architecture names to Go's naming
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    armv7l) ARCH="armv6l" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

download_and_install() {
    local VERSION=$1
    local INSTALL_PATH="${INSTALL_DIR}/go${VERSION}"

    if [ -d "$INSTALL_PATH" ]; then
        echo "→ Go ${VERSION} already installed at ${INSTALL_PATH}"
        return 0
    fi

    echo "→ Downloading Go ${VERSION}..."
    local DOWNLOAD_URL="https://go.dev/dl/go${VERSION}.${OS}-${ARCH}.tar.gz"
    local TEMP_FILE="/tmp/go${VERSION}.tar.gz"

    if ! curl -fsSL "$DOWNLOAD_URL" -o "$TEMP_FILE"; then
        echo "✗ Failed to download Go ${VERSION}"
        echo "  URL: $DOWNLOAD_URL"
        return 1
    fi

    echo "→ Installing Go ${VERSION} to ${INSTALL_PATH}..."
    mkdir -p "$INSTALL_PATH"
    tar -C "$INSTALL_PATH" -xzf "$TEMP_FILE" --strip-components=1
    rm "$TEMP_FILE"

    # Verify installation
    if [ -x "${INSTALL_PATH}/bin/go" ]; then
        echo "✓ Go ${VERSION} installed successfully"
        "${INSTALL_PATH}/bin/go" version
    else
        echo "✗ Installation failed"
        rm -rf "$INSTALL_PATH"
        return 1
    fi
}

list_installed() {
    echo "Installed Go versions:"
    if [ ! -d "$INSTALL_DIR" ] || [ -z "$(ls -A "$INSTALL_DIR" 2>/dev/null)" ]; then
        echo "  (none)"
        return
    fi

    for dir in "$INSTALL_DIR"/go*; do
        if [ -d "$dir" ] && [ -x "$dir/bin/go" ]; then
            local version=$("$dir/bin/go" version)
            echo "  ${dir} -> ${version}"
        fi
    done
}

show_usage() {
    echo "Usage: $0 <command> [args]"
    echo
    echo "Commands:"
    echo "  install <version>   Download and install Go version (e.g., 1.24.0, 1.25.5)"
    echo "  list                List installed Go versions"
    echo "  path <version>      Print path to Go binary for version"
    echo "  cleanup             Remove all downloaded Go versions"
    echo
    echo "Examples:"
    echo "  $0 install 1.24.0"
    echo "  $0 install 1.25.5"
    echo "  $0 list"
    echo "  $0 path 1.24"
}

get_go_path() {
    local VERSION=$1
    local GO_PATH="${INSTALL_DIR}/go${VERSION}/bin/go"

    if [ -x "$GO_PATH" ]; then
        echo "$GO_PATH"
        return 0
    fi

    # Try to find partial version match
    for dir in "$INSTALL_DIR"/go${VERSION}*; do
        if [ -d "$dir" ] && [ -x "$dir/bin/go" ]; then
            echo "$dir/bin/go"
            return 0
        fi
    done

    echo "Go version ${VERSION} not found" >&2
    return 1
}

cleanup_all() {
    if [ -d "$INSTALL_DIR" ]; then
        echo "Removing all Go installations from ${INSTALL_DIR}..."
        rm -rf "$INSTALL_DIR"
        echo "✓ Cleanup complete"
    else
        echo "No installations found"
    fi
}

# Main command dispatcher
case "${1:-}" in
    install)
        if [ -z "$2" ]; then
            echo "Error: version required"
            echo "Usage: $0 install <version>"
            exit 1
        fi
        download_and_install "$2"
        ;;
    list)
        list_installed
        ;;
    path)
        if [ -z "$2" ]; then
            echo "Error: version required"
            echo "Usage: $0 path <version>"
            exit 1
        fi
        get_go_path "$2"
        ;;
    cleanup)
        cleanup_all
        ;;
    *)
        show_usage
        exit 1
        ;;
esac
