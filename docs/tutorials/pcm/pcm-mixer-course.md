---
type: Overview
title: Build a PCM mixer
description: Recreate the mixer through eighteen incremental listening labs and a production coverage map.
resource: internal/mixer
tags: [tutorials, audio, mixer]
---

# Build a PCM mixer

Build a small Go audio engine yourself, starting with numbers and ending with
the behavior of this project's [PCM mixer](../discord/pcm-mixer.md). Each
lesson adds one conceptual layer and ends with sound you can play on your
workstation. The implementation is your exercise; there is no completed mixer
to copy into the lab.

The provided code is only a signal generator and WAV writer. Neither knows how
to mix, loop, seek, or schedule tracks. Do not use FFmpeg's mixing or fading
filters to implement the exercises: FFmpeg is the decoder and player at the
edges, not the engine you are rebuilding.

## What you need

Know Go slices, structs, methods, errors, and basic goroutines before starting.
You do not need DSP experience. Set aside several sessions: later labs involve
state machines, resource ownership, and concurrency, not just audio arithmetic.

Start with the [workbench](./pcm-workbench.md). It requires Go, FFmpeg/ffplay,
and a workstation audio output, but no running application, database, browser,
Discord account, bot token, or external media. Use low speaker volume first.
All fixtures are synthesized locally.

## How to use a lesson

Follow the numbered lessons in the [tutorial index](./index.md). Keep your
implementation in your own scratch module, outside the application. Do not edit
the production mixer while learning it.

For the cumulative path, keep your engine and replace `renderLab` with each
lesson's scenario. Save each working lesson in version control before moving
on. Keep output filenames distinct so you can compare before and after.

For an independent lab, make a fresh workbench directory and implement only the
small prerequisite subset listed under **Starting point** on that page.
That subset is specified there; you do not need a previous lab's output,
recording, hidden checkpoint, server, or credentials. Earlier lessons are
explanations you can consult, not downloadable solutions. A standalone slice
need not implement unrelated advanced behavior. In the cumulative project,
retain all earlier behavior.

Every lab has a concrete **Build**, **Listen**, and **Verify** contract.
Run the listed command from your lab module after implementing the scenario.
The WAV must come from your implementation. Numerical checks supplement
listening: a click-free recording alone cannot prove cursor positions,
validation, ownership, or nonblocking behavior. Do both before checking off a
lesson. You can use Go's standard `testing` package without any dependencies.

Read the production reference only after a serious attempt. Compare behavior,
not struct layouts; your implementation need not duplicate the original code.
Do not treat intentionally simple intermediate stages (immediate volume,
hard loops) as final production behavior.

## Mental model

```text
stored HLS -> asynchronous indexed source -> PCM mixer -> rendered frames
                                             ^
                              serialized controls and snapshots

rendered frames -> timed publisher -> subscribers -> encoder / device / Discord
```

The core does not make sound by itself. It writes numbers into a slice.
An offline lab can calculate ten seconds of sound in less than ten seconds;
the workstation player supplies the listening clock. Only the final lesson
adds a real-time clock to your own engine.

## Production coverage map

| Layer | Functionality to reproduce | Lessons | Reference after attempting |
| --- | --- | --- | --- |
| PCM contract | Stereo normalized finite samples, fixed sample rate | 01 | `internal/mixer/mixer.go`: `Frame`, `New`, `validSample` |
| Render engine | Clear output, persistent cursor, completion, final-only clipping, insertion order | 02-03 | `Render`, `track.next`, `track.finish` |
| Track API | Copy-on-add, paused initialization, IDs, direct seeks, state, removal, invalid-input errors | 04 | `Add`, `Play`, `Pause`, `Seek`, `State`, `Remove`, `find` |
| Region transport | Exclusive end, absolute offsets, loop-back, initialization at end | 05-06 | `Region`, `SetRegion`, `SetRepeating`, `track.finish` |
| Sample transitions | Two-second retargetable gain and independent repeating envelopes | 07-08 | `SetGain`, `ramp`, `track.setPlaying` |
| Two cursors | Natural-loop overlap, consumed head, seek preroll, bounds, destination state | 09-10 | `overlap`, `CrossfadeSeek`, `track.next` |
| Transition policy | Interrupted seeks, pause, region/repeat edits, idempotent commands | 11 | `SetRegion`, `SetRepeating`, `track.setPlaying`, `CrossfadeSeek` |
| Source protocol | `Len`, `FrameAt`, `Err`, `Close`, stalls, overlap readiness and prewarming | 12 | `Source`, `AddSource`, `track.frameAt`, `track.next` |
| Resource lifecycle | Invalid source PCM, terminal errors, `TrackError`, detach and close | 13 | `TrackError`, `DetachSource`, `Close`, `stopOnSourceError` |
| Byte boundary | Interleaved float32 little-endian encoding | 14 | `internal/mixer/pcm.go`: `EncodeFloat32` |
| Core conformance | Empty renders and identical results across buffer partitions | 15 | `internal/mixer/mixer_test.go` |
| Decoder adapter | HLS indexing, five-second windows, four-window LRU, lookahead and cancellation | 16 | `internal/server/windowed_pcm.go`, `internal/server/mixer.go` |
| Command adapter | Validation before mutation, gain conversion, table isolation, snapshots, source reuse | 17 | `internal/livemixer/service.go`, `internal/livemixer/events.go` |
| Live delivery | 960-frame blocks, catch-up clock, bounded subscriptions, cleanup | 18 | `internal/livemixer/service.go`, `internal/livemixer/stream.go` |

Lessons 01-15 reconstruct the entire standalone core. Lessons 16-18 explain
and rebuild its surrounding [PCM source](../discord/pcm-source.md) and
[live-mixer](../discord/live-mixer.md) responsibilities as local vertical
slices. They are not extra DSP hidden inside `Mixer`.

The live adapter also connects authenticated GM events to table identity,
exclusivity patches, acknowledgments, and lifecycle cleanup. Lesson 17 maps
those integration boundaries; reproducing the HTTP server, browser, or event
bus is not necessary to learn their audio behavior. Discord's Opus encoding,
DAVE transport, and reconnection belong to
[voice streaming](../discord/voice-streaming.md), not the PCM mixer, and are
outside this reconstruction.

## Definition of finished

You can generate and hear every lesson's result, explain each cursor and gain
at a transition boundary, reject malformed commands without corrupting a
running mix, and obtain identical PCM from the same timeline regardless of
buffer size. Your final local demo keeps playing when a source buffers or a
listener stops consuming, reports real failures, and closes its workers.
