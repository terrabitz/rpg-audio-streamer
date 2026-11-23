---
type: Pipeline
title: Pull request checks
description: Path-filtered CI jobs, live Discord validation, and the aggregate required check.
resource: .github/workflows
tags: [operations, ci, testing]
---

# Pull request checks

All pull requests run the Go tests, frontend domain tests, seeded development
tests, and full-app browser suite in `pr-checks` (see
[test layers](./test-layers.md)). `dorny/paths-filter` selects frontend
lint/build and Docker checks for their inputs. Changes to the workflow select
every check; manual runs also execute every check.

Offline Go tests retain the internal coverage gate, including SQLite and media
integration tests. Non-Discord Playwright projects run in their own job.
Neither job receives Discord secrets or acquires the live Discord concurrency
lock, so they can run alongside each other and the live checks.

## Shared environment

The local `setup-environment` composite action owns Node setup and optional
backend, frontend, media, and Playwright installation for the Go, browser,
frontend lint/build/coverage, and cache-warming jobs. Backend setup installs
Go, Go modules, and libdave; frontend setup enables npm caching and installs
the locked dependencies. Media setup installs yt-dlp and delegates FFmpeg
installation and caching to `setup-ffmpeg`. Browser jobs cache the Chromium
bundle by Playwright version, runner OS release, and architecture. Exact cache
hits skip the browser download; system dependencies are installed on every
runner. The Playwright npm packages use the existing npm download cache.
Jobs select components independently;
Node is available for frontend tooling and yt-dlp's JavaScript runtime.
Shared-action changes select frontend lint and build checks.

## Default-branch caches

The `warm-caches` workflow uses `setup-environment` to install backend, frontend,
media, and Playwright dependencies on pushes to `main`, and supports manual dispatch.
It uses the same runner, dependency manifests, setup actions, and cache keys as
the PR jobs, so new pull requests can restore the default branch's Go module,
npm download, compiled FFmpeg, and Chromium caches. It runs no tests and requires no Discord secrets.
Chromium and FFmpeg caches are saved at the end of a successful job. Caches are
immutable per key; dependency or FFmpeg build-input changes produce new entries.

## Docker checks

Pull requests that touch any image input or the
[getting started examples](./getting-started-examples.md) and the documents
embedding them run the same per-architecture builds as the
[release pipeline](./release-pipeline.md). They push the images and a
multi-arch manifest to `ghcr.io/terrabitz/rpg-audio-streamer/dev`, tagged with
the pull request head commit SHA, rather than the release repository. A separate
`examples` job depends on the build, pulls that image on amd64 and arm64
runners, and runs the getting started examples against it. The checks also
verify that the Dockerfile's Go and Node base images match
`go.mod` and `ui/.nvmrc`, since dependency bumps do not update the Dockerfile.

Fork and Dependabot pull requests receive a read-only `GITHUB_TOKEN`, so their
development image push fails.

The hourly `dev-image-cleanup` workflow uses `dataaxiom/ghcr-cleanup-action`
to delete every tagged and untagged development image older than 24 hours,
along with incomplete multi-arch images. Manual dispatch supports a dry run.

## FFmpeg in tests

The Go and browser jobs compile the headless FFmpeg tools using the same pinned
source archive and build script as the [runtime image](./release-pipeline.md).
The local `setup-ffmpeg` action caches the installation, including ffprobe and
shared libraries, by operating system release, architecture, and the build script
and action contents. Exact cache hits skip compilation; runtime dependencies
are still installed. Both suites put this installation first on `PATH`.

CI builds against Ubuntu's libraries while the image builds against Debian's.
Sharing the source and configuration aligns media capabilities, but does not
replace testing the packaged image for ABI and runtime dependency compatibility.

## Live Discord validation

The Go live voice connection test and the `chromium-discord` Playwright project
run in dedicated jobs for pull requests and manual dispatch. GitHub withholds
repository secrets from fork and Dependabot pull requests; the live tests fail
their configuration checks when required secrets are unavailable.
Only these two jobs share the
`discord-live` concurrency queue, serializing bot access across jobs and runs.
The browser Discord job does not upload failure diagnostics.

## Required check

The final `PR Checks` job depends on path detection and all check jobs, accepts
successful or skipped jobs, and fails on failed or cancelled jobs. It provides a
single required-check target, including for documentation-only pull requests.
