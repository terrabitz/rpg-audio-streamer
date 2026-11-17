---
type: Subsystem
title: Live mixer
description: Server-side table mixes driven by GM events and published as PCM.
resource: internal/livemixer
tags: [discord, audio, mixing, websocket]
---

# Live mixer

[Discord streaming](./voice-streaming.md) enables an `internal/livemixer`
service inside the Go API process. Browser HLS playback and player
synchronization remain available independently; the mixer supplies audio to
Discord. Each table session wraps a [PCM mixer](./pcm-mixer.md) fed by
[PCM sources](./pcm-source.md).

## Events

`livemixer.Subscribe` registers the enabled mixer as an independent
`events.Bus` consumer. Its adapter listens for `clientConnected`,
`playbackChanged`, `mixChanged`, `mixVolumesChanged`, `trackRemoved`,
`tableDeleted`, and `tableSessionEnded`; the hub has no mixer reference.
Registration returns an unsubscribe function. The HTTP server owns the mixer
and its Discord session lifecycle wrapper for the application's lifetime.
The adapter uses the named, typed `events.OnClientConnected`,
`events.OnPlaybackChanged`, and corresponding registration functions, just
like socket event consumers; its callbacks do not decode raw JSON.

The adapter answers each authenticated GM's `clientConnected` event with a
targeted `mixerReady` event (see
[WebSocket protocol](../playback/websocket-protocol.md)). The GM
answers with `mixChanged`: a full snapshot of playing tracks and the current
type-volume map. This happens on reconnect as well as initial connection.
Accepted `trackPlaybackChanged` inputs become canonical `playbackChanged`
batches that feed partial play/pause, seek, volume, repeating, and live
[playback-region](../playback/playback-regions.md) changes into the mixer.
Persisted track-volume changes use the same batches. Seek changes use `CrossfadeSeek` (see
[seek crossfades](../playback/seek-crossfades.md)); fresh snapshots initialize a
paused engine at the requested position before starting playback, without a
seek transition.

Track volumes are unscaled; the supplied type-volume map includes the GM master
multiplier. The service applies that multiplier and caps effective gain at
unity, matching browser per-track gain clamping. The GM sends `mixVolumesChanged` for
live type-volume and master-volume changes. Play/resume commands include the
GM's selected track volume, including unsaved and zero values. The hub also
supplies the resolved track volume and playback region to the mixer on play, so
clients omitting volume still honor saved settings.

When an exclusive track starts, the hub includes pause patches for displaced
tracks before the requested update in the same batch. These use the core's playback envelopes
and are serialized with the accepted playback update, so Discord fades out the
previous Music track while simultaneous track types continue playing. Frontend
partial updates coalesce by track ID, preserving independent fields and updates
to different tracks in the same debounce window. Immediate exclusivity and
region changes flush queued updates first.

## Authorization and errors

The hub derives table identity and GM authorization from the authenticated
socket, never the event payload. `playbackStateChanged` snapshots do not
mutate the mixer; `mixChanged` replaces it, including an empty snapshot on
leaving a table. Mixer-only events are ignored when disabled and
never forwarded to players. Successful events receive `mixerApplied`; failures
receive `mixerFailed` and are logged with table and event context. The adapter
queues these responses through the hub for delivery only to the originating GM.
The GM
displays errors without stopping browser playback. A successful reconnect
snapshot clears the error. Normal player broadcasts happen before mixer
processing, so warming a decoder does not delay that broadcast.

## Concurrency and rendering

Commands are serialized while a separate lock protects PCM rendering.
Validation precedes mutation, so invalid patches and snapshots do not partially
alter a mix. FFmpeg work runs asynchronously and source reads on the render
path are non-blocking.

The service renders 960-frame blocks on a 20 ms schedule derived from elapsed
time with a cancellable loop. After a scheduling stall it renders every missed
block, up to half a subscriber's buffer, so the Discord stream keeps pace with
real time rather than falling behind receivers' jitter buffers. It renders a
table only while that table has a PCM subscriber, avoiding duplicate decoding
while Discord is disconnected.

## Subscriptions

Rendered PCM blocks are published to internal subscriptions for
[Discord voice streaming](./voice-streaming.md). Subscribing does not start
tracks or change their positions. Each subscriber has a one-second buffer and
is closed when it falls further behind rather than delaying rendering. Closing
the table session or stopping the renderer closes its subscriptions. GM browser
audio continues through ordinary HLS playback; the server mix is audible
through Discord.

## Lifecycle

The adapter handles track-removal and table-lifecycle events independently of
the hub's authoritative playback handling. Track disassociation removes the
track from its table's mix; library deletion
removes it from all mixes (see
[table track associations](../data/table-track-associations.md)). Table deletion
and loss of its last GM connection discard the session and cancel its decoder
processes; in-flight source preparation cannot recreate it. Multiple GMs share
a table's mixer with last-accepted-event semantics. Mixer state is ephemeral,
not stored in SQLite.

## Testing

Domain tests assert rendered samples after partial updates and snapshots, table
isolation, rejection without mutation, and nonblocking rendering/closure. The
full-app browser journey drives real GM controls and checks audio received by a
separate Discord bot (see [test layers](../operations/test-layers.md)).
