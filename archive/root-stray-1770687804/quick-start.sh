#!/bin/bash

# Quick Start Test Script - Run Geoffrey's core workflow
# This script initializes Geoffrey and runs a quick test of the interview

set -e

echo "╔══════════════════════════════════════════════════════════╗"
echo "║              Geoffrey Quick Start Test                           ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""

# Build Geoffrey
echo "🔨 Building Geoffrey..."
go build ./cmd/geoffrussy
echo "✅ Build complete!"
echo ""

# Check if already initialized
if [ ! -d "$HOME/.geoffrussy" ]; then
    echo "📝 Step 1: Initialize Geoffrey"
    echo "   You will be prompted for API keys"
    echo ""
    ./geoffrussy init
    echo ""

    echo "📝 Step 2: Configure API Keys (optional)"
    echo "   You can skip this if you already entered keys above"
    echo "   Press Ctrl+C to skip configuration"
    echo ""
    ./geoffrussy config || true
    echo ""
else
    echo "📝 Step 1: Configuration already exists"
    echo "   To reconfigure: ./geoffrussy config"
    echo ""
    echo "📝 Step 2: Manage API Keys and Models (optional)"
    echo "   View or update your configuration"
    echo "   Press Ctrl+C to skip"
    echo ""
    ./geoffrussy config || true
    echo ""
fi

echo "✅ Geoffrey is ready!"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Next steps:"
echo "  1. Run './geoffrussy interview' to start gathering requirements"
echo "  2. Run './geoffrussy design' to generate architecture"
echo "  3. Run './geoffrussy plan' to create development plan"
echo "  4. Run './geoffrussy review' to review plan"
echo "  5. Run './geoffrussy develop' to execute development"
echo ""
echo "Configuration tips:"
echo "  - Run './geoffrussy config' to manage API keys"
echo "  - Run './geoffrussy config --list-providers' to see available models"
echo "  - Run './test-interactive.sh' for an interactive menu!"
echo ""

# Build Geoffrey
echo "🔨 Building Geoffrey..."
go build ./cmd/geoffrussy
echo "✅ Build complete!"
echo ""

# Initialize
echo "📝 Step 1: Initialize Geoffrey"
echo "   You will be prompted for API keys"
echo ""
./geoffrussy init
echo ""

echo "✅ Geoffrey is ready!"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Next steps:"
echo "  1. Run './geoffrussy interview' to start gathering requirements"
echo "  2. Run './geoffrussy design' to generate architecture"
echo "  3. Run './geoffrussy plan' to create development plan"
echo "  4. Run './geoffrussy review' to review the plan"
echo "  5. Run './geoffrussy develop' to execute development"
echo ""
echo "Or run './test-interactive.sh' for an interactive menu!"
echo ""
