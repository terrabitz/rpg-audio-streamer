---
name: pr-recordings
description: Record before/after GIFs of user-visible flows for RPG Audio Streamer pull requests from the Playwright browser tests.
---

# Pull request recordings

Every GIF in a pull request demonstrates exactly one flow and is generated from
exactly one browser test in `ui/e2e/app.spec.ts`. Do not hand-record screens or
combine several flows into one GIF. Follow the browser-test rules in
`.github/skills/testing/SKILL.md`.

## Conventions

- **One test, one flow, one GIF.** If a changed flow has no dedicated test,
  add one (or split an existing multi-flow test) before recording.
- **Name by test title.** The GIF file name is the slugified test title, and
  the pull request labels each GIF with the exact test title.
- **Before and after.** Record the "before" GIF from the same-titled test on
  the base branch. New flows only have an "after" GIF. Renamed tests have no
  before pair, so keep titles stable when a flow only changes appearance.
- **Watchable pacing.** Recording mode slows every Playwright action
  (`slowMo`), draws a visible cursor and click pulse, labels each page (`GM`,
  `GM (tab 2)`, `Player 1`), and holds the final frame. Do not speed this up.
- **All participants visible.** Pages from the GM context and players' separate
  contexts are composed into one time-aligned grid.

## Commands

| Command (repository root) | Purpose |
| --- | --- |
| `make record-e2e` | Record every browser test |
| `make record-e2e GREP="follow-up dialog"` | Record tests whose titles match |

Both run the real full-app suite with `E2E_RECORD=1`. GIFs are written to
`ui/recordings/` (git-ignored); set `RECORDING_OUTPUT` to choose another
directory. Failed tests produce no GIF. FFmpeg must be on `PATH`.

`ui/e2e/fixtures.ts` captures page videos and start offsets; `ui/e2e/record.mjs`
runs Playwright and renders the GIFs.

## Recording the base branch

The base branch may predate the recording harness. Create a temporary,
detached worktree of the base branch outside the repository, copy
`ui/e2e/fixtures.ts`, `ui/e2e/record.mjs`, and `ui/playwright.config.ts` into
it, change its spec to import `test` and `expect` from `./fixtures`, and run
`cd ui && node e2e/record.mjs --grep "<title>"` there. Remove the worktree
afterward and never commit these temporary changes to the base branch.

## Publishing

Upload GIFs with the `github-pr-media` skill. In the pull request body, use one
section or table row per flow: the test title, then the before and after GIFs.
Replace outdated GIFs whenever a flow's UI changes.
