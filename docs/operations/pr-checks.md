---
type: Pipeline
title: Pull request checks
description: Path-filtered CI jobs, live Discord validation, and the aggregate required check.
resource: .github/workflows
tags: [operations, ci, testing]
---

# Pull request checks

The `pr-checks` workflow selects Go, frontend, seeded development, browser,
and packaging checks by changed paths (see [test layers](./test-layers.md)).
Changes to the workflow select every check; manual runs also execute every check.

Offline Go tests retain the internal coverage gate, including SQLite and media
integration tests. Non-Discord Playwright projects run in their own job.
Neither job receives Discord secrets or acquires the live Discord concurrency
lock, so they can run alongside each other and the live checks.

## Shared environment

Go, frontend, seeded development, browser, and documentation checks run inside
the repository devcontainer using the official `devcontainers/ci` action. Its
image supplies Go, Node, libdave, FFmpeg, yt-dlp, and Chromium with its Linux
dependencies. The Playwright version comes from `ui/package-lock.json`, and
workspace dependencies are installed when the devcontainer starts. The same
definition is used for local tests.

## Default-branch caches

The `warm-caches` workflow uses `devcontainers/ci` to export builds to the
GitHub Actions BuildKit cache on pushes to `main` and on manual dispatch, with
separate scopes for amd64 and arm64. PR checks import the matching scope. The
PR's amd64 cache-preparation job fills new layers before the test jobs fan out;
the arm64 example job also exports its layers for subsequent runs. The export
includes intermediate stages, so changes to the browser install do not require
recompiling FFmpeg. BuildKit invalidates layers when their inputs change,
including the Playwright lockfile and shared installer scripts.

## Docker checks

Pull requests that touch any image input or the
[getting started examples](./getting-started-examples.md) run the same per-architecture builds as the
[release pipeline](./release-pipeline.md). They push the images and a
multi-arch manifest to `ghcr.io/terrabitz/rpg-audio-streamer/dev`, tagged with
the pull request head commit SHA, rather than the release repository. A separate
`examples` job depends on the build, pulls that image on amd64 and arm64
runners, and runs the example harness in the devcontainer image against it.
Only this harness needs the host Docker socket and network. The checks also
verify that the Dockerfile's Go and Node base images match
`go.mod` and `ui/.nvmrc`, since dependency bumps do not update the Dockerfile.

Fork and Dependabot pull requests receive a read-only `GITHUB_TOKEN`, so their
development image push fails.

The hourly `dev-image-cleanup` workflow uses `dataaxiom/ghcr-cleanup-action`
to delete every tagged and untagged development image older than 24 hours,
along with incomplete multi-arch images. Manual dispatch supports a dry run.

## Embedded examples

Changes to `examples/`, `README.md`, `INSTALLATION.md`, the `Makefile`, shared
actions, or the workflow run the `embeds` job, which fails when the documents'
embedded snippets differ from their [example](./getting-started-examples.md)
source files. It needs no image build.

## FFmpeg in tests

The devcontainer builds the headless FFmpeg tools with the same pinned source
archive and build script as the [runtime image](./release-pipeline.md).
The example tests exercise the packaged runtime separately.

## Live Discord validation

The Go live voice connection test and the `chromium-discord` Playwright project
run in dedicated jobs for pull requests and manual dispatch. GitHub withholds
repository secrets from fork and Dependabot pull requests; the live tests fail
their configuration checks when required secrets are unavailable.
Only these two jobs share the
`discord-live` concurrency queue, serializing bot access across jobs and runs.
The official action expands its environment input into CLI arguments, so it
starts these containers without credentials. A separate `docker exec` passes
credentials by environment variable name to the test process, not to the image
build or the container's dependency setup.
The browser Discord job does not upload failure diagnostics.

## Required check

The final `PR Checks` job depends on path detection and all check jobs, accepts
successful or skipped jobs, and fails on failed or cancelled jobs. It provides a
single required-check target, including for documentation-only pull requests.
