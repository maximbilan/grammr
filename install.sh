#!/bin/bash
# Installation script for grammr via Homebrew

set -e

echo "Installing grammr via Homebrew..."

# Check if Homebrew is installed
if ! command -v brew &> /dev/null; then
    echo "Error: Homebrew is not installed. Please install it first:"
    echo "  /bin/bash -c \"\$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\""
    exit 1
fi

# Add tap if not already added
if ! brew tap-info maximbilan/grammr &>/dev/null; then
    echo "Adding Homebrew tap..."
    brew tap maximbilan/grammr https://github.com/maximbilan/grammr
fi

# Install grammr
brew install grammr

echo ""
echo "✅ grammr installed successfully!"
echo ""
echo "Next steps:"
echo "  1. Get an API key from https://platform.openai.com/api-keys"
echo "  2. Configure: grammr config set api_key YOUR_API_KEY"
echo "  3. Run: grammr"
