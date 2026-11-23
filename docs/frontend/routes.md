---
type: UI Surface
title: Routes
description: Vue routes and how the shared table URL selects GM or player views.
resource: ui/src/router
tags: [frontend, routing]
---

# Routes

The Vue 3 application uses Vue Router, Vuetify, and Pinia.

| Route | Purpose |
| --- | --- |
| `/` | Landing page |
| `/login` | GM credentials; opens table management |
| `/setup` | [Claim a fresh instance](../backend/authentication.md) |
| `/settings` | GM profile and password changes |
| `/join` | Player invite-URL guidance |
| `/tables` | [Table management](./table-management.md) |
| `/library` | [Shared library](./shared-library.md) uploads and metadata management |
| `/tables/:externalID` | Shared GM/player table URL ([public table links](../backend/public-table-links.md)) |

Join as Player opens a small information page explaining that players need the
full `/tables/:externalID` URL from their GM; it creates no player session.

## Table route

The table route checks for an optional GM cookie and loads by public external
ID. It mounts the GM controls for a logged-in GM and the player view otherwise.
Players need no session, so switching links or opening multiple table tabs
requires no authentication exchange.

Table views show associated tracks only; GMs can add an existing library track
or upload a new one directly into the table through the
[Add Track dialog](./add-track-dialog.md). GM tables show the
[track grid](./track-grid.md) and [now playing](./now-playing.md) surfaces.

The player's audio button shows connection status and animated bars for local
media playback; clicking it disconnects audio. A hover or focus tooltip
describes playback, including starting and browser-blocked states. This reflects
media state, not the signal level or whether the output device is audible.
