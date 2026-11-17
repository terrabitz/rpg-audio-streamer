---
name: testing
description: Add or run domain, SQLite integration, and full-app browser tests; diagnose coverage or UX regressions in RPG Audio Streamer.
---

# Testing RPG Audio Streamer

Read `docs/architecture.md` for the app's boundaries and data flow. Keep that
document accurate when behavior, code structure, persistence, or interfaces
change. Architecture descriptions belong there, not in this procedural skill.
Keep root `AGENTS.md` short.

## Setup and commands

Use the Go version in `go.mod`, Node in `ui/.nvmrc`, and FFmpeg on `PATH`.
Building and running Go also requires a C compiler, pkg-config, and Discord
libdave 1.1.0. On glibc Linux or macOS, install the verified official binary
and configure the current shell:

```sh
sh scripts/install-libdave.sh "$HOME/.local/libdave"
export PKG_CONFIG_PATH="$HOME/.local/libdave/lib/pkgconfig${PKG_CONFIG_PATH:+:$PKG_CONFIG_PATH}"
export LD_LIBRARY_PATH="$HOME/.local/libdave/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
```

On macOS, set `DYLD_LIBRARY_PATH` instead of `LD_LIBRARY_PATH`. The Linux
release requires glibc 2.38 or newer; it does not support Alpine's musl.
Install frontend dependencies with `cd ui && npm ci`. Install Chromium once
with `cd ui && npx playwright install chromium` (Linux CI also uses `--with-deps`).
The UI compiler must remain compatible with `vue-tsc`; TypeScript 7's native
package does not expose the JavaScript compiler API required by the current
Vue tooling.

| Command (repository root) | Purpose |
| --- | --- |
| `make test` | Go race detection, SQLite integration, and frontend domain coverage |
| `make test-go` | All Go tests; enforce 80% internal statement coverage |
| `make test-ui` | Vitest domain coverage; enforce 80% statements, branches, functions, and lines |
| `make test-e2e` | Production UI build and real full-app Chromium journeys including live Discord |
| `make test-dev` | Seeded local dev startup, isolation, media fixtures, and cleanup |
| `cd ui && npm run build` | Vue/TypeScript checking, including tests, and production bundle |
| `cd ui && npm run lint:check` | Read-only frontend lint |

For focused iteration, use `go test ./internal/websocket -run TestMessageRouting`,
`cd ui && npm run test:unit -- run src/stores/__tests__/audio.test.ts`, or
`cd ui && npm run test:e2e -- --grep 'invited player'`. Run the affected coverage
gate after focused checks. No documentation-only validation is needed.

Mixer domain tests render known PCM buffers without sleeps; live-service tests
own the render loop's context and bound waits for decoding and cancellation.

### Live YouTube tests

Both the Go music-processing integration test and the `chromium-youtube`
browser project read `INTEGRATION_TEST_YT_DLP_LINK` into typed test
configuration. With the variable absent, these tests skip; an explicitly empty
or invalid value fails rather than falling back to a sample video. Other
YouTube browser tests keep their deterministic external-downloader fixture.

Use a public single-video URL within the application's three-hour and 200 MiB
limits. Put a current `yt-dlp[default]`, Node.js 20 or newer, and FFmpeg on
`PATH`, with the usual Go/libdave environment configured. For example:

```sh
export INTEGRATION_TEST_YT_DLP_LINK='https://www.youtube.com/watch?v=PyD9cMarVJk'
go test ./internal/musicprocessing -run '^TestYouTubeIntegration$' -count=1 -timeout=20m
cd ui && npm run test:e2e -- youtube-live.spec.ts --project=chromium-youtube
```

The Go test scans and downloads the real video, persists processing state in
temporary SQLite, converts it to HLS, checks waveform output, and decodes the
result with FFmpeg. The browser test uses the real downloader and isolated app
to scan, import, wait for readiness, reload, play actual browser media, and
delete the track through the UI. Neither test substitutes network responses or
the downloader. Availability, rate limits, and YouTube extractor changes fail
the configured tests; they are not converted into skips. Downloads and app
state are temporary. The live browser project allows seventeen minutes for
the scan and bounded processing job and is excluded from recording mode.

