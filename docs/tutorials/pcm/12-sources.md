---
type: Workflow
title: "12: Nonblocking sources"
description: Stall one track on unavailable PCM without blocking the mix.
resource: internal/mixer/mixer.go
tags: [tutorials, audio, buffering]
---

# 12: Nonblocking sources

## Learn

Unavailable is not silent. A valid zero sample consumes source time; an
unavailable frame must not. The rest of the mix must continue while one
track waits for a decoder.

## Starting point

Use the [workbench](./pcm-workbench.md). Independently, implement a two-track
cursor renderer with an always-ready bed and one indexed source track.
First complete the no-overlap experiment below; the two-cursor extension
uses the explicitly defined overlap behavior in
[lessons 09](./09-loop-crossfades.md) and [10](./10-seek-crossfades.md).
No FFmpeg worker is required yet.

## Build

Define the production-shaped source contract:

```go
type Source interface {
	Len() int
	FrameAt(int) (Frame, bool)
	Err() error
	Close() error
}
```

Add `AddSource`, leaving the new track paused. Reject nil and empty sources,
empty IDs, and duplicate IDs. Reads must not block rendering.
For unavailable outgoing PCM with no source error, emit zero for that track,
keep its playback cursor/overlap/envelope fixed, and leave requested playing
state true. Gain still advances because it steps before the source read.

Implement a fake indexed source with a switchable `ready` flag. Add and play
both the bed and marker before rendering. With the bed always available,
keep the marker source unavailable for two output seconds,
ready for three, unavailable for two, then ready for four. Toggle availability
between render calls, not with sleeps or wall-clock races.

Then extend readiness to different source ranges. If outgoing audio is ready
but incoming overlap audio is not, keep the outgoing audible and advance it.
Do not advance the unavailable incoming cursor as if it had played.
For a natural loop keep its end fixed and move overlap start forward, shortening
the fade. For a seek move its start forward and shift its end with it, capped
at region end; if the available tail shrinks, advance a non-beginning seek's
incoming origin by the lost fade length. Beginning seeks retain `Start`.
At an actual end boundary, use normal completion/loop resolution.

## Listen

```sh
go run . lab12.wav
ffplay -nodisp -autoexit -loglevel error -volume 20 lab12.wav
```

Hear the bed throughout. The marker first appears low-left at output second
2, disappears at 5, and returns center-mid at 7. It must not skip into
high-right immediately on return. Record a second take seeking from a ready
left range to an unavailable right range: left continues until right is ready.

## Verify

After the first two unavailable seconds, the marker cursor is still zero.
After the second stall it is still at source second 3.
When incoming PCM becomes ready, a loop fade starts at zero incoming weight,
not at a progress calculated from the original missed start.

Prewarm a repeating source's `LoopBack` window starting two seconds before
the natural overlap; readiness queries must not change playback position.
Log requested indices in the fake source to verify this before adding a
decoder. Check late incoming readiness near region end, including shortened
seek fades and beginning seeks. Only a source's terminal `Err` should turn
a buffering stall into a stopped track.

References are `Source`, `AddSource`, `track.frameAt`, and `track.next`.
Next, make [source lifetime and failure](./13-source-lifecycle.md) explicit.
