---
type: Protocol
title: WebSocket protocol
description: Event envelope, typed dispatch, recipient routing, and bounded delivery queues.
resource: internal/events/events.go
tags: [playback, websocket, protocol]
---

# WebSocket protocol

WebSocket text frames use `{ event, payload, senderId? }`. Event names and the
Go envelope live in `internal/events`; transport payload structs live in
`internal/websocket/protocol.go` and `playback.go`. The browser declares matching
payload interfaces and an event-to-payload `Payloads` map in
`ui/src/websocket/protocol.ts`. Nested table and region settings reuse application
types. These declarations are maintained together without code generation.

## Events

| Event | Direction | Payload |
| --- | --- | --- |
| `ping` / `pong` | Client to server / server to client | Opaque JSON diagnostic echo |
| `broadcast` | GM to server to other table clients | Opaque JSON diagnostic broadcast |
| `playbackSyncRequested` | Client to server | Empty object |
| `trackPlaybackChanged` | GM to server | Track ID and optional playback patch fields |
| `playbackStateChanged` | Server to client | Canonical track array; table settings included for GMs |
| `trackRemoved` | Server to client | Track ID |
| `mixChanged` | GM to server | Track patch array and optional type-volume map |
| `mixVolumesChanged` | GM to server | Type-volume map |
| `mixerReady`, `tableDeleted` | Server to client | Empty object |
| `mixerApplied` | Server to client | Applied event name in `event` |
| `mixerFailed` | Server to client | Failed event name in `event` and error message |
| `playbackFailed` | Server to client | Error message |
| `trackStatusChanged` | Server to GMs | Track ID, terminal processing status, and optional diagnostics |
| `discordStatusChanged` | Server to table GMs | Enabled flag, connection state, optional destination and error |

[Playback synchronization](./synchronization.md) describes playback events;
the [live mixer](../discord/live-mixer.md) describes mix events. Recipient
selection belongs to the hub: processing updates reach every connected GM,
Discord updates reach only GMs viewing that table, and playback stays table-scoped.
Socket sender identity always comes from the authenticated connection.

## Backend dispatch

Go producers depend on `events.Sender` and call `SendEvent(name, payload)`.
The hub owns a 256-entry FIFO queue shared by server and incoming socket events.
It serializes server payloads before enqueueing, so producers may reuse or mutate
their values afterward. Submission never waits for a handler, mixer cleanup, or
socket delivery. A full queue returns `ErrEventQueueFull`; a stopped hub returns
`ErrHubClosed`. Encoding errors also reject submission.

The hub runs handlers serially. `OnEvent(hub, name, func(payload T, client *Client))`
registers authenticated client handlers; `OnServerEvent` registers trusted Go
producer handlers. Registration happens before the hub starts, and registering
the same name replaces that origin's callback. A browser cannot invoke a
server-only handler, even with a GM connection.

Server handlers adapt domain payloads into browser payloads and own their
side effects. `tableDeleted` takes an internal table ID, clears its mixer and
playback, delivers the deletion event, and closes its clients. `trackRemoved`
takes `TableEvent[TrackReference]`; an empty table ID means library-wide removal.
The server-only `tableSettingsChanged` takes `TableEvent[TableSettingsPatch]`
and reconciles authoritative settings before publishing playback state.
Processing sends its track model; Discord sends `TableEvent[discord.Status]`.
Only the corresponding browser payloads leave the server.

HTTP producers log rejected submissions and report synchronization failure rather
than success; the database change may already be saved. Background processing
and Discord producers log submission failures. A socket producer that fills the
queue is disconnected so reconnect reconciliation can restore authoritative
state. Slow socket recipients are disconnected independently. Accepted events
are not coalesced; process shutdown ends delivery and closes clients.

Connection registration and player-access revocation remain synchronous
transport controls. Revocation cannot be rejected by the event queue, and
queued events from disconnected clients are ignored.

## Browser subscriptions

Any component can obtain the WebSocket store and call
`ws.sendEvent('trackPlaybackChanged', patch)`. The event name and payload are
coupled at compile time. Disconnected sends throw instead of replaying stale
intent after reconnect; the debug panel has a separate untyped entry point.

The store routes incoming events through `EventHub`. Pages subscribe with
`ws.on('playbackStateChanged', payload => { ... })`, receiving typed payloads
and optional sender metadata. Each subscription returns an idempotent
unsubscribe function for unmount cleanup. Independent subscribers survive
reconnects; throwing or rejecting callbacks are logged without preventing other
callbacks. Stale socket generations are ignored.

## Validation

The browser validates incoming envelopes, not payload schemas. Go decodes
payloads through typed adapters; optional patch fields use pointers so zero and
false remain distinct from omission. Authorization, table membership, regions,
and volume ranges remain domain checks. Invalid payloads are logged; playback
and mixer failures also produce their error events. Unknown client events are
logged; browser events without subscribers are ignored. Diagnostic echo and
broadcast accept arbitrary JSON.
