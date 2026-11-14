---
name: local-dev
description: Start a fresh seeded local RPG Audio Streamer instance with known GM credentials, automatic ports, and disposable data; stop and clean up the instance afterward.
---

# Seeded local development

Use this workflow for interactive development and browser previews, not
production or the empty-library browser test suite.

## Start

1. Use the Go version in `go.mod`, Node in `ui/.nvmrc` (24.7+ for native Argon2),
   and FFmpeg on `PATH`.
   Run `make dev` from the worktree root. If Vite is missing, install the
   locked UI dependencies with `cd ui && npm ci`, then retry.
   For Discord streaming, install `npm ci --prefix discord` and run
   `DEV_DISCORD=1 make dev`. This explicitly reads Discord configuration from
   the worktree `.env` or environment; ordinary `make dev` ignores Discord
   credentials. Only a bot token is needed. Use Connect to Discord at a table
   to install the bot if needed and choose a server/voice channel. Keep test
   previews in an isolated channel and disconnect before running live tests.
2. Keep the launcher alive for the entire preview. For an agent-managed preview,
   use a detached long-lived shell only when the execution environment permits
   it; retain its exact shell ID so it can be stopped later.
3. Wait for `DEV_INSTANCE_READY` before opening the browser. The line contains
   JSON with `url`, `apiURL`, `username`, `password`, `invite`, and `scratch`.
   A running process or the storage-path message alone does not mean ready.
   Verify the printed UI URL responds before handing it to the user.
4. Open the printed UI URL. GM credentials are **gm / P@ssw0rd**
   (public fixtures, never production credentials). Use the printed player URL
   in a separate browser context to avoid reusing the GM cookie.

Each invocation chooses free loopback ports; do not assume port 5173 or 8080.
The nine sample tracks include three each of Ambiance, Music, and One-Shot,
generated locally by FFmpeg and associated with the default table. They use
ordinary display names without development prefixes. They are intentionally
simple audio fixtures, not assets downloaded from another service. Editing
code uses the normal Vite HMR flow;
restart the launcher after Go changes to rebuild the API and reseed.

## Stop and reset

Press Ctrl-C in the launching terminal, or stop the exact agent-managed shell.
SIGINT and SIGTERM stop the API and Vite and remove that instance's temporary
database, uploads, generated audio, binary, and Vite cache. Startup failures
also run cleanup and exit nonzero. Restart with `make dev` for fresh data.
Do not delete repository databases or uploads to reset a dev instance.

SIGKILL, a machine crash, or loss of the launcher cannot run cleanup. If that
happens, use the exact printed `scratch` path to inspect the abandoned instance
and remove only that directory after verifying its processes have stopped.
Do not use wildcard cleanup or kill processes by name.

Separate instances have independent data, ports, invite tokens, and signing
keys. Cookies and local storage are scoped to browser origins differently:
cookies ignore ports, so use separate browser profiles/contexts when working
with multiple instances concurrently.

## Verify launcher changes

Run `make test-dev` for real startup, API seeding, HLS fixtures, isolation, and
shutdown checks. Use the testing skill for app behavior and browser journeys.
Keep `scripts/dev.mjs` and its tests current when app changes affect upload
contracts, table associations, track types, or the scenarios a preview needs.
