---
type: Protocol
title: WebSocket protocol
description: Command envelope, command table, typed handlers, and validation.
resource: internal/websocket/protocol.go
tags: [playback, websocket, protocol]
---

# WebSocket protocol

WebSocket text frames use `{ method, payload, senderId? }`. The Go side
declares command-name constants and ordinary payload structs in
`internal/websocket/protocol.go`, with playback models in `playback.go`. The
browser declares matching payload interfaces and a command-to-payload
`Payloads` map in `ui/src/websocket/protocol.ts`. Nested table and region
settings reuse existing application types. The declarations are maintained
together; there is no WebSocket code generation or runtime schema registry.

## Commands

| Commands | Direction | Payload |
| --- | --- | --- |
| `ping` / `pong` | Client to server / server to client | Opaque JSON diagnostic echo |
| `broadcast` | GM to server to other table clients | Opaque JSON diagnostic broadcast |
| `syncRequest` | Client to server | Empty object |
| `syncTrack` | GM to server | Track ID and optional playback patch fields |
| `syncAll` | Server to client | Canonical track array; table settings included for GMs |
| `mixSync` | GM to server | Track patch array and optional type-volume map |
| `mixVolumes` | GM to server | Type-volume map |
| `mixerReady`, `tableDeleted` | Server to client | Empty object |
| `mixerAck` | Server to client | Acknowledged command name |
| `mixerError` | Server to client | Failed command name and message |
| `playbackError` | Server to client | Error message |
| `trackStatus` | Server to GMs | Track ID, `ready` or `failed` status, and optional processing diagnostics |

[Playback synchronization](./synchronization.md) describes the playback
commands; the [live mixer](../discord/live-mixer.md) describes the mixer
commands.
The [processing service](../data/track-processing.md) sends `trackStatus` to all
connected GMs, not just those viewing the affected table.

## Backend handlers

Backend handlers register before connections are accepted with
`Handle(hub, SyncTrack, func(patch PlaybackPatch, client *Client) { ... })`.
The small generic adapter infers the payload type from the callback and uses
`json.Unmarshal` before invoking it. Registering the same name replaces its
callback. Authorization and domain validation stay in the application handlers.
`SendPayload` and `BroadcastPayload` encode ordinary Go values into the
envelope and keep the hub's table/role/recipient filters.

## Browser subscriptions

Browser pages subscribe with `ws.on('syncAll', payload => { ... })`, receiving
typed payloads and optional sender metadata rather than inspecting every
message. Each subscription returns an idempotent unsubscribe function, which
pages invoke on unmount. Multiple subscribers are independent; a throwing or
rejecting subscriber is logged without preventing other callbacks. Subscribers
survive socket reconnects, while stale socket generations are ignored.
`sendMessage` couples each command name to its payload type at compile time.
The debug panel uses a separate untyped entry point for form input and replay.

## Validation

The browser checks the incoming message envelope, then dispatches the payload
using its declared TypeScript type; it does not validate payload schemas at
runtime. Go uses standard JSON decoding, including pointer fields for optional
patch values so zero and false remain distinct from omission. Undecodable
messages are logged; invalid playback and mixer commands also use their error
responses. Table membership, playback-region constraints, and volume ranges
remain domain checks. The server logs commands without a registered handler;
the browser ignores messages without subscribers. Diagnostic echo and
broadcast accept arbitrary JSON.
