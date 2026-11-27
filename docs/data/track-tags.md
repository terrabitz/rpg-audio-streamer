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
- Tags can be renamed and recolored but not deleted. Deleting a track removes
  its tag associations.

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
| `PUT /api/v1/files/{trackID}/tags/{tagID}` | Attach a tag to a track and return the track |
| `DELETE /api/v1/files/{trackID}/tags/{tagID}` | Detach a tag from a track and return the track |

Saving a tag pushes `tagUpdated`, and attaching or detaching pushes
`trackUpdated`, to all GMs through the
[WebSocket protocol](../playback/websocket-protocol.md).

## Visibility

Tags are GM-only. Players' playback snapshots carry no tag data. GMs manage
tags in the [shared library](../frontend/shared-library.md) and see them as
color blocks on [track grid](../frontend/track-grid.md) tiles.
