---
type: Workflow
title: "11: Interrupted transitions"
description: Resolve overlapping commands without restarting or reviving stale transitions.
resource: internal/mixer/mixer.go
tags: [tutorials, audio, state-machine]
---

# 11: Interrupted transitions

## Learn

Transitions need explicit ownership and cancellation rules. "Start another
fade" is not sufficient when a previous seek, pause, or loop is in progress.
Duplicate settings are not new transitions.

## Starting point

Use the [workbench](./pcm-workbench.md). This is a transition-state lab:
implement a two-cursor renderer with direct and crossfade seeks and an
inspectable overlap record. Use the formulas and bounds on
[lesson 10](./10-seek-crossfades.md) as the complete minimal prerequisite.
A standalone attempt can use immediate play/pause; add envelopes and natural
loops for the full acceptance matrix.

## Build

Record whether an overlap is a seek or a natural loop. Implement these rules:

| Command | Existing transition and resulting behavior |
| --- | --- |
| Direct `Seek` | Cancel overlap; set the validated absolute position |
| Another `CrossfadeSeek` | If a seek has actually advanced, adopt its incoming cursor; otherwise retain outgoing; replace overlap |
| `Pause` | Adopt a seek's incoming cursor; cancel any overlap; fade only the remaining outgoing path when repeating |
| Changed region or repeat setting | Adopt a seek's incoming cursor, or keep a natural loop's outgoing cursor; cancel overlap |
| Duplicate play/pause, gain target, region, repeat setting | Leave existing ramps and overlaps untouched |

After a region edit, if the playhead is below the new start, a playing track
crossfades into that start; a paused one moves there immediately. At or beyond
the new end, a playing cursor is set to end so the next render resolves
completion/loop-back; a paused cursor moves to start. Repeat policy changes
do not independently restart the playback envelope.

Make three separate four-second takes from the long marker, then concatenate
with half-second silence separators:

1. Start at source 2, seek to 14, render one second, seek to 4, render three.
2. Start at source 2, seek to 14, render one second, direct seek to 14, render three.
3. Start at source 2, commit region `[10,20)` with loop-back 10, render four.

## Listen

```sh
go run . lab11.wav
ffplay -nodisp -autoexit -loglevel error -volume 20 lab11.wav
```

Take 1 turns back toward low-left without replaying the old fade.
Take 2 immediately becomes high-right after its first second.
Take 3 blends toward high-right instead of reading below the committed start
forever. Direct seeking may click: compare behavior, not just smoothness.

## Verify

In take 1, just before the second seek, the previous incoming position is 13,
not outgoing position 3. That is the new outgoing origin.
Issue the second seek before any frame has rendered as a separate case:
it must retain the original outgoing origin.

Midway through a natural loop overlap, resend the identical region and repeat
settings; compare output with a take receiving no duplicates. It must match.
Also move the end behind a playing track with repeat both enabled and
disabled, and pause during loop overlap near the end. Confirm that a paused
tail does not start another loop.

References are `CrossfadeSeek`, `SetRegion`, `SetRepeating`, and
`track.setPlaying`. Next, replace arrays with
[nonblocking sources](./12-sources.md).
