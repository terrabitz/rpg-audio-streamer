---
type: Workflow
title: "02: Pull-based rendering"
description: Advance a persistent playback cursor through caller-owned buffers.
resource: internal/mixer/mixer.go
tags: [tutorials, audio, rendering]
---

# 02: Pull-based rendering

## Learn

`Render(dst)` means "give me the next `len(dst)` frames," not "play this for
some wall-clock duration." The caller owns the buffer. A cursor must survive
between calls, and an empty render must not advance anything.

## Starting point

Use the [workbench](./pcm-workbench.md). Independently, build a one-track
struct containing a `[]Frame`, a cursor, and a playing flag. It can start
playing immediately for this lesson. No track IDs or commands are needed.

## Build

Implement `Render([]Frame)` to overwrite every destination frame. Copy the
next source frame while playing; leave silence after exhaustion. Do not leave
data from the caller's previous render in the buffer. On completion, stop and
reset the cursor to zero, but still output the last valid source frame.

Render the 12-second marker clip into a 14-second recording using repeated
960-frame blocks. Reuse a buffer whose contents you deliberately fill with
nonzero values between calls. Include an empty render at each boundary.

Do not allocate or start a player inside the engine. A renderer with no clock
is easier to reason about and can serve either files or real-time devices.

## Listen

```sh
go run . lab02.wav
ffplay -nodisp -autoexit -loglevel error -volume 20 lab02.wav
```

Hear three seconds left-low, three seconds center-mid, six seconds right-high,
then two seconds of silence. There must be no repeated first 20 ms or
stale final tone in the silent tail.

## Verify

At a tiny test rate, render a two-frame clip in three one-frame calls.
The third output must be zero; completion reports stopped at position zero.
Also render the entire scenario in one call and compare every frame.

Record how long calculation takes, then compare that with the WAV duration.
They need not match: the player, not the renderer, determines listening speed.
Production references are `Render`, `track.next`, and `track.finish`.
Next, [sum tracks](./03-summation.md).
