---
type: Subsystem
title: Domain services
description: Hexagonal use-case services for tables, the track library, and Discord voice, with ports for every adapter.
resource: internal/domain
tags: [backend, go, architecture]
---

# Domain services

The backend follows a hexagonal layout. Business rules live in use-case
services that depend only on small port interfaces; HTTP, WebSocket, SQLite,
FFmpeg, yt-dlp, and Discord are adapters wired together by
[process composition](../runtime/composition.md).

## Bounded contexts

| Package | Use cases |
| --- | --- |
| `internal/tables` | Create, rename, delete, and list tables; rotate [public table links](./public-table-links.md); [table settings](../data/table-settings.md); table track [associations](../data/table-track-associations.md); playback commands and live mix snapshots |
| `internal/library` | List, upload, [import](./youtube.md), scan, update, and delete tracks; track usage, track types, [waveforms](../data/track-uploads.md), and table-scoped HLS streaming |
| `internal/voice` | [Discord voice](../discord/voice-streaming.md) connect, disconnect, status, and setup, plus loading table tracks into the [live mixer](../discord/live-mixer.md) |

Each package has one file per use case. A use case reads top to bottom as the
rule it enforces: authorize the viewer, load what it needs through a port,
apply the rule, persist, and publish an event. Each service declares the ports
it consumes (`Store`, `Playback`, `Audio`, `Videos`, `Discord`, and
`events.Sender`), so unit tests replace every adapter with an in-memory fake.

## Shared kernel

`internal/domain` holds the value models every context shares: tables, tracks,
track types, playback settings, table settings, waveforms, and Discord
destinations. It also defines:

- `Viewer`, the authenticated caller. Adapters build a GM viewer or a player
  viewer scoped to one table; use cases decide what each viewer may do.
- Error kinds (`ErrNotFound`, `ErrInvalid`, `ErrForbidden`, `ErrConflict`,
  `ErrUnsupported`, `ErrUnavailable`, `ErrUpstream`, `ErrTimeout`, and
  `ErrInternal`). A `*domain.Error` pairs a kind with a user-facing message
  and an optional internal cause. Adapters wrap their own failures in a kind,
  such as the SQLite datastore translating missing rows to `ErrNotFound`.

## HTTP adapter

`internal/server` only parses requests, converts the auth token to a
`Viewer`, calls one use case, and writes the result. It maps each error kind to
an HTTP status and returns the error's message; errors without a kind become
500 responses with a generic message, and every 5xx response is logged.
