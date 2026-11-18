---
type: UI Surface
title: Track editor
description: Audio-controls dialog with waveform timeline, seeking, and region handles.
resource: ui/src/components/TrackTimeline.vue
tags: [frontend, playback, regions]
---

# Track editor

The audio-controls dialog (`AudioControls`, with `TrackTimeline` supplying the
editor) combines seeking and [playback-region](../playback/playback-regions.md)
editing on one full-duration timeline, with a play/pause button to its left
that uses the same playback path as the track tile. It opens from
[track tiles](./track-grid.md), [now-playing cards](./now-playing.md), and the
[shared library](./shared-library.md).

## Waveform

When the dialog opens it fetches the track's
[waveform](../data/track-uploads.md) once and draws it as the timeline bar: the
played portion is bright, the unplayed region dimmer, and trimmed areas grey.
If the waveform cannot be loaded the plain bar is shown instead.

## Region handles

Beginning and end handles sit above the bar; repeating types also have a
repeat-icon loop-back handle below it. The selected type controls its
visibility without discarding saved loop-back points. Areas outside the
selected region are dimmed and cannot be selected for seeking. Handles support
mouse/touch dragging and keyboard adjustment and display their timestamps
directly.

Moving a boundary keeps the loop-back point inside the region. The end defaults
to the track duration; moving the loop-back handle to the beginning reanchors
it so it follows later beginning adjustments. Moving the beginning to zero and
the end to the full duration restores the full track without separate reset
controls.

While dragging, marker labels and shading preview locally without changing
audio or sending region commands. Canceling discards that local preview;
keyboard edits commit immediately. Point edits update live playback on pointer
release (including listening players at the current table) and save to shared
library metadata automatically.

## Seeking

Dragging a seek cursor previews its position and time without changing audio;
releasing commits one [seek crossfade](../playback/seek-crossfades.md).
Canceling a timeline seek discards the preview. The released preview remains
visible until the seek version acknowledges the command (see
[playback synchronization](../playback/synchronization.md)), so intervening
clock reports cannot flash the previous position. Keyboard seeking commits
immediately.

## Saving

Name and region edits are briefly debounced; type changes save immediately.
Saves are serialized and contain only fields explicitly edited in this dialog.
Unopened controls never save on disposal. Remote metadata updates refresh
untouched fields, without turning stale drafts into writes or replacing pending
local edits. Closing waits for pending saves; invalid edits are not applied.
The metadata dialog retains the track-type selector. Save failures preserve
pending edits and show a [dismissible error toast](./app-shell.md#notifications)
with a separate retry action in the dialog.

## Volume

The track-settings dialog opened from a [now-playing card](./now-playing.md)
adds a volume slider aligned to the timeline and a play/pause button, but no
restart button. The editor's volume and seek bars share the same width,
thickness (6 px tracks), and thumb size (18 px), with elapsed/total time
centered vertically to the right of the seek bar.
