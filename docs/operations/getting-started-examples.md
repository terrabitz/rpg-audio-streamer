---
type: Quality Model
title: Getting started examples
description: Executable installation snippets embedded into the README and installation guide.
resource: examples
tags: [operations, testing, docker]
---

# Getting started examples

## Source of truth

The Docker Compose and plain Docker methods in `INSTALLATION.md` are generated
from files in `examples/`; the `README.md` quick start reuses the plain Docker
script. Each code block names its source file on its first line, and embedme
copies that file's contents into the block. Documentation edits to these
snippets belong in the example files.

## Validation

`examples/examples.test.mjs` copies each flow into a temporary directory,
replaces the published image reference with the image named by
`EXAMPLES_IMAGE`, and runs its scripts with Bash. The test fails when
`EXAMPLES_IMAGE` is unset or a flow no longer references the published image,
so it never falls back to a pulled release. It reads the one-time `setup_code` from
the documented log command, waits for the setup status to require a claim, then
claims the instance through `POST /api/v1/setup` (see
[authentication](../backend/authentication.md)) and checks that setup is no
longer required. Teardown removes the container, Compose project, volumes,
bind-mounted data, and temporary directory.

The examples use a fixed host port 8080 and container name, so the flows run
sequentially. The [pull request checks](./pr-checks.md) pull the commit's
development image on amd64 and arm64 runners and run the flows. A separate,
lightweight check verifies that the embedded snippets match their source files
whenever the examples, the embedding documents, or the shared build tooling
change.
