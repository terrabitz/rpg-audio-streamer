---
type: Subsystem
title: Playback synchronization
description: Server-authoritative live table playback in the WebSocket hub.
resource: internal/websocket
tags: [playback, websocket, synchronization]
---

# Playback synchronization

The WebSocket hub owns authoritative live playback state in memory, keyed by
table and track UUID. Its datastore boundary reads track membership, library
metadata, track-type policies, and saved [table settings](../data/table-settings.md)
through the existing store interfaces. Messages use the
[WebSocket protocol](./websocket-protocol.md).

## Table scope

Each connection has a server-validated table scope: the GM selects an existing
table by its external ID, while players supply that public external ID from the
URL (see [public table links](../backend/public-table-links.md)). All routing
filters, including targeted full syncs and sync requests, also require a
matching table.

## Snapshots and commands

Every GM and listening player requests a snapshot with `syncRequest` on
connection and reconnection. The server replies directly with `syncAll`,
including paused tracks and their positions; it does not need a connected GM.

GMs submit partial `syncTrack` commands for play/pause, seek, volume, duration,
and live [playback-region](./playback-regions.md) edits. The server validates
table membership and regions, applies commands serially, and broadcasts
canonical snapshots to both roles, including the commanding GM. Omitted fields
keep their current server values rather than being replaced by a stale browser
snapshot. Players cannot change playback, and client-originated `syncAll`
messages are not accepted.

The server enforces track-type exclusivity atomically: starting Music stops
other Music tracks, while Ambiance and One-Shot can overlap (see
[seeded track types](../data/sqlite-schema.md)).

## Clock

Snapshot positions advance from server timestamps, accounting for
repeating-region crossfade overlap and one-shot completion without browser
time-update messages. Browsers supply media duration as metadata becomes
available; the server does not decode audio. Active tables receive a snapshot
each second, keeping control-only browsers and completion state current even
when local audio cannot autoplay.

Explicit seek and play/pause commands advance a position version; only seeks
also advance a seek version. Running media uses that version to distinguish
commanded position changes from clock snapshots, preserving local loop handoffs
during live marker edits. While local media is playing, same-version snapshots
update other track fields without replacing its displayed position.
Control-only browsers continue to use snapshot positions. Media pause, playback
failure, and component disposal release local clock ownership. The Discord
[live mixer](../discord/live-mixer.md) receives playback commands but does not
send its render position back to the browser.

The seek version distinguishes [seek crossfade](./seek-crossfades.md) requests
from play/pause commands and periodic snapshots. Joining a table initializes
directly at the snapshot's position rather than replaying a previous seek
transition.

## Mixing

GM snapshots carry unscaled track volume and saved table settings. Successful
settings patches update connected GM mixes and listener playback without
echoing remote changes back into another settings save. Player snapshots apply
the table master volume once; each listener then applies their own master and
per-type multipliers. Local autoplay failures do not pause the shared table.

## Connection lifecycle

Leaving a GM table disconnects that browser without changing table playback.
Deleting a table notifies and disconnects its active clients. Server-originated
`removeTrack` messages stop removed or deleted tracks for both roles (see
[table track associations](../data/table-track-associations.md)). Socket
reconnects preserve table scope; leaving or switching tables cancels old
retries and ignores stale socket callbacks.

## Lifetime

Live playback survives every GM disconnect, table navigation, and browser
refresh for the lifetime of the server process. Reopening a table restores its
saved mix and layout together with the server's current playing/paused state
and positions. A server restart clears live playback; SQLite persists library
regions and table mixing/ordering settings, not the live playback clock.
