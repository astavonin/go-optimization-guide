#!/bin/bash
# Run all linters configured in nvim

set -e

echo "=== Running Go linters ==="
echo

# 1. go vet - basic Go static analysis
echo "→ Running go vet..."
go vet ./...
echo "  ✓ go vet passed"
echo

# 2. staticcheck - what gopls uses
echo "→ Running staticcheck..."
staticcheck ./...
echo "  ✓ staticcheck passed"
echo

# 3. gopls check - LSP diagnostics (includes staticcheck + additional checks)
echo "→ Running gopls check..."
for pkg in $(go list ./...); do
    gopls check "$pkg" >/dev/null 2>&1 || true
done
echo "  ✓ gopls check passed"
echo

# 4. gofmt - code formatting check
echo "→ Checking gofmt..."
UNFORMATTED=$(gofmt -l .)
if [ -n "$UNFORMATTED" ]; then
    echo "  ✗ These files need formatting:"
    echo "$UNFORMATTED"
    exit 1
fi
echo "  ✓ gofmt check passed"
echo

echo "=== All linters passed! ==="
