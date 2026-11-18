---
type: Pipeline
title: Release pipeline
description: Semantic versioning, GitHub releases, and multi-arch image publication.
resource: .github/workflows
tags: [operations, ci, release, docker]
---

# Release pipeline

Pushes to `main` that match the release workflow's path filters run
semantic-release in dry-run mode to calculate a version and release notes from
conventional commits and existing tags. Only the commit analyzer and notes
generator are enabled; the workflow does not commit a changelog or push to
`main`. `gh release create` creates the version tag at the triggering commit
and publishes the generated notes on GitHub, respecting branch protection.
Release notes live in GitHub releases.

Release runs are serialized to avoid competing version calculations. When no
release is needed, publication and image building are skipped.

## Image publication

After successful publication, the shared `docker-build` workflow builds the
Linux amd64 and arm64 images natively on matching GitHub-hosted runners, pushes
each by digest to GHCR, and combines them into a multi-arch manifest tagged
`latest`, full-version, major.minor, and major. Each architecture keeps its own
Buildx GitHub Actions cache scope. The image's runtime behavior is described in
[HTTP serving](../runtime/http-serving.md).

Pull requests that touch image inputs run the same builds without publishing
(see [pull request checks](./pr-checks.md)).

## Runtime image

The Debian slim runtime includes FFmpeg, Node.js, Python, and the
[YouTube downloader](../backend/youtube.md). A separate installer stage supplies
the downloader's virtual environment without pip or precompiled Python bytecode;
the runtime disables bytecode writes. Package indexes and build toolchains are
not included.

The Go executable and [libdave](../discord/voice-streaming.md) shared library are
stripped of debugging symbols, while libdave's licenses remain in the image.
Application files receive their ownership when copied into the runtime rather
than being duplicated into a later ownership-change layer.
