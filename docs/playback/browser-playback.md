---
type: Component
title: Browser playback
description: HLS.js media elements, loop crossfades, and gain ramps in the browser.
resource: ui/src/components/AudioTrackPlayer.vue
tags: [playback, frontend, hls]
---

# Browser playback

HLS.js binds authenticated playlists to hidden media elements
(`AudioPlayer`, `AudioTrackPlayer`; see [client state](../frontend/state-management.md)).
Track state from [playback synchronization](./synchronization.md) controls
play/pause, seeking, looping, and volume. Significant seek differences are
reconciled.

## Loop crossfades

Playback starts at the configured beginning and stops or loops at the
configured end of the [playback region](./playback-regions.md). Repeating
tracks use a second HLS media element to crossfade over a global two-second
interval (shortened for shorter loop regions), starting the incoming source at
the loop-back point or configured beginning.

## Seek crossfades

[Seek crossfades](./seek-crossfades.md) use the second HLS media element for
repeating and non-repeating tracks, keeping the outgoing source audible while
the destination loads. The displayed clock follows the destination during the
overlap. Another seek adopts the destination once its fade has begun, otherwise
keeping the audible outgoing source; pause adopts the incoming position, clears
the pending selection, and releases the tail.

## Play requests

Each media element tracks its current play request. Intentional pauses or
replacement requests invalidate pending work, so delayed rejections from
canceled requests cannot stop healthy playback; failures of current requests
still surface.

## Gain ramps

Track, master, and type-volume changes ramp the mixed gain over two seconds
independently of play/pause fades and loop crossfades. A new adjustment starts
from the current interpolated gain, including mute and non-repeating tracks.

## Disposal

Removing a media component destroys its HLS instances and clears playback
timers; queued media events cannot recreate deleted track state.
