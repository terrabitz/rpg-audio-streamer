---
type: Workflow
title: "06: Intro and hard loop"
description: Repeat a region from its loop-back point without replaying its intro.
resource: internal/mixer/mixer.go
tags: [tutorials, audio, loops]
---

# 06: Intro and hard loop

## Learn

The beginning of a playback region and its loop-back position serve different
purposes. The first pass may include an intro; later passes return to
`LoopBack`, not `Start`.

## Starting point

Use the [workbench](./pcm-workbench.md). A minimal independent engine needs
a cursor, playing flag, validated region, and a `Render` that emits that
region and otherwise stops/resets. It does not need gains or transitions.

## Build

Add a repeating flag and `SetRepeating`. When a playing track reaches the
exclusive end, choose between stopping/resetting to start and continuing
from loop-back. Preserve the final frame before the jump.

For the marker clip, set region `[0, 12)` and loop-back 3 seconds. Play with
repeat enabled and render 24 seconds. Then disable repeat and continue
rendering 12 more seconds. Keep all earlier output in the same WAV.

Hard wrapping is an intentionally incomplete transition, not the final
production algorithm. You will keep it as the zero-overlap fallback in
[lesson 09](./09-loop-crossfades.md).

## Listen

```sh
go run . lab06.wav
ffplay -nodisp -autoexit -loglevel error -volume 20 lab06.wav
```

In the first 24 seconds, hear low-left only once, center-mid at output seconds
3, 12, and 21, and high-right between those sections. After repeat is
disabled, the current pass finishes and the recording becomes silent.
Use non-matching phases in a second fixture if you want to expose the hard
wrap's click; do not assume every integer-cycle tone boundary will click.

## Verify

At a tiny rate, region `{Start:1, End:5, LoopBack:3}` must visit
`1,2,3,4,3,4,3,4...`, not `1,2,3,4,1...`.
Disable repeat partway through and confirm stopped state at 1 on completion.
Initialize at `End`; one nonempty render must resolve to loop-back when
playing/repeating, or stop/reset otherwise.

No recursion should be necessary to handle an end boundary.
Production references are `SetRepeating` and `track.finish`.
Next, smooth volume with [gain ramps](./07-gain.md).
