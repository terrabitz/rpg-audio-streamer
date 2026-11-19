---
type: Data Model
title: SQLite schema
description: Metadata tables, identifier encoding, seeded track types, and connection model.
resource: sql/migrations
tags: [data, sqlite, persistence]
---

# SQLite schema

SQLite stores metadata, not media bytes; audio lives on disk as HLS (see
[track uploads](./track-uploads.md)). `sql/migrations` defines schema
evolution and `sql/queries` is the authoritative query source for the
generated sqlc code (see [Go packages](../backend/packages.md)).

# Schema

| Table | Contents |
| --- | --- |
| `tracks` | UUID ID, creation timestamp, display name, disk path, track-type ID, shared [playback-region](../playback/playback-regions.md) points (beginning, optional end, optional loop-back), [processing status](./track-processing.md) (`processing`, `ready`, or `failed`), processing diagnostics, and a nullable JSON [waveform](./track-uploads.md#waveforms) |
| `track_types` | UUID ID, name, color, repeat policy, simultaneous-play policy, and creation timestamp |
| `game_tables` | UUID ID, unique random Base64URL external ID, name, JSON GM [mixing/ordering settings](./table-settings.md), and saved [Discord destination](../discord/voice-streaming.md) (`discord_destination`) |
| `table_tracks` | Many-to-many [table/track associations](./table-track-associations.md) |

The `game_tables` external ID is the [public table identifier](../backend/public-table-links.md)
used in player URLs; the UUID remains the internal database identity.

## Encoding

UUIDs are stored as BLOBs. Track creation timestamps are serialized as RFC3339
text, with second precision. Partial updates preserve omitted name/type fields.
Empty metadata collections are returned as JSON arrays, not `null`.

Playback-region points occupy discrete `tracks.start_time`, `tracks.end_time`,
and `tracks.loop_back_time` columns. Metadata patches only write these columns
when the GM explicitly edits the region in the track-settings dialog.
Playback snapshots, media duration reports, navigation, and refreshes do not
persist region settings.

## Seeded track types

The initial migration seeds:

| Type | Repeat policy | Simultaneous play |
| --- | --- | --- |
| Ambiance | Repeating | Simultaneous |
| Music | Repeating | Exclusive |
| One-Shot | Non-repeating | Simultaneous |

Exclusivity is enforced during [playback synchronization](../playback/synchronization.md).

## Connection model

The datastore uses one pooled SQLite connection to serialize in-process
access, including background processing updates, so concurrent settings writes
cannot lock out table or stream reads.
