---
type: Quality Model
title: Test layers
description: Domain, SQLite integration, browser, and live-service test boundaries.
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
| Live YouTube validation | An environment-configured public video scanned, retrieved, persisted, processed, and played through the production [YouTube service](../backend/youtube.md) |
| Getting started examples | The documented [quick start and installation snippets](./getting-started-examples.md) run against the Docker image and claimed with the logged setup code |

Each browser test attempt owns a separate server, datastore, upload directory,
and WebSocket state; the built UI and server executable are shared as immutable
artifacts.

Go upload integration tests generate PCM/WAV, MP3, AAC/M4A, ALAC/M4A, FLAC,
Opus/WebM, Vorbis/Ogg, and WMA tones and process them through the production
music service with real SQLite. They check ready-state persistence,
multi-segment AAC HLS, pre-generated normalized waveforms, and non-silent stereo
PCM at the expected frequency after seeking past the first segment. These
synthetic round trips exercise the FFmpeg on `PATH`; they do not establish
compatibility with every independently encoded input.

Live YouTube integration uses the production YouTube and processing services,
real SQLite, yt-dlp, and FFmpeg. Its separate browser project exercises import,
readiness, persistence, browser playback, and UI deletion. Deterministic
YouTube journeys substitute only the external downloader; the live journeys
do not. Both YouTube layers use typed environment configuration and are opt-in
when `INTEGRATION_TEST_YT_DLP_LINK` is present.

These layers cover different boundaries rather than substituting mocks for
end-to-end behavior. [Pull request checks](./pr-checks.md) run the automated
layers in CI; live YouTube checks remain opt-in.
Test execution and maintenance procedures live in the `testing` skill
(`.github/skills/testing/SKILL.md`).
