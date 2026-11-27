---
type: Workflow
title: "18: Clock and subscribers"
description: Deliver timed PCM blocks without letting slow listeners stall rendering.
resource: internal/livemixer/stream.go
tags: [tutorials, audio, realtime]
---

# 18: Clock and subscribers

## Learn

A mixer has a sample clock; a service must relate it to elapsed wall time.
Counting received ticker messages loses time when the scheduler drops ticks.
Sending synchronously to a slow listener can stall every other listener.

## Starting point

Use the [workbench](./pcm-workbench.md), `context`, `time`, and channels.
For an independent slice, a simple persistent renderer playing 30 seconds
of centered 220 Hz at amplitude 0.1 is sufficient. Add a table-ID map to
associate it with subscribers. No decoder or JSON adapter is necessary.
For the cumulative path, keep your real source and live command adapter.

## Build

At 48 kHz, render 960-frame blocks (20 ms). Give one goroutine serialized
ownership of the core, or use the adapter's PCM lock consistently for
commands and renders; never call a core concurrently.

Maintain a clock's next due time, initialized to start plus 20 ms.
Before that deadline, zero blocks are due. Otherwise calculate
`floor((now-next)/20ms)+1`. Normally advance the deadline by that many
block durations. Cap catch-up at 25 blocks; beyond the cap, schedule the next
deadline at `now + 20ms`. This discards excess clock debt rather than
bursting arbitrarily many blocks. It does not seek the core forward over
frames that were never rendered.

Render only tables with a subscriber. Subscribing observes a mix; it does not
start tracks or change cursors. Give each subscriber a 50-block channel
(one second). Clone each rendered block once before publishing so buffer
reuse cannot change already queued samples; consumers treat blocks as
read-only. Use nonblocking sends. If a channel is full, remove and close
that subscription with an explicit slow-listener error; do not block
rendering or silently drop only its newest block.

Return a cancellation function for each subscription. Closing a table closes
its subscriptions with a session-closed reason. Stopping the service closes
remaining subscriptions with a renderer-stopped reason and closes engines.
Ensure cancellation/removal cannot race into sending on a closed channel.
Queued blocks may still be read after close; an error explains why no more
will arrive.

## Listen

For a bounded eight-second run, add/play the long tone and subscribe a fast
collector. Have it continuously drain blocks into a recording, then return
that recording from `renderLab` after cancellation/join.
At approximately second 2, add a second subscriber that never drains its
channel. Log when it is evicted. Keep the fast subscriber running throughout.

```sh
go run . lab18.wav
ffplay -nodisp -autoexit -loglevel error -volume 20 lab18.wav
```

Generating this file now takes about eight wall-clock seconds, unlike the
offline labs. The fast recording should contain an uninterrupted tone and
last about eight seconds; exact startup/shutdown block counts may vary by
scheduling. It must not freeze or develop a permanent gap when the slow
subscriber fills around second 3.

You can additionally send a fast subscriber's
[float32 encoding](./14-encoding.md) to ffplay's stdin with
`-f f32le -ar 48000 -ch_layout stereo -i pipe:0` for sound during the run.
Own and join that player process and handle pipe-write failures explicitly;
keep it outside the core and do not leave it running after the experiment.
File playback above is the required, device-independent artifact.

## Verify

Using a fresh injected clock for each case, assert exact results: first
deadline gives one block; 100 ms after start gives five; a one-second jump
gives only 25 and places the next deadline 20 ms after the injected time.
A second call at the same time gives zero. Real sleeps are not a precise
scheduling assertion.

Before any subscriber exists, scheduled rendering leaves the table cursor
unchanged. Removing the last subscriber has the same effect. Manual offline
`Render` remains an explicit request and may still advance it.
The slow subscriber is closed on an attempted send to its full channel;
the fast one's queue remains usable. Old queued block values must survive
subsequent buffer reuse.

Cancel a subscription twice safely; close its table; stop the service with
active subscriptions. Confirm reasons and worker completion, not just
silence. The service must not leak workers, subprocesses, or references to
discarded tables.

References are `blockClock`, `Run`, and `renderBlocks` in
`internal/livemixer/service.go`, and `Subscribe`, `publish`, and
`closeStreams` in `internal/livemixer/stream.go`.
Use the [course coverage map](./pcm-mixer-course.md) for a final audit:
the core renders, the source supplies indexed PCM, the adapter translates
commands, and the publisher handles time and backpressure.
