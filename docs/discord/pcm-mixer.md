---
type: Component
title: PCM mixer
description: Standalone deterministic stereo PCM mixing core.
resource: internal/mixer
tags: [discord, audio, mixing]
---

# PCM mixer

For a hands-on reconstruction, follow the
[PCM mixer course](../tutorials/pcm-mixer-course.md), from your first audible
frame through the core's controls, transitions, and source lifecycle.

`internal/mixer` is not part of the HTTP, WebSocket, or browser playback path;
the [live mixer](./live-mixer.md) drives it. It accepts normalized stereo PCM
frames at one caller-selected sample rate. `Add` copies an in-memory clip and
leaves it paused; `AddSource` accepts a bounded indexed source (such as the
[PCM source](./pcm-source.md)) whose reads never block rendering.

`Play`, `Pause`, `Seek`, `CrossfadeSeek`, `SetRegion`, `SetRepeating`,
`SetGain`, and `Remove` control tracks; `State` exposes their absolute frame
position and requested playing state. `Render` overwrites a caller-owned output
buffer and advances the mix by that many frames. No device, codec, network
connection, clock, timer, or goroutine is created. One owner must serialize
renders and controls.

## Regions and loops

[Regions](../playback/playback-regions.md) use absolute frame offsets with an
exclusive end; initialization may land exactly on the end, resolving completion
or looping when playback renders. A caller resolves an unset end to the decoded
clip length and an unset loop-back to the beginning before setting a region.
Completion stops non-repeating tracks and resets their position to the
beginning.

Repeating tracks overlap the outgoing tail with an incoming cursor at the
loop-back point using complementary linear gains. The overlap length is
`min(2 * sampleRate, (end - loopBack) / 3, end - start)`, rounded down to whole
frames. The incoming cursor advances during the overlap and continues from that
position afterward, so an intro before the loop-back is not replayed. Sub-frame
overlaps become hard loops.

Natural loops warm their incoming PCM window two seconds before the overlap. If
that window is still decoding, outgoing audio continues while the overlap
shortens to the remaining tail instead of emitting silence.

Changing a region or repeat policy cancels overlap at the destination of a
seek, or at the outgoing position of a natural loop. A committed beginning
beyond the playhead crossfades into the beginning; a committed end before it
resolves to completion or the configured loop-back.

## Seeking

`Seek` repositions directly for initialization. `CrossfadeSeek` implements
[seek crossfades](../playback/seek-crossfades.md): it caps targets two seconds
before the region end (or at the beginning for shorter regions) and blends the
current outgoing cursor with preroll ending at the destination over up to two
seconds. Targets in the first two seconds start the incoming cursor at the
region beginning. Fade duration is limited by the outgoing tail and the region
length.

`State` reports the advancing destination during seek overlap. A repeated seek
adopts the previous incoming cursor once the fade has begun. While destination
PCM is unavailable, the outgoing audio continues and the fade waits. Paused
seeks reposition directly within the same seekable bounds.

## Envelopes and gain

Repeating play/pause transitions use independent two-second linear envelopes.
Gain changes while fully silent apply directly, so the first audible frame uses
the configured volume rather than ramping down from full volume. Pause cancels
loop overlap, lets the outgoing cursor advance while fading, then freezes it;
reaching the region end during fade-out stops and resets it rather than
starting another loop. Non-repeating play/pause is immediate.

`SetGain` takes an effective track/master/type product in `[0, 1]` and ramps it
over two seconds while audible. Interrupted ramps start at the current
interpolated gain. Duplicate play/pause, gain, repeat, and region settings do
not restart ramps or loops.

Tracks are summed in insertion order and only the final stereo output is
saturated to `[-1, 1]`; there is no normalization or peak limiter.

## Testing

Tests use small known PCM arrays and exact sample positions to assert audible
output, completion, crossfade boundaries, interruption, and silence. Rendering
the same command timeline with single, irregular, or whole buffers produces the
same samples and track state, including empty renders. File decoding, source
buffering, playback-state adapters, real-time scheduling, encoding, and
external audio delivery are outside this package.