### Live Discord tests

Run `make test-e2e` only against an isolated, regular voice channel with
two distinct bot accounts already installed in that server. Configuration is
read in increasing precedence from the worktree `.env.test`, the
persistent `~/.config/rpg-audio-streamer/discord-test.env`, and the environment.
`DISCORD_TEST_ENV_FILE` replaces the persistent file path and must exist when
specified. Never copy secrets into a worktree. The required names are:

- `INTEGRATION_TEST_DISCORD_SERVER_ID`
- `INTEGRATION_TEST_DISCORD_VOICE_CHANNEL_ID`
- `INTEGRATION_TEST_DISCORD_BOT_STREAMER_TOKEN`
- `INTEGRATION_TEST_DISCORD_BOT_STREAMER_CLIENT_ID`
- `INTEGRATION_TEST_DISCORD_BOT_LISTENER_TOKEN`

Server/channel IDs are fixture inputs for explicit UI selection, not server
configuration. The application server receives only the streamer token.
OAuth client secrets are not used. Never
print tokens, embed them in URLs, or pass them through command arguments.
The streamer needs View Channel, Connect, and Speak; the listener needs View
Channel and Connect and joins with deafening disabled. Neither needs privileged
Gateway intents.

Do not run multiple copies against the same bots or while a local preview holds
the streamer. Playwright covers server/channel selection,
playback, mute, pause/resume, disconnect, and reconnect. Its separate Node bot
decrypts and decodes real voice audio, checks fresh 440/880 Hz tone windows,
and accepts decoded silence or packet inactivity on pause before requiring
fresh audio on resume. Live runs disable recordings, browser traces,
screenshots and the HTML report. Recording mode is unavailable for the
Discord journey; unrelated flows may be recorded individually.
Installation approval is interactive in Discord and is not automated.
Offline Go HTTP tests use a local fake Discord API for discovery, filtering,
and failure handling. Set `DISCORD_LIVE_INTEGRATION=1` and provide the
streamer token, server ID and voice channel ID as environment variables to
include the Go connection integration test in `make test-go`. Missing values
fail the test.

The `pr-checks` workflow runs the Go suite and full Playwright suite in separate
jobs on pull requests, pushes to main, nightly, and manual dispatch. Configure
the five names above as repository secrets. A shared concurrency group
serializes both live tests across jobs and runs. Fork and Dependabot pull
requests cannot access these secrets and fail the live checks rather than
running untrusted code with them.

## Coverage policy

- The Go gate measures **all `internal/...` packages**, including generated
  sqlc code exercised through the real datastore. The CLI entry points in
  `main.go` and `cmd/hashpass` are outside the coverage denominator, but all Go
  tests still run. Test-result caching is disabled so the combined profile uses
  the current run across all instrumented packages. Inspect `coverage.out` with
  `go tool cover -func=coverage.out` or `go tool cover -html=coverage.out`.
- The frontend gate includes **every store, composable, and WebSocket router**,
  including files without tests. Reports are in `ui/coverage/`. Generated SDK
  code and Vue templates are outside this domain metric.
- Browser tests establish observable UX and real integration, not a synthetic
  "80% E2E coverage" claim. They do not mock HTTP, SQLite, FFmpeg, WebSockets,
  HLS, or browser media playback.
- Favor meaningful main-path assertions and selected failures over exhaustive
  low-value permutations. Do not lower thresholds or narrow includes to hide
  gaps. A green coverage number is not a substitute for behavior assertions.

## Domain tests

Use Go's standard `testing` package and table-driven cases for authentication,
role restrictions, message routing, and recipient filtering. Keep fixtures
small; test outcomes and payloads, not implementation call counts alone.
`internal/websocket/hub_test.go` shows buffered test clients and real socket
pumps with bounded waits and shutdown.

Frontend tests live alongside their subject under `__tests__`. Create a fresh
Pinia for each test. Mock the generated SDK boundary in store unit tests rather
than editing `ui/src/client/apiClient`. Cover state transitions, partial updates,
failed requests, mute/zero values, and disabled audio synchronization. Use
fake timers and a deterministic WebSocket double for reconnect tests; restore
timers, globals, environment overrides, and spies after each test.

