---
type: UI Surface
title: Now playing
description: Desktop sidebar and mobile drawer of currently playing tracks.
resource: ui/src/components/PlayingTrackList.vue
tags: [frontend, playback, layout]
---

# Now playing

A currently-playing surface lists active table tracks with volume,
playback-position, elapsed/total time, pause, and restart controls, and
provides master volume even when no tracks are playing. `PlayingTrackList`
renders the same cards in the desktop sidebar and the mobile drawer;
`TrackPlaybackControls` supplies the compact controls. It also hosts the
[Discord control](../discord/setup-ui.md).

## Desktop sidebar

The 260 px desktop sidebar sits flush against the right edge below the app bar,
with a distinct surface, left divider, full available viewport height, and
independent scrolling, beside the [track grid](./track-grid.md).

## Mobile drawer

On smaller screens, a fixed bottom bar shows the playing-track count and opens
a scrollable bottom drawer rather than taking space above the grid. The drawer
is capped at 70% of the viewport height and supports closing with its button,
Escape, or the backdrop; closing it does not affect playback. Now-playing cards
wrap into compact columns when space permits, including inside the tablet
bottom drawer; narrow phones keep a single column.

## Cards

Compact cards use a single-line name and tightly spaced controls to preserve
grid space. Truncated names keep their full accessible label and hover title.
Each card's always-visible three-dot button opens the track-settings variant of
the [track editor](./track-editor.md).

Paused, completed, removed, and type-exclusive displaced tracks leave the list;
starting a track adds it automatically.

## Controls

The timer and seek bar's accessible value text measure elapsed time from the
configured beginning and total time to the configured end of the
[playback region](../playback/playback-regions.md). Seeking still uses absolute
positions within the source audio.

Compact seek sliders commit on release; keyboard seeking and restart controls
commit immediately (see [seek crossfades](../playback/seek-crossfades.md)).
Seeking preserves the playing/paused state; playing tracks synchronize volume
and seek changes to listening players. Restart seeks to the beginning.

Repeating tracks with a loop-back point distinct from the beginning show a
small orange tick on the compact seek bar. Anchored loop-back points have no
tick. Volume and seek bars use 6 px tracks and 18 px thumbs, matching the
editor.
