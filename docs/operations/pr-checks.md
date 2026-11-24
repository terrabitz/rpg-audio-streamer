---
type: Pipeline
title: Pull request checks
description: Devcontainer CI checks, live Discord validation, and the aggregate required check.
resource: .github/workflows
tags: [operations, ci, testing]
---

# Pull request checks

The `pr-checks` workflow selects checks by changed paths (see
[test layers](./test-layers.md)). One devcontainer job runs the offline Go,
frontend, seeded development, browser, embedded examples, and README demo
checks together whenever any relevant path changes. Manual runs bypass path
filtering. Offline checks do not receive Discord secrets or acquire the live
Discord concurrency lock.

## Shared environment

Go, frontend, seeded development, browser, and documentation checks run inside
the repository devcontainer using the official `devcontainers/ci` action. Its
image supplies Go, Node, libdave, FFmpeg, yt-dlp, and Chromium with its Linux
dependencies. The Playwright version comes from `ui/package-lock.json`, and
workspace dependencies are installed when the devcontainer starts. The same
definition and Make targets are used for local tests. Each devcontainer job
calls the `.github/actions/devcontainer` composite action with its Make target;
the action owns Buildx setup, layer and dependency caching, and environment
forwarding. GitHub Actions handles job selection, artifact handling, and
publication separately. The combined target runs checks in parallel, ordering
the frontend build and browser suites to avoid concurrent writes to the UI
build and Playwright output directories.

## Default-branch caches

The `warm-caches` workflow uses `devcontainers/ci` to export builds to the
GitHub Actions BuildKit cache on pushes to `main` and on manual dispatch, with
separate scopes for amd64 and arm64. PR checks import the matching scope. The combined amd64 job and arm64 example
job export their layers for subsequent runs. The export
includes intermediate stages, so changes to the browser install do not require
recompiling FFmpeg. BuildKit invalidates layers when their inputs change,
including the Playwright lockfile and shared installer scripts.

Jobs that run commands in the devcontainer also restore a `.cache` directory
in the checkout with `actions/cache`, keyed by `go.sum` and
`ui/package-lock.json`. The composite action points `GOMODCACHE` and
`npm_config_cache` there and forwards both through the action's `env` input,
so the container's dependency installation reads downloaded Go modules and
npm packages from the cache.

## Docker checks

Pull requests that touch any image input or the
[getting started examples](./getting-started-examples.md) run the same per-architecture builds as the
[release pipeline](./release-pipeline.md). They push the images and a
multi-arch manifest to `ghcr.io/terrabitz/rpg-audio-streamer/dev`, tagged with
the pull request head commit SHA, rather than the release repository. A separate
`examples` job depends on the build, pulls that image on amd64 and arm64
runners, and runs the example harness through the same devcontainer action as
the other checks. The shared `devcontainer` filter also selects the image build
so the examples have an image to test. The devcontainer uses the host Docker
daemon; the examples launch the packaged app alongside it and reach it through
the host's published port. The checkout has the same absolute path inside the
devcontainer and on the host so the Docker daemon can resolve bind mounts from
the examples.

Fork and Dependabot pull requests receive a read-only `GITHUB_TOKEN`, so their
development image push fails.

The hourly `dev-image-cleanup` workflow uses `dataaxiom/ghcr-cleanup-action`
to delete every tagged and untagged development image older than 24 hours,
along with incomplete multi-arch images. Manual dispatch supports a dry run.

## Embedded examples

Changes to `examples/`, `README.md`, `INSTALLATION.md`, or the shared
`devcontainer` inputs select the combined job, which fails when the documents'
embedded snippets differ from their [example](./getting-started-examples.md)
source files. The check does not need the packaged app image.

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
The live jobs set only the required credentials on their composite action step
and list their names in its `env` input, which forwards them into the
devcontainer through the `devcontainers/ci` action's `env` input. The action
passes them as
`devcontainer` CLI arguments, and GitHub masks the values in logs; other test
jobs do not receive them.
The browser Discord job does not upload failure diagnostics.

## README demo

The README demo check reuses the browser recording harness to produce a looping
GIF of a GM switching music, playing a one-shot, and stopping playback while a
player listens in a separate browser context. The recording overlay uses a
fixed seed and the movement endpoints to reproduce the same curved cursor
paths. The GM cursor settles before each click and returns to the same resting
spot at the end of the loop. On
pull requests selected for the combined checks, it compares stable GM and
player screenshots with the checked-in baselines. Manual runs record the demo
without publishing it.
Pointer overlays and playback-position motion are excluded from comparison.
The `README_DEMO_DIFF_RATIO` environment variable overrides the default
tolerance of `0.002` (0.2%) differing pixels in each screenshot.
Only missing or materially changed baselines cause the Git LFS GIF and
checkpoints to be refreshed: `make test-readme-demo` rewrites the changed
checkpoints and, when it rewrites either one, copies the new recording to
`media/audio-playing.gif`. Agents run it before pushing and commit the
refreshed media. In CI, the check attaches changed media as an artifact.
A separate job pushes those assets only to the current head of a same-repository
PR branch, excluding Dependabot and the default branch. A push with the built-in
GitHub token requires approval for the subsequent PR workflow run.

## Required check

The final `PR Checks` job depends on path detection and all check jobs, accepts
successful or skipped jobs, and fails on failed or cancelled jobs. It provides a
single required-check target, including for documentation-only pull requests.
