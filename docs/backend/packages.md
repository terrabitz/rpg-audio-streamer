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
| `internal/server` | HTTP handlers, cookies/middleware, [public table access](./public-table-links.md), track/table contracts, datastore and hub interfaces |
| `internal/musicprocessing` | Bounded [background jobs](../data/track-processing.md), staging, HLS conversion, waveforms, restart recovery, and completion notifications |
| `internal/youtube` | [YouTube retrieval](./youtube.md), URL canonicalization, and yt-dlp metadata scans with import limits |
| `internal/middlewares` | API request logging and CORS |
| `internal/domain` | Shared track metadata, playback settings and validation, table settings, and Discord destination/status value models |
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

Authentication rules live in `auth`; command authorization and HTTP store
interfaces live in `server`; socket recipient routing lives in `websocket`.
`domain` holds track metadata and shared value models so typed event payloads do not depend on
their producer or consumer packages. Existing HTTP and processing contracts
alias those models. The datastore implements both packages' store interfaces, keeping
processing and HTTP handlers independent of SQLite details.

Processing depends on small `Downloader` and `events.Sender` interfaces, satisfied
in `main.go` by the YouTube client and the hub. Discord and HTTP notifications
use the same sender interface. The HTTP server depends on a
`VideoScanner` interface for scans. These services share no package state.

The HTTP server composes the optional mixer and Discord lifecycle wrapper.
`livemixer.Subscribe` connects it through `events.Bus`; the hub implements
subscriptions and delivers playback and lifecycle events without
depending on the mixer implementation.
Socket connections register named, typed event listeners on the same bus and
own their role-specific payload projection and recipient checks.

`sql/migrations` defines schema evolution; `sql/queries` is the authoritative
query source. `openapi.yaml` defines the HTTP client contract. Generated Go
queries and the TypeScript SDK are adapters, not handwritten business logic.
