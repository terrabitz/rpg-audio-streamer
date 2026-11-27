---
type: Workflow
title: "10: Crossfade seeks"
description: Blend outgoing audio with preroll ending at a seek destination.
resource: internal/mixer/mixer.go
tags: [tutorials, audio, seeking]
---

# 10: Crossfade seeks

## Learn

A seek target is the **end** of the incoming fade, not usually where its
incoming cursor starts. A seek from source 2 to source 14 blends 2-4 with
12-14, then continues at 14.

## Starting point

Use the [workbench](./pcm-workbench.md). Independently, build a one-track
renderer with play/pause, direct seek, validated region, and a two-cursor
linear blend. You can omit natural repeating to isolate seek behavior.

## Build

Add `CrossfadeSeek`. Validate the requested target inside `[Start, End]`,
then cap it to `min(target, max(Start, End-2*rate))`.
Choose fade length `min(2*rate, End-currentPosition, End-Start)`.

When playing with a positive length, start the incoming cursor at
`cappedTarget - fadeLength`. Exception: if the capped target is within the
first two seconds of the region (including exactly two), start at `Start`.
Blend with the same linear progress as a loop; after blending adopt the
advanced incoming cursor. Beginning seeks therefore land after the opening
audio they actually consumed, not necessarily at the selected target.

When paused, or when there is no outgoing tail, reposition directly to
the capped target without creating overlap. `State` reports the incoming
cursor during a seek fade, but the outgoing cursor during a natural loop.
The first may be different from the selected target until the fade completes.

Use the 20-second long marker. Play three seconds; seek to source 14; render
four seconds; seek to source 1; render four more. Use non-repeating playback
to avoid a natural loop interfering with this experiment.

## Listen

```sh
go run . lab10.wav
ffplay -nodisp -autoexit -loglevel error -volume 20 lab10.wav
```

At output seconds 3-5 hear low-left blend into high-right; it then stays
right. At seconds 7-9 hear the opposite blend into the opening low-left.
The second incoming cursor starts at source zero, not source minus one.

## Verify

For the first seek, state starts at source second 12 and reaches 14 after
two seconds. For the beginning seek, it starts at zero and reaches 2.
Request source 19 on a 20-second region: the capped target is 18.
In a region shorter than two seconds, the capped target is always `Start`.

Place the outgoing cursor half a second before the end: the fade shortens
to half a second. Pause and seek: subsequent paused renders stay silent.
Compare with `CrossfadeSeek`, `State`, and
[seek semantics](../playback/seek-crossfades.md).
Next, handle [interrupted transitions](./11-interruptions.md).
