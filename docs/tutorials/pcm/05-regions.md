---
type: Workflow
title: "05: Playback regions"
description: Restrict playback to absolute frame bounds with an exclusive end.
resource: internal/mixer/mixer.go
tags: [tutorials, audio, regions]
---

# 05: Playback regions

## Learn

A region is a view into a clip, not a newly sliced clip. Cursor positions stay
absolute. `Start` is inclusive and `End` is exclusive, so its length is
`End - Start`. This avoids counting the end frame twice.

## Starting point

Use the [workbench](./pcm-workbench.md). Supply a paused-on-add one-track
renderer with `Play`, direct `Seek`, and inspectable position. No repeat,
ramps, or crossfades are needed in this independent slice.

## Build

Define `Region{Start, End, LoopBack int}` and `SetRegion`. Require
`0 <= Start < End <= clipLength` and `Start <= LoopBack < End`.
The default region spans the whole clip with loop-back zero. Failed validation
must not mutate the existing region.

Set the marker clip's region to source seconds `[3, 9)`, with loop-back 3,
while paused. A playhead before the new start moves to the start.
Play and render eight output seconds. Completion resets to region start,
not clip zero. Keep direct seeks inside the inclusive command bounds
`[Start, End]`; the end remains an allowed initialization position.

A paused playhead at or beyond a newly committed end moves to the new start.
Defer edits during audible playback to [lesson 11](./11-interruptions.md),
where you will have the necessary transition machinery.

## Listen

```sh
go run . lab05.wav
ffplay -nodisp -autoexit -loglevel error -volume 20 lab05.wav
```

Hear three seconds center-mid, three seconds high-right, then two seconds
silence. The original three-second low-left opening must never be audible.

## Verify

For a four-frame clip and region `{Start:1, End:3, LoopBack:1}`, render only
source frames 1 and 2, then silence. Final state is stopped at 1.
Reject negative start, empty region, end beyond the clip, and loop-back
equal to end. Seeking outside the region fails even when still inside the
full clip.

Compare with `Region` and `SetRegion` and the
[region domain concept](../playback/playback-regions.md).
Next, introduce an [intro followed by a hard loop](./06-hard-loops.md).
