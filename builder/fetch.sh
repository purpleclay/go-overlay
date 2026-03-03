#!/usr/bin/env bash
set -euo pipefail

export HOME=$(mktemp -d)
export GOPATH="$HOME/go"
export GOCACHE="$HOME/go-cache"

# If a netrc file was provided, copy it into $HOME/.netrc for authentication.
# Both Go's HTTP client and git (via libcurl) read ~/.netrc for credentials
# when accessing private module hosts.
if [ -n "${netrcFile:-}" ] && [ -f "$netrcFile" ]; then
  cp "$netrcFile" "$HOME/.netrc"
fi

# Download the module and get the directory path from JSON output.
# This handles case-encoded paths correctly (e.g., BurntSushi -> !burnt!sushi).
# Temporarily disable set -e so we can capture the exit code and print the
# error JSON. Without this, set -e would exit immediately on failure and the
# error details (reported via JSON stdout) would be lost.
set +e
modInfo=$(go mod download -json "${goPackagePath}@${version}")
download_exit=$?
set -e

if [ $download_exit -ne 0 ]; then
  echo "go mod download failed (exit $download_exit):" >&2
  echo "$modInfo" >&2
  exit 1
fi

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
