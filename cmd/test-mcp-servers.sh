#!/bin/bash
# Test script for MCP server direct invocation

set -e

echo "🧪 Testing MCP Server Direct Invocation"
echo "========================================="

# Change to repo root
cd "$(dirname "$0")/.."

# Build the MCP server binaries if they don't exist
if [ ! -f "./build/bin/go-build" ]; then
    echo "Building go-build..."
    go build -o ./build/bin/go-build ./cmd/go-build
fi

if [ ! -f "./build/bin/container-build" ]; then
    echo "Building container-build..."
    go build -o ./build/bin/container-build ./cmd/container-build
fi

echo ""
echo "📦 Testing go-build MCP server..."
echo "---------------------------------"

# Test go-build with a simple JSON-RPC request
# We'll use a test program to build
TEST_REQUEST='{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "build",
    "arguments": {
      "name": "test-mcp-build",
      "src": "./cmd/go-lint",
      "dest": "./build/bin",
      "engine": "go://go-build"
    }
  }
}'

# Create a temporary directory for test output
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

# Send request to go-build MCP server
echo "$TEST_REQUEST" | ./build/bin/go-build --mcp > "$TEMP_DIR/response.json" 2> "$TEMP_DIR/stderr.log"

echo "MCP Server Response:"
cat "$TEMP_DIR/response.json" | head -20
echo ""
echo "MCP Server Stderr:"
cat "$TEMP_DIR/stderr.log" | head -10

# Check if the binary was built
if [ -f "./build/bin/test-mcp-build" ]; then
    echo "✅ go-build MCP server test PASSED: Binary was built"
    rm -f "./build/bin/test-mcp-build"
else
    echo "❌ go-build MCP server test FAILED: Binary was not built"
    exit 1
fi

echo ""
echo "🎉 MCP Server Direct Invocation Tests Complete!"
echo ""
echo "Note: container-build MCP test requires CONTAINER_ENGINE environment variable"
echo "      and a valid Containerfile to test properly."
