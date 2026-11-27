---
type: Workflow
title: "15: Deterministic reconstruction"
description: Verify the full core against a command timeline regardless of render partitioning.
resource: internal/mixer/mixer_test.go
tags: [tutorials, audio, determinism]
---

# 15: Deterministic reconstruction

## Learn

A correct mixer is a deterministic state machine over frames and commands.
The same timeline must not change because a device asks for one frame,
960 frames, or an irregular number of frames.

## Starting point

Use the [workbench](./pcm-workbench.md). The full-core checkpoint expects
the interfaces and rules in lessons 01-14, not any saved recordings.
For an independent first pass, implement just a cursor renderer with a
four-frame gain ramp and prove the same partition property; then expand it
to the explicit full-core scenario below. A passing small slice alone is
not the course's core-completion criterion.

## Build

Construct a fresh engine for each run. Add the long marker with region
`[2,18)` seconds, loop-back 6, repeat on, and initial gain 0.5.
Add a 30-second bed. Execute this output-time script:

| Output second | Command |
| --- | --- |
| 0 | Play both |
| 3 | Set marker gain to 0.2 |
| 4 | Set marker gain to 0.8 |
| 6 | Crossfade-seek marker to source 14 |
| 7 | Crossfade-seek marker to source 3 |
| 10 | Pause marker |
| 11 | Resume marker |
| 14 | Commit marker region `[4,16)`, loop-back 6 |
| 17 | Disable marker repeating |
| 25 | End recording |

Render it three ways: one block **per interval between commands**, individual
frames, and repeating block sizes `1,3,7,2,11,960,127`.
Always split at command times even if the next requested buffer is larger.
Insert empty renders throughout one run. Never apply a command at a nearby
block boundary instead of its specified frame.

Return the first recording as `lab15.wav`. Compare all samples and final
states before writing it. Report mismatches as errors with frame/channel
context; do not quietly write a successful-looking output.

## Listen

```sh
go run . lab15.wav
ffplay -nodisp -autoexit -loglevel error -volume 20 lab15.wav
```

Hear gain changes, a left-to-right seek, its interrupted return, pause/resume,
and a final stop over the continuing bed. Listen to the other two render
variants as separate files if the numerical comparison fails.

## Verify

Use `1e-9` float64 tolerance, equal final state/error values, and identical
encoded float32 bytes when the values are bit-identical.
Render a null signal `variantA - variantB`: it must contain zero samples
for an exact implementation, not merely sound quiet. This diagnostic is
additional to, not a substitute for, the audible main scenario.

Exercise source unavailability with readiness toggles at fixed output frames
as another timeline; do not base a deterministic test on worker sleep timing.
Repeat the small-rate boundary checks from the earlier pages, including invalid
inputs, duplicate commands, source errors, and ownership transfer.
Check that the engine creates no clocks, devices, goroutines, or I/O.

Only after your own checks pass, compare with `internal/mixer/mixer_test.go`,
especially `TestRenderPartitionInvariant`. Passing invariance proves
consistency, not correct seek/loop semantics; both the earlier known-value
checks and this checkpoint are required.

You have now reconstructed the standalone core. Continue with
[windowed decoding](./16-windowed-decoding.md) to understand the sources
that feed it in the application.
