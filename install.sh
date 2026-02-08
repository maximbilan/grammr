#!/bin/bash
# Installation script for grammr via Homebrew

set -e

VERSION="${1:-v1.0.0}"
FORMULA_URL="https://raw.githubusercontent.com/maximbilan/grammr/${VERSION}/Formula/grammr.rb"

echo "Installing grammr ${VERSION} via Homebrew..."

# Check if Homebrew is installed
if ! command -v brew &> /dev/null; then
    echo "Error: Homebrew is not installed. Please install it first:"
    echo "  /bin/bash -c \"\$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\""
    exit 1
fi

# Install using the formula URL
brew install --build-from-source "${FORMULA_URL}"

echo ""
echo "✅ grammr installed successfully!"
echo ""
echo "Next steps:"
echo "  1. Get an API key from https://platform.openai.com/api-keys"
echo "  2. Configure: grammr config set api_key YOUR_API_KEY"
echo "  3. Run: grammr"
