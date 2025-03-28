#!/bin/sh

# Used to run program locally.

set -e # Exit early if any commands fail

# Determine the current working directory (the directory that the script is in)
CURRENT_DIR="$(pwd)"

# Find the project root
# If we're in the project root, use the current directory
# Otherwise, we're in a `mygit` repository, so use the parent directory
if [ "$(basename "$CURRENT_DIR")" = "git-in-go" ]; then
  PROJECT_ROOT="$CURRENT_DIR"
else
  PROJECT_ROOT="$(dirname "$CURRENT_DIR")"
fi

# Build the Go program in the mygit/ directory
(
  cd "$PROJECT_ROOT/mygit"
  go build -buildvcs="false" -o /tmp/git-in-go .
)

# Run the Go program
exec /tmp/git-in-go "$@"
