#!/bin/bash
# Install all required tools for performance tracking

set -e

echo "=== Installing Performance Tracking Tools ==="
echo

# Check Go is available
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed"
    exit 1
fi

echo "Go version: $(go version)"
echo

# Tools to install (parallel arrays for Bash 3.2 compatibility on macOS)
TOOL_NAMES=(benchstat staticcheck gopls)
TOOL_PACKAGES=(
    "golang.org/x/perf/cmd/benchstat@latest"
    "honnef.co/go/tools/cmd/staticcheck@latest"
    "golang.org/x/tools/gopls@latest"
)

# Install each tool
for i in "${!TOOL_NAMES[@]}"; do
    tool="${TOOL_NAMES[$i]}"
    package="${TOOL_PACKAGES[$i]}"

    echo "→ Installing $tool..."
    if go install "$package"; then
        echo "  ✓ $tool installed"
    else
        echo "  ✗ Failed to install $tool"
        exit 1
    fi
done

echo
echo "=== Verifying Installations ==="
echo

# Verify installations
for tool in "${TOOL_NAMES[@]}"; do
    if command -v "$tool" &> /dev/null; then
        version=$("$tool" --version 2>&1 | head -1 || echo "installed")
        echo "✓ $tool: $version"
    else
        echo "✗ $tool: not found in PATH"
        echo "  Add $(go env GOPATH)/bin to your PATH"
        exit 1
    fi
done

echo
echo "=== Tool Installation Complete ==="
echo
echo "Tools installed to: $(go env GOPATH)/bin"
echo "Make sure this directory is in your PATH"
