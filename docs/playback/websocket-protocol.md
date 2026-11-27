---
type: Protocol
title: WebSocket protocol
description: Event envelope, synchronous typed dispatch, subscriptions, and recipient routing.
resource: internal/events/events.go
tags: [playback, websocket, protocol]
---

# WebSocket protocol

The browser receives JSON text frames with `{ event, payload }`. It sends no
application messages: [playback commands](./synchronization.md) and
[mix commands](../discord/live-mixer.md) use HTTP. The server sends a WebSocket
Ping control frame every 40 seconds. Browsers automatically return Pong control
frames, which extend the server's 60-second read deadline. This heartbeat keeps
idle connections active behind proxies and detects dead peers.

## Browser events

| Event | Recipients | Payload |
| --- | --- | --- |
| `playbackStateChanged` | Table clients | Revision, tracks, and GM-only settings |
| `trackRemoved` | Table clients, or all clients for library deletion | Track ID |
| `tableDeleted` | Table clients | Empty object; connection closes afterward |
| `trackStatusChanged` | All GMs | Track ID, terminal processing status, and optional diagnostics |
| `trackUpdated` | All GMs | Saved track metadata including playback region and tag IDs |
| `tagUpdated` | All GMs | Saved [tag](../data/track-tags.md) ID, name, and color |
| `tagDeleted` | All GMs | ID of a [tag](../data/track-tags.md) deleted after its last track lost it |
| `discordStatusChanged` | Table GMs | Connection state, optional destination and error |
| `mixerReady` | Newly connected GM | Empty object |
| `mixerApplied` | Table GMs | Applied command name |
| `mixerFailed` | Table GMs | Failed command name and error message |

Go event contracts live in `internal/events`. The browser routes typed
subscriptions through `ui/src/websocket/router.ts`. Each subscription returns
an idempotent cleanup callback, remains registered across reconnects, and is
isolated from other callbacks' failures. Stale socket generations are ignored.

## Server event bus

Go producers call `events.Sender.SendEvent`; the hub runs trusted producer
handlers and typed subscribers synchronously on the caller's goroutine.
Callbacks finishing does not imply the browser received its buffered frame.
Independent dispatches can overlap. Encoding, admission, and shutdown errors
are returned to the producer; subscriber failures are logged. The playback
service serializes its own state changes and publishes through an output
interface.

Services and connections subscribe through `events.Bus`, using named typed
helpers such as `events.OnPlaybackChanged` and `events.OnTrackRemoved`. Each
listener receives its own decoded payload. The hub takes a dispatch snapshot,
so registering or unsubscribing only changes later dispatches. Metadata scopes
events to a table, a particular connection, or explicit global delivery.
Socket listeners project payloads by role and apply these recipient filters.
Slow consumers and closed connections lose their subscriptions independently.

Internal events include `clientConnected`, `tableSessionEnded`,
`playbackChanged`, `mixChanged`, `mixVolumesChanged`, and
`tableSettingsChanged`. `playbackChanged` carries accepted partial track
patches, with any exclusivity stops before the requested update. HTTP snapshot
reads and periodic pushes do not publish playback mutations.
