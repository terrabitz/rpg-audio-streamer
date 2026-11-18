---
type: Domain Concept
title: Seek crossfades
description: Two-second crossfade seek semantics shared by browsers and the Discord mix.
resource: internal/mixer
tags: [playback, seeking, crossfade]
---

# Seek crossfades

Committed seeks during playback crossfade the outgoing position into the
destination over two seconds in both the browser and Discord mixes. The
selected time is the end of the incoming fade: a seek from 10 to 20 seconds
blends 10–12 with 18–20 and continues at 20.

## Bounds

- Seek targets are capped two seconds before the
  [playback-region](./playback-regions.md) end, leaving the final tail for the
  natural loop crossfade.
- For regions shorter than two seconds, only the beginning is seekable.
- A seek within the first two seconds of a region blends in from its beginning
  and continues two seconds later.
- Fades shorten to the available outgoing tail or region length.
- Paused seeks reposition silently.

## Versioned snapshots

[Playback synchronization](./synchronization.md) advances a seek version for
each seek. Snapshots carry a `seekTarget` for the selection and a `seekLanding`
for the end of the fade; `currentTime` starts at the incoming preroll and
advances throughout the fade, keeping control-only views and new listeners on
that clock.

Consecutive keyboard seeks advance from the pending selection until the fade
lands; the target then clears so later keys follow the live playhead. A local
listener keeps its pending target while its incoming media buffers.

## Implementations

- [Browser playback](./browser-playback.md) uses a second HLS media element.
- The [PCM mixer](../discord/pcm-mixer.md) implements `CrossfadeSeek`, which
  the [live mixer](../discord/live-mixer.md) invokes for Discord.
- The [track editor](../frontend/track-editor.md) and
  [now-playing](../frontend/now-playing.md) controls commit seeks on release or
  keyboard input.
