#!/usr/bin/env bash
set -euo pipefail

export HOME=$(mktemp -d)
export GOPATH="$HOME/go"
export GOCACHE="$HOME/go-cache"

# Download the module and get the directory path from JSON output
# This handles case-encoded paths correctly (e.g., BurntSushi -> !burnt!sushi)
modInfo=$(go mod download -json "${goPackagePath}@${version}")
modDir=$(echo "$modInfo" | jq -r '.Dir')

if [ -z "$modDir" ] || [ "$modDir" = "null" ]; then
  echo "Failed to get module directory from go mod download" >&2
  echo "$modInfo" >&2
  exit 1
fi

# Copy to output
cp -r "$modDir" "$out"

# Ensure all files are readable
chmod -R +r "$out"
