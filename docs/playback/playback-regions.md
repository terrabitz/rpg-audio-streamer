---
type: Domain Concept
title: Playback regions
description: Beginning, end, and loop-back points and how live edits move playheads.
resource: internal/websocket/playback.go
tags: [playback, regions]
---

# Playback regions

Each library track stores a beginning, an optional end, and an optional
loop-back point (see the `tracks` [schema](../data/sqlite-schema.md)). These
settings are stored with the shared library track and apply in every table.
GMs edit them in the [track editor](../frontend/track-editor.md).

Playback starts at the configured beginning and stops or loops at the
configured end. The end defaults to the track duration and the loop-back point
defaults to the beginning. Repeating tracks loop to the loop-back point with a
crossfade (see [browser playback](./browser-playback.md) and the
[PCM mixer](../discord/pcm-mixer.md)).

## Live edits

Region edits preserve each listener's live playhead without seeking to a
reported position:

- A playhead before the new beginning crossfades into that beginning.
- A repeating playhead beyond the new end loops to the loop-back point.
- An in-flight loop crossfade keeps its incoming source playing through marker
  edits, preserving its position when valid and moving it into the edited
  region otherwise.
- [Seek crossfades](./seek-crossfades.md) preserve their gain progress when
  marker edits move the incoming source, and a pending selection outside the
  edited region is cleared.
- Shortening the end can complete that handoff early; extending it applies to
  the next loop.
- A source that reaches its end without a prepared crossfade starts the next
  loop from the configured loop-back point.
