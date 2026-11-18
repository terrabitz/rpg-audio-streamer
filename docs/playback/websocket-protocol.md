---
type: Protocol
title: WebSocket protocol
description: Event envelope, synchronous typed dispatch, subscriptions, and recipient routing.
resource: internal/events/events.go
tags: [playback, websocket, protocol]
---

# WebSocket protocol

WebSocket text frames use `{ event, payload, senderId? }`. Event names and the
Go envelope and published payload structs live in `internal/events`; shared
playback, table-settings, and Discord value models live in `internal/domain`.
The browser declares matching
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
the [live mixer](../discord/live-mixer.md) describes mix events. Socket
subscriptions determine recipients: processing updates reach every connected
GM, Discord updates reach only GMs viewing that table, and playback stays table-scoped.
Socket sender identity always comes from the authenticated connection.

## Backend dispatch

Go producers depend on `events.Sender` and call `SendEvent(name, payload)`.
The hub serializes the payload and runs handlers and subscribers synchronously
on the caller's goroutine. Returning means those callbacks have finished, not
that a browser has received its buffered socket frame. There is no shared event
queue. Unknown server events, encoding failures, and shutdown reject sends;
shutdown returns `ErrHubClosed`. Handler and subscriber failures are logged;
they are not returned by `SendEvent`.

Socket events run on their connection's read pump, preserving per-connection
order. Independent callers can dispatch concurrently, so consumers protect
their own state and callers own cross-event ordering. Playback mutations and
snapshots share the hub's state lock. Callers that need asynchronous dispatch
start their own goroutine; [Discord status notifications](../discord/voice-streaming.md)
use this boundary without reordering transitions.

`OnEvent(hub, name, func(payload T, client *Client))`
registers authenticated client handlers; `OnServerEvent` registers trusted Go
producer handlers. Registration happens before the hub starts, and registering
the same name replaces that origin's callback. A browser cannot invoke a
server-only handler, even with a GM connection.

Server handlers validate changes, maintain authoritative playback, and adapt
domain payloads into published events. `tableDeleted` takes an internal table ID,
clears playback, publishes deletion, and closes its clients. `trackRemoved`
takes `TableEvent[TrackReference]`; an empty table ID means library-wide removal.
The server-only `tableSettingsChanged` takes `TableEvent[TableSettingsPatch]`
and reconciles authoritative settings before publishing playback state.
Processing sends its track model; Discord sends `TableEvent[discord.Status]`.
Only the corresponding browser payloads leave the server.

HTTP producers log rejected sends and report synchronization failure rather than
success; the database change may already be saved. Background processing and
Discord producers log send failures. Slow socket recipients are disconnected
independently. Shutdown rejects new dispatches, waits for in-flight handlers,
and then closes clients.

Connection registration and player-access revocation are synchronous transport
controls. Events arriving from disconnected clients are ignored.

## Shared server subscriptions

Services and socket connections subscribe through the same `events.Bus`.
Consumers use named registration functions such as
`events.OnTrackRemoved(bus, func(events.TrackReference, events.Metadata) error)`
and `events.OnPlaybackStateChanged(bus, func(events.Snapshot, events.Metadata) error)`.
Each function fixes the event-to-payload pairing at compile time. The bus's
low-level `On` hook and JSON decoding are confined to the subscription codec;
listeners receive concrete payloads. Diagnostic ping, pong, and broadcast
payloads remain intentionally opaque JSON.

The hub implements the bus without owning its consumers. Each registration returns an
independent, idempotent unsubscribe function. Registration is concurrency-safe;
callbacks run in registration order using a dispatch snapshot, so changes to
subscriptions affect subsequent dispatches. Callback errors are logged without
preventing other subscribers from running. Within one dispatch, callbacks run
serially on the caller's goroutine; separate dispatches may overlap.
Each listener receives its own decoded payload,
so changing a slice, map, or pointer cannot affect another listener.

Subscribers observe authorized events with table, client, and role scope
supplied by the hub rather than socket payloads. `ClientID` identifies the
originating connection; `TargetClientID` identifies a particular recipient.
Global delivery is explicit: an empty table ID alone means a library-scoped
connection, not every table. Playback validation and state
resolution precede notification; mix-specific payload validation belongs to
the mixer consumer. `tableSettingsChanged` subscribers receive reconciled
`TableSettings`, not the original HTTP patch.

On connection, each socket registers typed listeners for its authorized events.
GM sockets additionally subscribe to processing, Discord, and mixer results.
Listeners enforce table and recipient scope; the hub does not iterate clients
to route published events. Socket subscriptions are removed on disconnect,
slow-consumer eviction, player-access revocation, table deletion, and shutdown.
There is no browser-controlled subscription request or additional wire handshake.

Playback publishes one canonical snapshot, including unscaled track volumes and
table settings. The socket listener sends GMs that snapshot; for players it
applies the table master volume and removes settings. The canonical event remains
unchanged for server services and other connections.

The hub also publishes internal-only events: `clientConnected` carries an empty
object with authenticated connection scope; `tableSessionEnded` identifies a
table losing its last GM; `playbackChanged` carries a `tracks` array of accepted
partial patches. Playback batches include exclusivity stops followed by the
requested update, with volume and region resolved on play. Persisted track-volume
changes use the same event without an originating client. Periodic and targeted
`playbackStateChanged` snapshots do not generate these mutation events.

The [mixer event adapter](../discord/live-mixer.md) subscribes independently and
submits targeted `mixerReady`, `mixerApplied`, and `mixerFailed` responses as
`ClientEvent` wrappers. Only their inner payloads reach the specified table and
client. Connection observers run before socket reads begin, ensuring readiness
is placed in the socket's output buffer before the initial playback-sync request.

## Browser subscriptions

Any component can obtain the WebSocket store and call
`ws.sendEvent('trackPlaybackChanged', patch)`. The event name and payload are
coupled at compile time. Disconnected sends throw instead of replaying stale
intent after reconnect; the debug panel has a separate untyped entry point.

The store routes incoming events through `EventHub`. Pages subscribe with
`ws.onPlaybackStateChanged(payload => { ... })` or
`ws.onTrackRemoved(payload => { ... })`, receiving typed payloads
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
