---
type: Workflow
title: "03: Sum and saturate"
description: Mix independent tracks and clamp only the final stereo sum.
resource: internal/mixer/mixer.go
tags: [tutorials, audio, mixing]
---

# 03: Sum and saturate

## Learn

Mixing adds simultaneous amplitudes channel by channel. It is not
concatenation, averaging, or selecting the loudest track. Saturation prevents
the output from exceeding unity, but does not prevent distortion.

## Starting point

Use the [workbench](./pcm-workbench.md). Implement the small renderer from
[lesson 02](./02-rendering.md): a slice and persistent cursor per source,
silence after completion. Store sources in insertion order. Commands,
repeating, and gains can be absent.

## Build

Extend rendering to sum all active tracks into a cleared destination.
Clamp each final left and right sample to `[-1, 1]` only after summing every
track. Keep each track's cursor independent.

Make a six-second recording: two seconds of A alone, two of A plus B, two of
B alone. Both fixtures are long enough; advance only active tracks.

For a second, quiet comparison, mix two identical centered 220 Hz signals at
amplitude 0.7 to force clipping. Lower the **already clipped result** by 0.1
before adding it to a separate recording; compare with one clean 220 Hz tone
at amplitude 0.1. Do not play the full-amplitude clipped version.

## Listen

```sh
go run . lab03.wav
ffplay -nodisp -autoexit -loglevel error -volume 20 lab03.wav
```

Hear low-left, both pitches in opposite channels, then high-right. The
separate attenuated clipping comparison should sound buzzier than the clean
tone. The WAV writer must not be doing your saturation for you.

## Verify

For one frame, insert tracks with left values `0.8`, `0.7`, and `-0.6`.
The result must be approximately `0.9`, not `0.4` (which reveals per-track
clipping of the accumulating sum). A final sum of `1.4` must become `1`.
Use an absolute tolerance such as `1e-9` for float64 checks.

Rendering an empty mix over a dirty buffer must produce zeros.
The production reference is `Mixer.Render`. Next, add
[track transport](./04-transport.md).
