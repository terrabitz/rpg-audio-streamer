---
name: pr-recordings
description: Record before/after GIFs of user-visible flows for RPG Audio Streamer pull requests from the Playwright browser tests.
---

# Pull request recordings

Every GIF in a pull request demonstrates exactly one flow and is generated from
exactly one browser test in `ui/e2e/*.spec.ts`. Do not hand-record screens or
combine several flows into one GIF. Follow the browser-test rules in
`.github/skills/testing/SKILL.md`.

## Conventions

- **One test, one flow, one GIF.** If a changed flow has no dedicated test,
  add one (or split an existing multi-flow test) before recording.
- **Name by test title.** The GIF file name is the slugified test title, and
  the pull request uses the exact test title as the flow's H3 heading.
- **Show only the flow.** Use the `recordFlow` fixture for every published
  recording. Mark one contiguous window after setup and before teardown or
  follow-up checks. Keep all test steps and assertions running. Start just
  before the first relevant interaction and stop once its result is visible.
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

`ui/e2e/fixtures.ts` captures page videos, start offsets, and the optional flow
window. `ui/e2e/record.mjs` trims the synchronized recordings to that window
before holding the last visible frame. Unmarked tests keep full-test recordings
for local use. The marks do nothing in normal test runs.

To reproduce the root README demo check, run from the repository root in the
devcontainer:

```sh
make test-readme-demo
```

The check uses the LFS-backed `media/readme-demo/` checkpoints. Hydrate them
before local comparison. If neither checkpoint changes, leave
`media/audio-playing.gif` alone. Set `README_DEMO_DIFF_RATIO` to override the
allowed fraction of differing pixels (`0` to `1`). The
[PR checks concept](../../../docs/operations/pr-checks.md) describes how CI
publishes updates.

```ts
test('GM performs the flow', async ({ page, recordFlow }) => {
  await login(page)
  recordFlow.start()
  await page.getByRole('button', { name: 'Add Track', exact: true }).click()
  await expect(page.getByRole('dialog')).toBeVisible()
  recordFlow.stop()
  // Keep any additional checks or cleanup outside the GIF.
})
```

## Recording the base branch

The base branch may predate the recording harness. Create a temporary,
detached worktree of the base branch outside the repository, copy
`ui/e2e/fixtures.ts`, `ui/e2e/record.mjs`, and `ui/playwright.config.ts` into
it, change its spec to import `test` and `expect` from `./fixtures`, mark the
equivalent flow window with `recordFlow.start()` and `recordFlow.stop()`, and run
`cd ui && node e2e/record.mjs --grep "<title>"` there. Remove the worktree
afterward and never commit these temporary changes to the base branch.

## Publishing

Upload GIFs with the `github-pr-media` skill. Under `## Recordings`, give each
flow its own H3 using the exact test title. Put paired GIFs in a two-column
`Before` / `After` table. Place a single GIF directly under its heading without
a table or empty placeholder. Check that the clips open on the ready screen and
end on the visible result. Replace outdated GIFs whenever a flow's UI changes.

```md
## Recordings

### Existing flow test title

| Before | After |
| --- | --- |
| ![Existing flow before](BEFORE_URL) | ![Existing flow after](AFTER_URL) |

### New flow test title

![New flow](AFTER_URL)
```
