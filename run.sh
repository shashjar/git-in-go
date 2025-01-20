#!/bin/sh

# Used to run program locally.

set -e # Exit early if any commands fail

(
  cd "$(dirname "$0")" # Ensure compile steps are run within the repository directory
  go build -buildvcs="false" -o /tmp/git-in-go ./mygit
)

exec /tmp/git-in-go "$@"
