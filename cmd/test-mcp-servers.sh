#!/bin/bash
# Test script for MCP server direct invocation

set -e

echo "🧪 Testing MCP Server Direct Invocation"
echo "========================================="

# Change to repo root
cd "$(dirname "$0")/.."

# Build the MCP server binaries if they don't exist
if [ ! -f "./build/bin/build-go" ]; then
    echo "Building build-go..."
    go build -o ./build/bin/build-go ./cmd/build-go
fi

if [ ! -f "./build/bin/build-container" ]; then
    echo "Building build-container..."
    go build -o ./build/bin/build-container ./cmd/build-container
fi

echo ""
echo "📦 Testing build-go MCP server..."
echo "---------------------------------"

# Test build-go with a simple JSON-RPC request
# We'll use a test program to build
TEST_REQUEST='{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "build",
    "arguments": {
      "name": "test-mcp-build",
      "src": "./cmd/lint-go",
      "dest": "./build/bin",
      "engine": "go://build-go"
    }
  }
}'

# Create a temporary directory for test output
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

# Send request to build-go MCP server
echo "$TEST_REQUEST" | ./build/bin/build-go --mcp > "$TEMP_DIR/response.json" 2> "$TEMP_DIR/stderr.log"

echo "MCP Server Response:"
cat "$TEMP_DIR/response.json" | head -20
echo ""
echo "MCP Server Stderr:"
cat "$TEMP_DIR/stderr.log" | head -10

# Check if the binary was built
if [ -f "./build/bin/test-mcp-build" ]; then
    echo "✅ build-go MCP server test PASSED: Binary was built"
    rm -f "./build/bin/test-mcp-build"
else
    echo "❌ build-go MCP server test FAILED: Binary was not built"
    exit 1
fi

echo ""
echo "🎉 MCP Server Direct Invocation Tests Complete!"
echo ""
echo "Note: build-container MCP test requires CONTAINER_ENGINE environment variable"
echo "      and a valid Containerfile to test properly."
