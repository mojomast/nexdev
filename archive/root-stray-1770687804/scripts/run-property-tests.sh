#!/bin/bash

# Script to run property-based tests for Geoffrussy
# This script runs the state persistence round-trip property tests

set -e

echo "=========================================="
echo "Running Property-Based Tests"
echo "=========================================="
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed"
    echo "Please install Go from https://golang.org/dl/"
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | awk '{print $3}')
echo "Go version: $GO_VERSION"
echo ""

# Install dependencies
echo "Installing dependencies..."
go mod download
echo ""

# Run property tests
echo "Running Property 4: State Preservation Round-Trip..."
echo "Minimum iterations: 100"
echo ""

go test -v ./internal/state/... -run TestProperty4_StatePreservationRoundTrip

# Check exit code
if [ $? -eq 0 ]; then
    echo ""
    echo "=========================================="
    echo "✓ All property tests passed!"
    echo "=========================================="
else
    echo ""
    echo "=========================================="
    echo "✗ Property tests failed"
    echo "=========================================="
    exit 1
fi
