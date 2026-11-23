#!/usr/bin/env bash
set -euo pipefail

: "${EXAMPLES_IMAGE:?Set EXAMPLES_IMAGE to the packaged image under test}"
workspace=$(pwd -P)
socket=/var/run/docker.sock
scratch=$(mktemp -d)
trap 'rmdir "$scratch"' EXIT

# Use the same absolute path on both sides of the Docker socket for the example bind mounts.
docker run --rm --network host \
  --user "$(id -u):$(id -g)" --group-add "$(stat -c %g "$socket")" \
  --mount "type=bind,source=$socket,target=$socket" \
  --mount "type=bind,source=$workspace,target=$workspace,readonly" \
  --mount "type=bind,source=$scratch,target=$scratch" \
  --workdir "$workspace" --env TMPDIR="$scratch" --env EXAMPLES_IMAGE \
  rpg-audio-streamer-test-devcontainer make test-examples
