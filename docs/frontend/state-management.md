---
type: Component Map
title: Client state
description: Pinia stores, components, and what browsers persist.
resource: ui/src/stores
tags: [frontend, state, persistence]
---

# Client state

Server-side changes reach connected browsers through
[WebSocket events](../playback/websocket-protocol.md), not periodic HTTP
polling. Initial loads and reconnect reconciliation fetch authoritative state.

| Surface | Responsibility |
| --- | --- |
| `stores/auth.ts` | GM session role, credentials, and logout ([authentication](../backend/authentication.md)) |
| `stores/files.ts`, `stores/trackTypes.ts`, `stores/tags.ts`, `stores/tables.ts` | Library, track-type, tag, table, and association metadata through the generated SDK |
| `stores/audio.ts` | Per-file playback state, [synchronization](../playback/synchronization.md) reconciliation, fades, master/type volumes |
| `stores/playbackCommands.ts` | Queued HTTP playback and Discord mixer commands ([synchronization](../playback/synchronization.md)) |
| `stores/websocket.ts` | Socket connection/reconnection and named subscriptions to server events such as `onTrackRemoved` ([protocol](../playback/websocket-protocol.md)) |
| `stores/clientPreferences.ts` | Browser-persisted [screen-awake](./screen-wake-lock.md) preference |
| `composables/` | Shared app-bar state, browser origin, immutable partial-object patches, screen wake-lock lifecycle |
| `FileList`, `AudioControls`, `PlayingTrackList`, `TrackPlaybackControls`, `TrackTimeline`, `AudioUploader` | [Shared-library](./shared-library.md) metadata management and [uploads](./add-track-dialog.md); table associations, responsive [now-playing](./now-playing.md) surfaces, and shared playback controls ([track editor](./track-editor.md)) |
| `AudioPlayer`, `AudioTrackPlayer` | Per-track HLS.js media elements and [browser playback](../playback/browser-playback.md) |
| `VolumeMixer`, `VolumeSlider` | Active master and per-type mixing; player preferences stay local |

## Persistence

Library and track-type metadata and player mixer preferences use the Pinia
persistence plugin. GM mixes and track order load from the server's
[table settings](../data/table-settings.md) before table controls become
available; they do not inherit browser-local mixer preferences. Mix edits are
debounced and flushed when leaving the table; saves are serialized with
ordering edits, and failures show an explicit retry action. Table metadata, GM
settings, and playback state are not persisted in browser storage.

## Playback state

Browser playback is keyed by file UUID within the active table and cleared on
table changes, so even the same library track cannot inherit another table's
playback. GM controls wait for the server's playback snapshot before becoming
available. Local browser state is not authoritative for stored metadata or
table playback.

Players must explicitly connect audio; full-state synchronization is ignored
while listening is disabled. Associated entries initialize GM playback state as
they load.

## SDK errors

The SDK's shared runtime configuration throws on HTTP failures so store and
upload error handlers cannot report false success.

## Processing updates

The GM table and library subscribe to
[`trackStatusChanged`](../playback/websocket-protocol.md), applying terminal processing
status and diagnostics to their track lists without polling. A push arriving
before the upload response is held in a nonpersisted unmatched-status map
until `addTrack()` adds the track. Successful library fetches clear that map.

Reloading, or reconnecting while tracks are still processing, refetches
authoritative metadata. Table track lists also refetch on mount after socket
synchronization. These updates drive the
[processing indicators and diagnostics](./track-processing.md).
