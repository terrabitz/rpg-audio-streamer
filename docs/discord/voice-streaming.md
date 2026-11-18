---
type: Subsystem
title: Discord voice streaming
description: Configuration, per-server voice connections, DAVE transport, and reconnection.
resource: internal/discord
tags: [discord, voice, configuration]
---

# Discord voice streaming

## Configuration

Set `DISCORD_STREAMING_ENABLED` and `DISCORD_STREAMING_BOT_TOKEN` to enable
Discord setup and the [live mixer](./live-mixer.md). When disabled, the UI hides
Discord controls; enabling it without a token fails startup. The server uses
its configured bot identity; the GM's existing app login authorizes controls
(see [authentication](../backend/authentication.md)). This path does not link a
Discord user account or use OAuth client secrets. Voice transport runs in the
Go API process. GMs choose destinations through the
[Discord setup UI](./setup-ui.md).

## Voice connections

`internal/discord` reserves one voice connection per Discord server. Tables can
stream concurrently in different servers, but another table in the same server
receives a conflict and cannot take over the connection. Voice startup does not
hold the rendering lock; the PCM subscription is created only once voice is
ready, avoiding a full subscriber queue during the handshake.

## Transport

The transport encodes 48 kHz stereo PCM into Opus and sends it over Discord's
DAVE-encrypted voice connection. It reannounces the speaking source when
participants join and periodically so late listeners can associate incoming
packets with the bot. The stream bot joins self-deafened and does not receive
participants' audio. Bot credentials are not passed in process arguments,
browser payloads, or logs.

DAVE uses Discord's native libdave through the `godave/golibdave` Go bindings
in the server process; MLS itself is implemented by native libdave. The Linux
image uses glibc and includes the pinned libdave shared library and its
licenses. Development and CI install the same verified upstream release; a C
compiler and pkg-config are needed to build.

## Lifecycle and reconnection

Disconnecting, closing the table mix, loss of its last GM, or renderer shutdown
ends the subscription and voice connection. Transport failures and slow PCM
consumers become observable error states.

Status notifications use producer-owned goroutines to call the
[synchronous event bus](../playback/websocket-protocol.md). Each notification
waits for its predecessor, preserving transition order without running event
listeners under the service mutex. Connection startup and teardown do not wait
for notifications, so a slow or reentrant listener cannot stall voice lifecycle
cleanup.

The first successful mixer sync of a GM table session follows an authoritative
[playback snapshot](../playback/synchronization.md), so a reconnect sync
includes the table's active tracks before attempting the saved destination
once. Repeated playback updates and status reads do not reconnect a manually
disconnected bot. Leaving a table or closing its last GM tab cancels pending
reconnection and disconnects the connection. Returning creates a new session
and attempts reconnection. Multiple GM tabs keep the bot active until the last
one closes. A lost network connection is detected by the WebSocket heartbeat.
Reconnection conflicts and failures are visible and require an explicit retry
or destination change within that session.

## Validation

The Playwright journey drives the real UI and uses a separate Node listener bot
with `@discordjs/voice` DAVE decryption and Opus decoding. It measures the two
generated tones and silence after playback, volume, pause, disconnect, and
reconnect controls. It verifies delivery through Discord rather than trusting
sender-side telemetry. Voice receiving is not a stable, documented Discord API
contract. See [test layers](../operations/test-layers.md).
