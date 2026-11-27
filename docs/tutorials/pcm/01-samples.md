---
type: Workflow
title: "01: Samples become sound"
description: Turn normalized stereo frames into an audible signal.
resource: internal/mixer/mixer.go
tags: [tutorials, audio, pcm]
---

# 01: Samples become sound

## Learn

PCM is a sequence of instantaneous amplitudes. A stereo frame has one left
sample and one right sample; at 48,000 frames per second, one second contains
48,000 frames, not 96,000. Zero is silence, not a missing value. The core
accepts finite normalized samples in `[-1, 1]`. It does not resample.

## Starting point

Use the [workbench](./pcm-workbench.md), with no engine at all. You only need
`Frame`, `tone`, `silence`, `join`, and the provided WAV writer.

## Build

Replace `renderLab` with a four-second stereo identification signal:
one second of 220 Hz on the left at amplitude 0.1, one second of silence,
one second of 660 Hz on the right at amplitude 0.1, then one second of
440 Hz in both channels at amplitude 0.05.

Calculate how the sampled sine changes if you change frequency, sample rate,
or amplitude. Add an input check for NaN, infinity, and out-of-range values
in the PCM boundary you will give your engine. A valid sample rate is positive
and small enough that doubling it cannot overflow an `int`.

## Listen

```sh
go run . lab01.wav
ffplay -nodisp -autoexit -loglevel error -volume 20 lab01.wav
```

Hear low-left, silence, high-right, then quieter-center. If left and right
sound centered throughout, check workstation mono settings before changing
the sample layout.

## Verify

The output contains exactly 192,000 frames. During the first second every
right sample is zero; during the silent second both channels are zero.
Peak amplitude never exceeds 0.1. Reject invalid PCM rather than silently
clamping input: final mix saturation is a different responsibility.

Explain why changing only the WAV rate to 24 kHz makes the same sample sequence
last twice as long and sound an octave lower. Restore 48 kHz before proceeding.

The production reference is `Frame`, `New`, and `validSample` in
`internal/mixer/mixer.go`. Next, make a
[pull-based renderer](./02-rendering.md).
