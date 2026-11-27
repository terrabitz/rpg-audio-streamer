---
type: Workflow
title: "07: Gain ramps"
description: Retarget effective volume with sample-counted linear ramps.
resource: internal/mixer/mixer.go
tags: [tutorials, audio, gain]
---

# 07: Gain ramps

## Learn

A gain multiplies samples before summation. A sudden change can introduce
a discontinuity; a ramp spreads that change across samples. The ramp's clock
is rendered frames, not render calls.

## Starting point

Use the [workbench](./pcm-workbench.md). For an independent lab, implement
a non-repeating one-track renderer with play/pause and a long centered
220 Hz tone at amplitude 0.12. No regions or overlaps are necessary.

## Build

Implement a ramp with `from`, `to`, `elapsed`, and `total`.
Before stepping, its value is
`from + (to-from)*elapsed/total` while `elapsed < total`, otherwise `to`.
A step returns the current value and then increments elapsed toward total.
Retarget from the current interpolated value, not the previous target.

Add `SetGain(id, gain)` accepting only finite `[0, 1]` values. Initialize at
unity. While audible, retarget over `2 * sampleRate` frames.
While fully paused and silent, set gain immediately. A repeated target
does not restart a ramp. Advance the gain ramp even while the track is paused.

Set gain to 0.25 before playing. Render two seconds; set it to 1 and render
one; retarget to 0 and render three; set to 0.5 and render three. Keep the
track playing throughout these nine seconds.

Effective gain is already combined when it reaches the core. For example,
track `0.8`, master `0.5`, and type `0.6` give `0.24`. Do not add a separate
master volume to every frame if the caller has already included it.

## Listen

```sh
go run . lab07.wav
ffplay -nodisp -autoexit -loglevel error -volume 20 lab07.wav
```

Hear a quiet start, a rise interrupted halfway, a smooth fall into silence,
and a two-second rise to half gain. There should be no full-volume first
frame caused by initializing a ramp from unity after playback starts.

## Verify

At rate 2, a four-frame ramp from 1 to 0 returns
`1, 0.75, 0.5, 0.25`, then `0`. Retarget after two steps: the new `from`
is `0.5`. A duplicate target leaves elapsed unchanged.
Check the product before summation with two tracks: clipping first and then
applying gain is not equivalent to applying per-track gain and then clipping.

Reject NaN, infinity, negative gain, and gain above 1 without affecting the
recording. References are `SetGain` and `ramp`.
Next, separate volume from [playback envelopes](./08-envelopes.md).
