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
| `make test-e2e` | Production UI build and real full-app Chromium journeys |
| `make test-dev` | Seeded local dev startup, isolation, media fixtures, and cleanup |
| `cd ui && npm run build` | Vue/TypeScript checking, including tests, and production bundle |
| `cd ui && npm run lint:check` | Read-only frontend lint |

For focused iteration, use `go test ./internal/websocket -run TestMessageRouting`,
`cd ui && npm run test:unit -- run src/stores/__tests__/audio.test.ts`, or
`cd ui && npm run test:e2e -- --grep 'invited player'`. Run the affected coverage
gate after focused checks. No documentation-only validation is needed.

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
database, uploads, and executable after the child exits. No developer `.env`,
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
