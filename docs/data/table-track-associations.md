---
type: Data Model
title: Table track associations
description: How tracks join tables and what deletion removes.
resource: sql/migrations
tags: [data, tables, library]
---

# Table track associations

Tracks live in one shared library; the `table_tracks`
[table](./sqlite-schema.md) associates them with tables many-to-many.

The table migration seeds "My Table" and associates all pre-existing tracks
with it. New tables start empty.

## Deletion semantics

- Removing a track association or deleting a table never deletes library audio.
- Deleting a library track removes its associations everywhere.
- Cleanup triggers enforce association removal even on SQLite connections
  without foreign keys enabled.

Removing an association stops that track for the table; library deletion stops
it in every connected table (see [playback synchronization](../playback/synchronization.md)
and the [live mixer](../discord/live-mixer.md)). The
[shared library](../frontend/shared-library.md) confirms deletions that affect
tables.
