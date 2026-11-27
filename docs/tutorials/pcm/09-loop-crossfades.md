---
type: Workflow
title: "09: Loop crossfades"
description: Blend a loop tail with an advancing incoming cursor.
resource: internal/mixer/mixer.go
tags: [tutorials, audio, crossfade]
---

# 09: Loop crossfades

## Learn

A crossfade reads two source positions for one output frame. The incoming
cursor advances while being blended; jumping back to its original position
afterward would replay audio the listener has already heard.

## Starting point

Use the [workbench](./pcm-workbench.md). Independently, make a hard-looping
renderer with `Start`, exclusive `End`, and `LoopBack` as specified in
[lesson 06](./06-hard-loops.md). Unity gain/envelope is sufficient to study
the boundary; retain real envelopes on the cumulative path.

## Build

Calculate overlap length, in frames:
`min(2*rate, (End-LoopBack)/3, End-Start)`, using integer division.
Start overlap when the outgoing cursor reaches `End - overlapLength`.
Initialize an incoming cursor at `LoopBack`.

For progress `p = (outgoing-overlapStart)/(overlapEnd-overlapStart)`,
blend `outgoingSample*(1-p) + incomingSample*p`. Advance both cursors.
On the boundary, adopt the **advanced** incoming cursor. Use complementary
linear gains, not equal-power sine/cosine weights. A zero-frame overlap
uses the hard-loop fallback.

Use the marker clip with region `[0, 12)` and loop-back 3. Render 25 seconds.
Here the overlap is two seconds: output source 10-12 blends the high-right
tail with the center-mid audio at source 3-5. The next frame is source 5,
not source 3. The steady-state output loop period is seven seconds,
not nine, because two seconds of incoming audio are consumed in the blend.

## Listen

```sh
go run . lab09.wav
ffplay -nodisp -autoexit -loglevel error -volume 20 lab09.wav
```

Hear the intro once. At the first loop, high-right gradually gives way to
center-mid rather than abruptly switching. Compare with your hard-loop
recording. A crossfade between different pitches sounds like a temporary
mixture; it cannot guarantee musically beat-aligned looping.

## Verify

With rate 3, region `{Start:1, End:12, LoopBack:3}` has overlap length 3.
It starts at 9 and blends progress `0, 1/3, 2/3`; after those frames the
cursor is 6. The next output must read source frame 6.
Also test the two-second cap on a long region and a one-frame loop span,
whose overlap rounds to zero.

Reference the overlap branch in `track.next` and `track.finish`.
Next, reuse two cursors for [seek transitions](./10-seek-crossfades.md).
