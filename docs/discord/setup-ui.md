---
type: UI Surface
title: Discord setup UI
description: Table Discord control, setup dialog, and destination endpoints.
resource: ui/src/composables
tags: [discord, frontend, api]
---

# Discord setup UI

Each table has a Discord control at the bottom of its desktop
[Currently playing sidebar](../frontend/now-playing.md), or above the tracks in
its mobile drawer. It shows a grey Connect button when disconnected, a loading
spinner while connecting, and a green Connected button while streaming.
Clicking Connected disconnects; hovering reveals the destination server and
voice channel. Returning to a table requires clicking Connect again; a saved
destination does not connect automatically. The button uses the Discord logo.

## Setup dialog

The setup dialog loads `GET /api/v1/discord/setup`, which obtains the
application ID and the bot's installed servers from Discord. The server
dropdown ends with an `+ Add to server` option that opens Discord's
authorization page with only bot scope and View Channel, Connect, and Speak
permissions. The user approves installation in Discord, returns to the app, and
refreshes the server list using the adjacent refresh icon. No callback URL or
OAuth client secret is needed. The app's single configured GM account is
authorized to select any server that has installed this configured bot.

The dialog prompts for a server and a regular voice channel. The GM-only
`GET /api/v1/discord/servers/{guildID}/channels` endpoint lists channels.

## Destination endpoints

`POST /api/v1/tables/{tableID}/discord` accepts the selected server/channel
IDs, verifies their association through Discord, connects, and saves the
destination and display names in that table's SQLite `discord_destination`
column (see [schema](../data/sqlite-schema.md)). A failed save cancels the
connection. An omitted request body reconnects the saved destination. `GET`
returns state and the saved destination; `DELETE` disconnects only this table
without forgetting it.

The GM table loads the initial status with its other
[page data](../frontend/app-shell.md#page-loading), and the composable fetches
status again when the table WebSocket connects or reconnects. Server-side connection transitions arrive through the GM-only,
table-scoped [`discordStatusChanged` event](../playback/websocket-protocol.md), without
polling. Pushed status takes precedence over in-flight HTTP responses and clears
resolved connection errors. The composable prevents duplicate commands,
discards stale status/channel responses, and displays failures. Connection behavior is described in
[Discord voice streaming](./voice-streaming.md).
