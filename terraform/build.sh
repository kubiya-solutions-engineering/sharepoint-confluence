#!/bin/bash

# Build script for the Confluence import tool
set -e

echo "Building Confluence import tool..."

# Check if binary exists and compare timestamps
if [ -f "import_confluence" ] && [ "import_confluence" -nt "import_confluence.go" ]; then
  echo "✅ Binary is already up to date!"
else
  echo "🔨 Building new binary..."
  # Build the Go binary for Linux (Terraform Cloud runs on Linux)
  GOOS=linux GOARCH=amd64 go build -o import_confluence import_confluence.go
  
  # Make it executable
  chmod +x import_confluence
  
  echo "✅ Build complete! Binary: ./import_confluence (Linux/amd64)"
fi

echo "📝 Binary timestamp: $(stat -f "%Sm" import_confluence 2>/dev/null || stat -c "%y" import_confluence 2>/dev/null || echo "unknown")"
echo "📝 Source timestamp: $(stat -f "%Sm" import_confluence.go 2>/dev/null || stat -c "%y" import_confluence.go 2>/dev/null || echo "unknown")" 