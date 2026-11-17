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
| `internal/auth` | Argon2id credential checks, signed JWTs, GM/player roles ([authentication](./authentication.md)) |
| `internal/server` | HTTP handlers, cookies/middleware, [public table access](./public-table-links.md), track/table contracts, datastore and hub interfaces |
| `internal/musicprocessing` | Track/playback models, bounded [background jobs](../data/track-processing.md), staging, HLS conversion, waveforms, restart recovery, and completion notifications |
| `internal/youtube` | [YouTube retrieval](./youtube.md), URL canonicalization, and yt-dlp metadata scans with import limits |
| `internal/middlewares` | API request logging and CORS |
| `internal/events` | Shared Go event names, envelope, table-scoped payload wrapper, and nonblocking sender interface ([protocol](../playback/websocket-protocol.md)) |
| `internal/mixer` | [Standalone PCM mixer](../discord/pcm-mixer.md): deterministic stereo mixing, transport, loop overlap, and gain envelopes |
| `internal/livemixer` | [Live mixer](../discord/live-mixer.md): serialized live commands, table-scoped PCM engines, rendering lifecycle, and diagnostics |
| `internal/discord` | [Discord voice streaming](../discord/voice-streaming.md): bot discovery, table-bound voice connections, per-server reservations, Opus encoding, and DAVE transport |
| `ui/e2e` | Full-app browser journeys and Discord listener audio validation ([test layers](../operations/test-layers.md)) |
| `internal/sqlitedatastore` | Domain-to-SQLite mapping, migrations, context-aware datastore implementation ([schema](../data/sqlite-schema.md)) |
| `internal/sqlitedatastore/sqlitedb` | Generated sqlc models and query implementations |
| `internal/websocket` | [Authoritative live table playback](../playback/synchronization.md), bounded [event dispatch](../playback/websocket-protocol.md), connected clients, recipient filters, read/write pumps |

## Boundaries

Authentication rules live in `auth`; synchronization authorization and routing
live in `websocket`; table models and HTTP store interfaces live in `server`.
Track and playback models live in `musicprocessing`, with aliases in the HTTP
contracts. The datastore implements both packages' store interfaces, keeping
processing and HTTP handlers independent of SQLite details.

Processing depends on small `Downloader` and `events.Sender` interfaces, satisfied
in `main.go` by the YouTube client and the hub. Discord and HTTP notifications
use the same sender interface. The HTTP server depends on a
`VideoScanner` interface for scans. These services share no package state.

`sql/migrations` defines schema evolution; `sql/queries` is the authoritative
query source. `openapi.yaml` defines the HTTP client contract. Generated Go
queries and the TypeScript SDK are adapters, not handwritten business logic.
