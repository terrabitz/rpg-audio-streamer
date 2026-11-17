# Project guidance

- `docs/` is an OKF wiki of atomic, linked concept files; start at `docs/index.md`. Keep affected concepts accurate whenever code or architecture changes.
- Use `.github/skills/documentation/SKILL.md` for adding, updating, or reorganizing docs.
- Use `.github/skills/testing/SKILL.md` for test commands, coverage policy, and test design.
- Use `.github/skills/pr-recordings/SKILL.md` for pull request GIFs: one GIF per flow, each generated from its own browser test.
- Use `.github/skills/local-dev/SKILL.md` for fresh seeded local previews and cleanup.
- Update the development seeding scripts and their tests when app changes affect fixture data, API contracts, or representative preview scenarios.
- Keep app structure and behavior in `docs/`; keep procedures in skills.
- Push server-side updates to the frontend through WebSocket events instead of polling. Initial loads and reconnect reconciliation may fetch authoritative state.
- Every PR with a UX change must include before-and-after screenshots or GIF recordings of the affected UI, uploaded to GitHub and embedded in the PR. Capture the baseline before editing; see `.github/skills/testing/SKILL.md` for real-app browser setup.
