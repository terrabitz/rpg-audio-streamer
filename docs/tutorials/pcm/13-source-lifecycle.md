---
type: Workflow
title: "13: Source ownership and failure"
description: Distinguish buffering from failure and transfer source ownership safely.
resource: internal/mixer/mixer.go
tags: [tutorials, audio, lifecycle]
---

# 13: Source ownership and failure

## Learn

An engine owns resources as well as cursors. A snapshot can reuse a decoder,
but two owners closing the same decoder is not safe. Failure is observable
state, not an indefinite buffering spinner.

## Starting point

Use the [workbench](./pcm-workbench.md) and the four-method `Source` contract
on [lesson 12](./12-sources.md). An independent implementation needs a
source-backed track, an always-ready bed, remove/close controls, and an
error slot. No real subprocess is needed: use a fake source that counts
closes and can return injected values/errors.

## Build

When `FrameAt` returns unavailable, inspect `Err`. A nil error means waiting.
A non-nil error stops/resets that track to region start, clears its envelope
and overlap, and is exposed by `TrackError(id)`. The rest of the mix survives.
Validate samples returned as ready, too: NaN, infinity, or values outside
`[-1,1]` cause a terminal track error rather than contaminating summation.

Removing a source track closes its source. Closing the engine closes every
remaining owned source and removes all tracks. Add `DetachSource(id)` for
ownership transfer: clear the old owner's source reference without closing
it. Do not render the detached track; the live adapter uses detach immediately
before disposing of the old engine. Detach is not a reusable playback mode.

Make two recordings with a bed plus source: play three seconds, then inject
temporary unavailability in one take and a terminal error in the other.
Restore readiness after one second and render three more.
For an ownership take, play the source in one engine, transfer it to a
replacement engine initialized at the saved position, close the old engine,
and keep recording. Announce intentional injected failures on stderr.

## Listen

```sh
go run . lab13.wav
ffplay -nodisp -autoexit -loglevel error -volume 20 lab13.wav
```

The buffering take resumes its marker; the failed take stays bed-only.
The transfer take continues its marker instead of dying when the old owner
closes. Separate takes with half-second silence so they are identifiable.

## Verify

Check `TrackError` identifies the injected error and failed state is stopped
at region start. Check invalid ready PCM takes the same failure path.
The failed source is still owned until remove/close; stopping alone must not
double-close it. The core exposes the error; the adapter is responsible for
reporting it to a user.

A successful transfer causes zero closes at detach/old-close and exactly
one at replacement-close. A removed source is absent when the engine later
closes. Invalid IDs return errors. The core intentionally discards source
close return values during cleanup; do not model those as decoder-read errors.

References are `TrackError`, `track.stopOnSourceError`, `track.frameAt`,
`Remove`, `DetachSource`, and `Close`.
Next, examine the [byte boundary](./14-encoding.md).
