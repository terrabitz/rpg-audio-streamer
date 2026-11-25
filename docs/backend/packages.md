---
type: Component Map
title: Go packages
description: Responsibilities and boundaries of the Go packages.
resource: internal
tags: [backend, go]
---

# Go packages

| Package | Responsibility |
| --- | --- |
| `internal/auth` | Admin setup and password changes, datastore-backed credential checks, signed JWTs, and GM/player roles ([authentication](./authentication.md)) |
| `internal/server` | HTTP and WebSocket adapter: request parsing, cookies/middleware, viewer construction, and error-kind status mapping |
| `internal/tables` | [Table use cases](./domain-services.md): tables, [public table links](./public-table-links.md), settings, associations, playback commands, and live mix snapshots |
| `internal/library` | [Track library use cases](./domain-services.md): uploads, imports, scans, edits, deletion, waveforms, track types, and table-scoped streaming |
| `internal/voice` | [Discord voice use cases](./domain-services.md) and the live mixer's table track loader |
| `internal/hlsaudio` | [Windowed PCM decoding](../discord/pcm-source.md) of stored HLS for the live mixer |
| `internal/musicprocessing` | Bounded [background jobs](../data/track-processing.md), staging, HLS conversion, waveforms, restart recovery, and completion notifications |
| `internal/youtube` | [YouTube retrieval](./youtube.md), URL canonicalization, and yt-dlp metadata scans with import limits |
| `internal/middlewares` | API request logging and CORS |
| `internal/domain` | [Shared kernel](./domain-services.md): value models, viewers, and error kinds |
| `internal/events` | Shared event names, typed payloads, named subscription functions, subscription metadata, and sender/bus interfaces ([protocol](../playback/websocket-protocol.md)) |
| `internal/mixer` | [Standalone PCM mixer](../discord/pcm-mixer.md): deterministic stereo mixing, transport, loop overlap, and gain envelopes |
| `internal/livemixer` | [Live mixer](../discord/live-mixer.md): independent event subscriptions, serialized updates, table-scoped PCM engines, rendering lifecycle, and diagnostics |
| `internal/discord` | [Discord voice streaming](../discord/voice-streaming.md): bot discovery, table-bound voice connections, per-server reservations, Opus encoding, and DAVE transport |
| `ui/e2e` | Full-app browser journeys and Discord listener audio validation ([test layers](../operations/test-layers.md)) |
| `internal/sqlitedatastore` | Domain-to-SQLite mapping, migrations, context-aware datastore implementation ([schema](../data/sqlite-schema.md)) |
| `internal/sqlitedatastore/sqlitedb` | Generated sqlc models and query implementations |
| `internal/playback` | [Authoritative live table playback](../playback/synchronization.md): serialized commands, track clocks, validation, and snapshots |
| `internal/websocket` | Synchronous [event dispatch](../playback/websocket-protocol.md), scoped socket subscriptions, connection lifecycle, and heartbeat |

## Boundaries

Authentication rules live in `auth`; business rules and authorization live in
the [domain services](./domain-services.md); socket recipient routing lives in
`websocket`. `domain` holds the shared value models so typed event payloads do
not depend on their producer or consumer packages. Each domain service declares
its own store port; the SQLite datastore implements all of them, keeping use
cases independent of SQLite details. `server` depends on the services, never on
the datastore directly.

Processing depends on small `Downloader` and `events.Sender` interfaces, satisfied
in `main.go` by the YouTube client and the hub. The domain services and Discord
publish through the same sender interface. These services share no package state.

`main.go` composes the optional mixer and Discord lifecycle wrapper.
`livemixer.Subscribe` connects it through `events.Bus`; the hub implements
subscriptions and delivers playback and lifecycle events without
depending on the mixer implementation.
Socket connections register named, typed event listeners on the same bus and
own their role-specific payload projection and recipient checks.

`sql/migrations` defines schema evolution; `sql/queries` is the authoritative
query source. `openapi.yaml` defines the HTTP client contract. Generated Go
queries and the TypeScript SDK are adapters, not handwritten business logic.
