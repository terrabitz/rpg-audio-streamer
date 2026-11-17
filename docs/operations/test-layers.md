---
type: Quality Model
title: Test layers
description: Domain, SQLite integration, browser, and live Discord test boundaries.
resource: ui/e2e
tags: [operations, testing]
---

# Test layers

| Layer | Boundary |
| --- | --- |
| Domain tests | Authentication, routing policies, frontend state transitions, and the [PCM mixer](../discord/pcm-mixer.md) and [live mixer](../discord/live-mixer.md) |
| SQLite integration tests | The real [schema](../data/sqlite-schema.md) and adapter against isolated file-backed databases |
| Browser journeys | The built Vue app with the real Go server, database, FFmpeg, WebSockets, and HLS media playback |
| Live Discord validation | Audio received by a separate listener bot in a shared external voice channel ([Discord voice streaming](../discord/voice-streaming.md)) |

Each browser test attempt owns a separate server, datastore, upload directory,
and WebSocket state; the built UI and server executable are shared as immutable
artifacts.

These layers cover different boundaries rather than substituting mocks for
end-to-end behavior. [Pull request checks](./pr-checks.md) run them in CI.
Test execution and maintenance procedures live in the `testing` skill
(`.github/skills/testing/SKILL.md`).
