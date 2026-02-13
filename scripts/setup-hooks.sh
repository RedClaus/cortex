#!/bin/bash
# Install git hooks for the Cortex monorepo
# Run this once after cloning: ./scripts/setup-hooks.sh

HOOKS_DIR="$(git rev-parse --git-dir)/hooks"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "Installing Cortex git hooks..."

# Symlink commit-msg hook
ln -sf "$SCRIPT_DIR/hooks/commit-msg" "$HOOKS_DIR/commit-msg"
echo "  ✓ commit-msg hook installed"

# Symlink pre-push hook
ln -sf "$SCRIPT_DIR/hooks/pre-push" "$HOOKS_DIR/pre-push"
echo "  ✓ pre-push hook installed"

echo ""
echo "Done! Hooks installed to $HOOKS_DIR"
