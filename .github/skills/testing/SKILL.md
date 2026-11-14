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
| `make test-discord` | PCM conversion, real Opus round trip, and isolated credential loading |
| `make test-e2e-discord` | Opt-in Go audio/lifecycle suite, then browser setup and reconnect journeys |
| `make test-e2e` | Production UI build and real full-app Chromium journeys |
| `make test-dev` | Seeded local dev startup, isolation, media fixtures, and cleanup |
| `cd ui && npm run build` | Vue/TypeScript checking, including tests, and production bundle |
| `cd ui && npm run lint:check` | Read-only frontend lint |

For focused iteration, use `go test ./internal/websocket -run TestMessageRouting`,
`cd ui && npm run test:unit -- run src/stores/__tests__/audio.test.ts`, or
`cd ui && npm run test:e2e -- --grep 'invited player'`. Run the affected coverage
gate after focused checks. No documentation-only validation is needed.

The opt-in server mixer journeys use
`cd ui && E2E_EXPERIMENTAL_MIXER=1 npm run test:e2e -- --grep 'experimental server mix|browser playback without a server mix'`.
The ordinary browser suite keeps mixing disabled; CI runs these journeys separately
with the flag enabled. Assert rendered PCM telemetry as well as GM control state.
Mixer domain tests render known PCM buffers without sleeps; live-service tests
own the render loop's context and bound waits for decoding and cancellation.

### Live Discord tests

Install the worker dependencies with `npm ci --prefix discord`. Run
`make test-e2e-discord` only against an isolated, regular voice channel with
two distinct bot accounts. This opt-in run reads the worktree `.env` or
environment through an explicit allowlist:

- `INTEGRATION_TEST_DISCORD_SERVER_ID`
- `INTEGRATION_TEST_DISCORD_VOICE_CHANNEL_ID`
- `INTEGRATION_TEST_DISCORD_BOT_STREAMER_TOKEN`
- `INTEGRATION_TEST_DISCORD_BOT_STREAMER_CLIENT_ID`
- `INTEGRATION_TEST_DISCORD_BOT_LISTENER_TOKEN`

For browser journeys, runtime `DISCORD_BOT_TOKEN` can override the streamer's
test token. Server/channel IDs are fixture inputs for explicit UI selection,
not server configuration. The Go suite reads only the list above and passes
only the bot token to the server it starts. OAuth client secrets are not used. Never
print tokens, embed them in URLs, or pass them through command arguments.
The streamer needs View Channel, Connect, and Speak; the listener needs View
Channel and Connect and joins with deafening disabled. Neither needs privileged
Gateway intents.

`TestDiscordStreamsServerMix` in `e2e/` (`E2E_DISCORD=1 go test ./e2e/ -run
TestDiscord -v`) starts the real server, drives the GM API and mixer socket,
and runs a Go listener bot in-process. The listener joins after the streamer
connects, subscribes only to the configured streamer, decrypts and decodes
received Opus with FFmpeg, and measures generated 440/880 Hz tones. Assertions
use fresh observation windows and tolerance for lossy coding, not byte equality
or a speaking indicator. Pause assertions accept decoded silence or packet
inactivity and then require fresh audio on resume. It also closes and reopens
the GM socket, verifies automatic reconnection with fresh audio, and checks
the bot leaves when the last GM socket closes. Both connections are cleaned
up on success and failure; server logs print only on failure. Do not run
multiple copies against the same bots or while a local preview holds the
streamer. The `e2e` package's offline signal, decoding, monitor, configuration,
and DAVE tests run in ordinary `go test ./...` and need FFmpeg with libopus.
Playwright covers the real installation link, server/channel selection, and
disconnect, plus a separate return-to-table reconnect journey. Each uses its
own table. Installation approval itself is interactive in Discord and is not
automated; live bots must already be installed. Offline Go HTTP tests use a
local fake Discord API for discovery, filtering, and failure handling.
The manual `pr-checks` workflow exposes `discord_e2e`; its secret names match
the list above. Live Discord tests never run automatically on pull requests.

## Coverage policy

- The Go gate measures **all `internal/...` packages**, including generated
  sqlc code exercised through the real datastore. The CLI entry points in
  `main.go` and `cmd/hashpass` are outside the coverage denominator, but all Go
  tests still run. Test-result caching is disabled so the combined profile uses
  the current run across all instrumented packages. Inspect `coverage.out` with
  `go tool cover -func=coverage.out` or `go tool cover -html=coverage.out`.
- The frontend gate includes **every store and composable**, including files
  without tests. Reports are in `ui/coverage/`. Generated SDK code and Vue
  templates are outside this domain metric.
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

`ui/playwright.config.ts` owns both processes and refuses to reuse running
servers. Ports **15173** (production Vite preview) and **18080** (Go API) must
be free. `API_PROXY_TARGET` directs the preview proxy to the isolated API;
ordinary development still defaults to port 8080.

`ui/e2e/server.mjs` builds the real Go executable into a unique temporary
directory, starts it with public test-only credentials, and removes its SQLite
database, uploads, and executable after the child exits. Except for the explicit
live Discord configuration described above, no developer `.env`,
database, uploads, or existing server is used. Do not point these tests at a
production environment. Public fixture credentials are `e2e-gm` /
`e2e-password`, with invite `e2e-invite`.

The browser suite uses one worker because the running app has one shared
library. Each test has a fresh browser session; use a separate browser context
for players so a GM cookie cannot masquerade as player authentication. Generate
audio using FFmpeg instead of committing media fixtures. Drive setup and edits
through the UI. Assert real media `paused` and `currentTime` properties for
playback, not just CSS classes or socket receipt. Use accessible names for
controls, keyboard selection for Vuetify selects, and retrying assertions
instead of fixed sleeps. Ensure added fixtures are removed through the UI.

Each browser test covers one user-visible flow, because every test doubles as a
pull request recording (see `.github/skills/pr-recordings/SKILL.md`). Name the
test after the flow, split a test that demonstrates several unrelated flows,
and import `test` and `expect` from `ui/e2e/fixtures.ts` rather than
`@playwright/test` so recording mode captures every page. Use `test.slow()`
instead of fixed `test.setTimeout()` values so recording mode's longer limits
still apply.

Failures preserve screenshots and traces in `ui/test-results/` and an HTML
report in `ui/playwright-report/`; CI uploads both. Inspect traces with
`cd ui && npx playwright show-trace <trace.zip>`. Diagnose the underlying
behavior before changing selectors, timeouts, or assertions. Do not bypass
failed app paths with mocked responses or injected Pinia/localStorage state.

## Change checklist

1. Add a regression assertion at the lowest meaningful layer.
2. Update queries/OpenAPI at their source and regenerate clients when needed.
3. Keep `docs/architecture.md` synchronized with changed app behavior and
   boundaries; keep commands and testing procedures in this skill.
4. Run focused tests, the affected coverage gates, and relevant build/lint.
5. Run browser journeys when HTTP, auth, playback, persistence, or UX changes.
