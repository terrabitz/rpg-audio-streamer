---
type: Workflow
title: "16: Windowed decoding"
description: Feed indexed PCM from a bounded asynchronous HLS decoder cache.
resource: internal/server/windowed_pcm.go
tags: [tutorials, audio, decoding]
---

# 16: Windowed decoding

## Learn

Long tracks should not require decoding the entire file into memory.
An indexed source can decode only the windows requested by the core.
This is an adapter around the engine, not another branch inside `Render`.

## Starting point

Use the [workbench](./pcm-workbench.md), Go's `os/exec` and `context`,
and the `Source` contract in [lesson 12](./12-sources.md).
An independent slice needs only a source-backed renderer that emits silence
and holds its cursor when unavailable. It can use direct seeks instead of
crossfades. The production-parity extension also exercises two cursors.

## Make the local fixture

First temporarily replace `renderLab` with a fixture builder: join eight
five-second tones at frequencies 220, 330, 440, 550, 660, 770, 880, and 990 Hz.
Put odd-numbered sections in the left channel and even-numbered sections in
the right, all at amplitude 0.1. Return that 40-second slice. Then run:

```sh
mkdir -p fixture
go run . fixture/source.wav
ffmpeg -nostdin -y -v error -i fixture/source.wav -c:a aac -b:a 192k -hls_time 2 -hls_list_size 0 -hls_segment_filename fixture/segment-%03d.ts -f hls fixture/index.m3u8
```

All following paths are inside this lab directory. HLS/AAC durations may
include codec padding; use manifest durations for indexed length rather
than assuming the encoded duration is exactly 40 seconds.
Restore `renderLab` as the decoder experiment before producing `lab16.wav`.

## Build

Read `#EXTINF` durations and media paths from this local media playlist.
Resolve paths against the playlist directory and make them absolute for use
in temporary playlists outside that directory. Validate finite positive
durations, complete duration/path pairs, and nonempty segments. Indexed
length is `round(sum(segmentDurations) * rate)`.

Divide source positions into five-second windows. Keep at most four decoded
windows in an LRU cache. `FrameAt` returns immediately; on a miss, enqueue
the window unless already pending/cached. Use a bounded queue (production
capacity 16); if full, clear that request's pending marker so a later lookup
can retry. A worker decodes off the render path.

For each window, construct a temporary media playlist starting with the
segment containing its start. Include enough segments for the intra-segment
offset, requested count, and a 100 ms margin. Use the original segment
durations, correct target duration, and corresponding media sequence.
Decode with FFmpeg using an argument slice, not shell interpolation:

```text
ffmpeg -nostdin -v error -i WINDOW_PLAYLIST -ss OFFSET_SECONDS
       -t WINDOW_SECONDS_PLUS_0.1 -vn -ac 2 -ar 48000 -f f32le -
```

`-ss` is after `-i`; the offset is relative to that temporary playlist's
first segment, not the original track. Decode stdout in memory as eight
bytes per frame using your [encoding knowledge](./14-encoding.md).
Reject empty output, partial frames, non-finite samples, and shortfalls
larger than `ceil(0.1*rate)` frames. Trim extra frames or zero-pad a smaller
shortfall to the exact indexed window length. Clamp finite AAC overshoot
to unity. Bound stdout/stderr capture so a failed decoder cannot grow memory
without limit; production allows requested bytes plus one second on stdout
and 64 KiB of stderr.

Request the next window during the last second of the current window.
The core's loop prewarming from lesson 12 complements this source lookahead.
Expose worker failures through `Err`. Cancel and join the worker on `Close`,
cancel its FFmpeg process, and remove temporary manifests even on failure.
Do not write decoded PCM windows to disk.

## Listen

In the scenario, warm window 0 off the render path, render source 0-7 seconds,
then directly seek to source 27 and render five seconds. Wait for requested
windows only in your offline driver, never inside `Render` or `FrameAt`;
keep those waits out of the recorded output timeline for this comparison.
Before rendering each block, request and await the windows it needs, and split
blocks at window boundaries. Do not render unready output and then discard it:
that would still step the engine's gain ramps.
Use bounded waits which fail with context instead of hanging forever.

```sh
go run . lab16.wav
ffplay -nodisp -autoexit -loglevel error -volume 20 lab16.wav
```

Hear 220 Hz left for five seconds, 330 Hz right for two, then 770 Hz right
for about three and 880 Hz left for two. Codec padding may shift landmarks
slightly; replaying the opening after the seek is a decoder-offset bug.
For a separate stall recording, omit driver warming: verify the source
waits without skipping unheard audio.

## Verify

Log cache hits, misses, decoded ranges, and evictions. Touch at least six
distinct windows and assert the cache never exceeds four. At 48 kHz,
four full windows contain 960,000 `Frame` values; account separately for
one in-flight decode and temporary byte buffers.
Check the source offset and exact frame count of a window beginning
mid-segment. Test a malformed playlist, a missing segment, decoder failure,
queue saturation, cancellation, and a short final window.

For two-cursor parity, run a loop whose tail and loop-back occupy distant
windows, then a seek to another distant window. Verify prewarming and LRU
reuse, rather than decoding the full source from its beginning.
References are `readHLSPlaylist`, `windowPlaylist`, `decodeWindow`,
`requestWindow`, and `storeWindow` in `internal/server/windowed_pcm.go`.
The app's [PCM source](../discord/pcm-source.md) additionally checks table
association and loads track metadata before creating this adapter.

Next, drive the core with [live commands](./17-live-commands.md).
