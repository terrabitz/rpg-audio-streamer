---
type: Domain Concept
title: Track tags
description: Shared colored labels that GMs attach to library tracks, and who can see them.
resource: internal/domain/tag.go
tags: [data, library, tags]
---

# Track tags

Tags are labels shared by every track in the library. A track can carry any
number of tags, and a tag can be attached to any number of tracks.

## Rules

- A name is 1–40 characters after trimming and is unique without regard to
  case.
- A color is a `#RRGGBB` hex value. The UI offers a fixed palette of named
  swatches, and new tags cycle through it.
- Tags can be renamed and recolored. A tag is deleted automatically when it is
  removed from its last track, either by a track edit or by deleting the
  track.

## Storage and API

The `tags` table holds names and colors, and `track_tags` joins tags to tracks
(see the [SQLite schema](./sqlite-schema.md)). Each track's `tagIDs` lists its
tags in name order and is omitted when the track has none.

All tag endpoints are GM-only:

| Endpoint | Purpose |
| --- | --- |
| `GET /api/v1/tags` | List tags in name order |
| `POST /api/v1/tags` | Create a tag; a duplicate name returns 409 |
| `PUT /api/v1/tags/{tagID}` | Rename or recolor a tag; a duplicate name returns 409 |
| `PATCH /api/v1/files` | Edit several tracks at once and return them |

The bulk track edit takes `trackIDs` plus any of `typeID`, `addTagIDs`, and
`removeTagIDs`. It applies every change in one transaction, so an unknown track
or tag fails the whole edit. Adding a tag a track already has, or removing one
it lacks, is a no-op, and a tag cannot be both added and removed.

Saving a tag pushes `tagUpdated`, editing tracks pushes `trackUpdated` per
track, and deleting an unused tag pushes `tagDeleted`, to all GMs through the
[WebSocket protocol](../playback/websocket-protocol.md).

## Visibility

Tags are GM-only. Players' playback snapshots carry no tag data. GMs manage
tags in the [shared library](../frontend/shared-library.md) and see them as
color blocks on [track grid](../frontend/track-grid.md) tiles.
