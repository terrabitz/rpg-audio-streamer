---
type: Workflow
title: "04: Track transport"
description: Add paused tracks and control play, pause, direct seek, and removal.
resource: internal/mixer/mixer.go
tags: [tutorials, audio, transport]
---

# 04: Track transport

## Learn

Track state is separate from the source samples and from output time.
Pause holds a source position while the output timeline continues.
Direct seek changes that position without consuming intermediate frames.

## Starting point

Use the [workbench](./pcm-workbench.md) and a one-track cursor renderer
that clears output and stops at clip exhaustion. For this independent slice,
you can implement one ID before generalizing to multiple tracks.

## Build

Add `Add(id, frames)`, `Play`, `Pause`, `Seek`, `State`, and `Remove`.
`Add` copies the input slice and leaves the track paused. Reject empty IDs,
duplicate IDs, empty clips, and invalid samples. Missing-track operations
return errors. Reject out-of-bounds direct seeks but allow position exactly
equal to clip length; rendering resolves completion, not the command itself.

Use the marker clip. Render one second while initially paused; play for two;
pause for one; resume for two; directly seek to source second 8 and render
two; remove the track and render one. The result is nine seconds.
At this stage every play/pause is immediate.

## Listen

```sh
go run . lab04.wav
ffplay -nodisp -autoexit -loglevel error -volume 20 lab04.wav
```

Hear silence, two seconds low-left, silence, one second low-left then one
center-mid, two seconds high-right, then silence. Resuming must not restart
the clip. Removal must not leave a ghost tone.

## Verify

Mutate the caller's original samples after `Add`; playback must be unchanged.
After a pause, several renders must leave the cursor fixed.
An invalid seek must leave both samples and state unchanged.
Seek a paused track to the exact end, confirm that state, then play/render:
it must stop/reset without reading past the slice. An empty render must not
resolve that end position.

The production references are `Add`, `Play`, `Pause`, `Seek`, `State`,
`Remove`, and `find`. Next, constrain transport with
[regions](./05-regions.md).
