#!/bin/bash

# This script configures your local git repository to use the hooks in scripts/git-hooks/
# Run this once after cloning the repository.

set -e

HOOKS_DIR="scripts/git-hooks"

if [ ! -d "$HOOKS_DIR" ]; then
    echo "Error: $HOOKS_DIR directory not found."
    exit 1
fi

echo "Setting git core.hooksPath to $HOOKS_DIR..."
git config core.hooksPath "$HOOKS_DIR"

# Ensure hooks are executable
chmod +x "$HOOKS_DIR"/*

echo "Git hooks successfully configured!"
