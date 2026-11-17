---
type: Security Model
title: Public table links
description: External IDs that grant player access to a table and how GMs rotate them.
resource: internal/server
tags: [backend, security, tables]
---

# Public table links

Each table has a database UUID and a separate, randomly generated Base64URL
external ID (see the `game_tables` [schema](../data/sqlite-schema.md)). Anyone
with `/tables/<external-id>` can read that table's name and associated tracks,
stream those tracks, and connect to its WebSocket without signing in or
exchanging credentials.

API stream URLs and WebSocket connections carry the external ID; the server
resolves it to the internal UUID for table-scoped playback and track access.
There is no player session, so each tab can open a different table
independently. A valid GM cookie on the same URL selects the GM controls (see
[authentication](./authentication.md)); it does not change the URL.

## Rotation

GMs can rotate the external ID; this invalidates the old URL and disconnects
connected players. The [app shell](../frontend/app-shell.md) and
[table management](../frontend/table-management.md) expose rotation behind a
confirmation dialog.
