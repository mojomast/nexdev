#!/bin/bash

# Geoffrey Interactive Test Script
# This script helps you test Geoffrey in interactive mode

set -e

echo "╔════════════════════════════════════════════════════════════╗"
echo "║         Geoffrey AI Coding Agent - Interactive Test Mode          ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# Build Geoffrey if binary doesn't exist
if [ ! -f "./geoffrussy" ]; then
    echo "🔨 Building Geoffrey..."
    go build ./cmd/geoffrussy
    echo "✅ Build complete!"
    echo ""
fi

# Check if already initialized
if [ ! -d "$HOME/.geoffrussy" ]; then
    echo "📝 Initializing Geoffrey for the first time..."
    echo "   You will be prompted for API keys (press Enter to skip providers you don't need)"
    echo ""
    ./geoffrussy init
    echo ""
    echo "✅ Initialization complete!"
    echo ""
fi

# Interactive menu
while true; do
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "What would you like to do?"
    echo ""
    echo "1) ⚙️  Configuration (API keys, models, settings)"
    echo "2) 🎤 Start Interview (gather requirements)"
    echo "3) 🏗️  Generate Architecture Design"
    echo "4) 📋 Generate Development Plan"
    echo "5) 🔍 Review Development Plan"
    echo "6) 🚀 Execute Development"
    echo "7) 📊 Show Status"
    echo "8) 📈 Show Token Stats"
    echo "9) 💰 Check Quotas"
    echo "10) 💾 Create Checkpoint"
    echo "11) 📋 List Checkpoints"
    echo "12) 🔄 Rollback to Checkpoint"
    echo "13) ▶️  Resume from Checkpoint"
    echo "14) 🧭 Navigate Pipeline Stages"
    echo "q) Quit"
    echo ""
    read -p "Select an option: " choice

    case $choice in
        1)
            echo ""
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "⚙️  Configuration Management..."
            echo ""
            ./geoffrussy config
            ;;
        2)
            echo ""
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "🎤 Starting Interview..."
            echo ""
            ./geoffrussy interview
            ;;
        3)
            echo ""
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "🏗️  Generating Architecture..."
            echo ""
            ./geoffrussy design
            ;;
        4)
            echo ""
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "📋 Generating Development Plan..."
            echo ""
            ./geoffrussy plan
            ;;
        5)
            echo ""
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "🔍 Reviewing Development Plan..."
            echo ""
            ./geoffrussy review
            ;;
        6)
            echo ""
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "🚀 Executing Development..."
            echo ""
            ./geoffrussy develop
            ;;
        7)
            echo ""
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "📊 Showing Status..."
            echo ""
            ./geoffrussy status
            ;;
        8)
            echo ""
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "📈 Showing Token Stats..."
            echo ""
            ./geoffrussy stats
            ;;
        9)
            echo ""
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "💰 Checking Quotas..."
            echo ""
            ./geoffrussy quota
            ;;
        10)
            echo ""
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "💾 Creating Checkpoint..."
            echo ""
            read -p "Enter checkpoint name (or press Enter for auto-generated): " name
            if [ -z "$name" ]; then
                ./geoffrussy checkpoint
            else
                ./geoffrussy checkpoint --name="$name"
            fi
            ;;
        11)
            echo ""
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "📋 Listing Checkpoints..."
            echo ""
            ./geoffrussy checkpoint --list
            ;;
        12)
            echo ""
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "🔄 Rolling Back to Checkpoint..."
            echo ""
            read -p "Enter checkpoint name to rollback to: " checkpoint_name
            if [ -n "$checkpoint_name" ]; then
                ./geoffrussy rollback "$checkpoint_name"
            else
                echo "⚠️  No checkpoint name provided. Skipping rollback."
            fi
            ;;
        13)
            echo ""
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "▶️  Resuming from Checkpoint..."
            echo ""
            ./geoffrussy resume
            ;;
        14)
            echo ""
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "🧭 Navigating Pipeline Stages..."
            echo ""
            ./geoffrussy navigate
            ;;
        q|Q)
            echo ""
            echo "👋 Goodbye!"
            exit 0
            ;;
        *)
            echo ""
            echo "⚠️  Invalid option. Please try again."
            ;;
    esac

    echo ""
    read -p "Press Enter to continue..."
    echo ""
done
