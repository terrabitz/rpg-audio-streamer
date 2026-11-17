---
type: Tool
title: Seeded local development launcher
description: Disposable, seeded API and Vite instances started by make dev.
resource: scripts/dev.mjs
tags: [runtime, development, fixtures]
---

# Seeded local development launcher

The seeded local development launcher (`scripts/dev.mjs`, exposed as
`make dev`) binds both the API and Vite to loopback on automatically selected
ports. Each invocation builds the real API into a unique temporary directory
and creates a fresh SQLite database, upload directory, and Vite cache. It uses
public fixture GM credentials, freshly salted native Argon2id password hashes,
and per-instance random signing keys.

## Fixtures

Nine generated audio fixtures (three per default
[track type](../data/sqlite-schema.md)) use ordinary display names and are
uploaded into the default table through authenticated HTTP. Music fixtures use
short original note sequences; seeded ambiance noise gently swells over time so
loop crossfades are audible. Repeating fixtures are 12 seconds long; one-shot
tones keep their short durations. Uploading exercises normal validation,
[conversion, and persistence](../data/track-uploads.md) rather than writing
database fixtures directly.
The launcher waits for each accepted fixture's
[background processing](../data/track-processing.md) to finish before announcing
the preview as ready.

## Isolation and cleanup

Vite uses the isolated API proxy and ignores developer dotenv files; the API
runs from the temporary directory with explicit configuration. The launcher
owns shutdown and removes its temporary state on normal termination or startup
failure. Uncatchable termination cannot guarantee cleanup.

## Discord

Discord is enabled in a seeded preview only when explicitly requested through
`DEV_DISCORD=1`; an allowlisted loader reads its configuration from the
worktree environment or `.env`, and the API automatically enables the
[live mixer](../discord/live-mixer.md). Ordinary previews and browser tests
strip ambient Discord credentials before starting the API.

Procedures for running the launcher live in the `local-dev` skill
(`.github/skills/local-dev/SKILL.md`).