## SQLite integration tests

`internal/sqlitedatastore/datastore_test.go` uses real file-backed SQLite in
`t.TempDir()` and the production SQL migrations from `sql/migrations`.
Exercise datastore methods rather than mock SQL or duplicate the schema.
Verify seed policies, CRUD round trips, partial patches, missing records,
cancellation, persistence across reopening, and migration up/down/reapplication.
Use whole-second UTC timestamps because storage uses RFC3339 precision.

Never write handwritten SQL execution in Go or edit generated `sqlitedb`
files. Query changes belong in `sql/queries`; run `make gen-sqlc`. Migration
changes belong in `sql/migrations`. HTTP tests use `Server.Handler()` so the
same routing and middleware are exercised as the listener, without fixed ports.
Use real SQLite for persistence guarantees; mock stores only for HTTP failure
injection.

## Full-app browser tests

Playwright builds the Go executable and production UI once per run. Each test
attempt starts its own Go server serving the production UI on a loopback port, with a temporary
SQLite database, upload directory, and WebSocket hub. Its fixture closes browser
contexts, stops the server, and removes temporary state even when a
test fails. Except for the explicit live Discord configuration described above,
no existing server, database, or uploads are used. Do not point these tests at a
production environment. Public fixture credentials are `e2e-gm` /
`e2e-password`.

The suite uses two workers; the timing-sensitive and live Discord projects each
have a one-worker limit. Only the live Discord journey loads credentials or
connects the bots; do not run multiple suite invocations against the same bots.
All projects disable Chromium audio output so media clocks do not depend on the
host audio device; they still check real media playback. Loop-editing journeys
with continuous playback measurements belong in the timing project.
Continuous marker-drag observations stop before releasing the pointer.
Natural-loop drag checks restore a known playback region between markers and
assert real loop handoffs. Both drag flows require progress in every half-second
window; exact subsecond playback-rate invariants belong in deterministic
component tests, not browser wall-clock comparisons affected by decoder warm-up.
Recording mode uses one worker and excludes infrastructure and live Discord
tests. Each test has a fresh browser session; use a
separate browser context for players so a GM cookie cannot masquerade as player
authentication. Generate audio using FFmpeg instead of committing media fixtures. Drive setup and edits
through the UI. Assert real media `paused` and `currentTime` properties for
playback, not just CSS classes or socket receipt. Use accessible names for
controls, keyboard selection for Vuetify selects, and retrying assertions
instead of fixed sleeps. Exercise deletion through the UI when it is part of the
flow, rather than relying on it for test isolation.

Each browser test covers one user-visible flow, because every test doubles as a
pull request recording (see `.github/skills/pr-recordings/SKILL.md`). Name the
test after the flow, split a test that demonstrates several unrelated flows,
and import `test` and `expect` from `ui/e2e/fixtures.ts` rather than
`@playwright/test` so recording mode captures every page. Use `test.slow()`
instead of fixed `test.setTimeout()` values so recording mode's longer limits
still apply.

Failed non-Discord tests retain browser traces, screenshots, and bounded server
logs in `ui/test-results/`; CI uploads them as attempt-specific artifacts with
seven-day retention when the job fails. Live Discord tests disable these artifacts
and the HTML report to protect credentials. Diagnose the underlying
behavior before changing selectors, timeouts, or assertions. Do not bypass
failed app paths with mocked responses or injected Pinia/localStorage state.

## Change checklist

1. Add a regression assertion at the lowest meaningful layer.
2. Update queries/OpenAPI at their source and regenerate clients when needed.
   For WebSocket changes, keep the Go payload structs and browser payload
   interfaces aligned, and exercise the affected typed callbacks.
3. Keep `docs/architecture.md` synchronized with changed app behavior and
   boundaries; keep commands and testing procedures in this skill.
4. Run focused tests, the affected coverage gates, and relevant build/lint.
5. Run browser journeys when HTTP, auth, playback, persistence, or UX changes.
