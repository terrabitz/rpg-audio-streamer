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
every check; manual runs also execute every check. Pushes to main, scheduled
runs, and manual dispatch also run live tests.

## Docker checks

Pull requests that touch any image input run the same per-architecture builds
as the [release pipeline](./release-pipeline.md) without pushing or publishing
a manifest, and check that the Dockerfile's Go and Node base images match
`go.mod` and `ui/.nvmrc`, since dependency bumps do not update the Dockerfile.

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

The Go and browser suites include live
[Discord validation](../discord/voice-streaming.md) in separate jobs for trusted
same-repository pull requests. Fork and Dependabot pull requests fail those
credentialed checks without access to repository secrets. A shared concurrency
queue serializes the jobs across runs.

The separate Discord live workflow runs on pushes to `main`, nightly, and on
manual dispatch from `main`. One concurrency group serializes its Go and
Playwright tests across runs; pull request checks do not receive live secrets.

## Required check

The final `PR Checks` job depends on path detection and all check jobs, accepts
successful or skipped jobs, and fails on failed or cancelled jobs. It provides a
single required-check target, including for documentation-only pull requests.
