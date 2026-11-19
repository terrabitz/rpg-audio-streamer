---
name: local-dev
description: Start a fresh seeded RPG Audio Streamer preview in the repository devcontainer with known GM credentials, automatic ports, and disposable data; stop and clean up afterward.
---

# Seeded local development

Use the repository devcontainer for all interactive development previews in
this skill. Do not install dependencies or run the launcher on the host. If
the devcontainer cannot be started, report the blocker instead of falling back
to a host setup. This workflow is not for production or the empty-library
browser test suite.

## Start

1. Reopen the current worktree in the editor's devcontainer, or run
   `devcontainer up --workspace-folder .` from the worktree root if using
   the Dev Container CLI. Wait for the post-create dependency setup. Rebuild
   the container if its image or configuration changed.
2. Run `make dev` from the worktree root **inside** the devcontainer. From
   a host shell with the Dev Container CLI, use
   `devcontainer exec --workspace-folder . make dev` instead of running
   `make dev` on the host.
   For Discord streaming, run
   `DEV_DISCORD=1 make dev` inside the container. The bot token can come from
   the container process environment, the worktree `.env`, or
   `~/.config/rpg-audio-streamer/.env` in the container (in that precedence
   order); ordinary `make dev` ignores Discord
   credentials. The user-level file may contain `DISCORD_STREAMING_BOT_TOKEN` and should
   be private. Only a bot token is needed. Use Connect to Discord at a table
   to install the bot if needed and choose a server/voice channel. Keep test
   previews in an isolated channel and disconnect before running live tests.
3. Keep the launcher alive for the entire preview. For an agent-managed preview,
   use a detached long-lived shell only when the execution environment permits
   it; retain its exact shell ID so it can be stopped later.
4. Wait for `DEV_INSTANCE_READY` before opening the browser. The line contains
   JSON with `url`, `apiURL`, `username`, `password`, `invite`, and `scratch`.
   A running process or the storage-path message alone does not mean ready.
   Verify the printed UI URL responds before handing it to the user.
5. Forward the printed UI port using the editor's Ports panel if it is not
   forwarded automatically, then open the forwarded UI URL. Use that origin
   for the printed player path too if the forwarded port differs.
   GM credentials are **gm / P@ssw0rd**
   (public fixtures, never production credentials). Use the printed player URL
   in a separate browser context to avoid reusing the GM cookie.

Each invocation chooses free loopback ports; do not assume port 5173 or 8080.
Only forward the UI port for the seeded preview; the API stays behind Vite's proxy.
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

## Verify launcher and devcontainer changes

When changing `docker/Dockerfile`, check the devcontainer's Go and Node
versions, libdave and FFmpeg installers, yt-dlp setup, and system libraries
for matching changes. Keep development-only tools out of the production
image. After changing the devcontainer Dockerfile or configuration, run
`make devcontainer-build` from the worktree root with the Dev Containers CLI
on `PATH` and Docker's default network. This builds the image without
restarting an existing container. Reopen or recreate the devcontainer to
verify setup.
Run `make test-dev` inside the devcontainer for real startup, API seeding,
HLS fixtures, isolation, and shutdown checks. Use the testing skill for app
behavior and browser journeys.
Keep `scripts/dev.mjs` and its tests current when app changes affect upload
contracts, table associations, track types, or the scenarios a preview needs.
