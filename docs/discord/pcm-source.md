---
type: Component
title: PCM source
description: Lazy, windowed FFmpeg decoding of stored HLS for the live mixer.
resource: internal/hlsaudio
tags: [discord, audio, ffmpeg, hls]
---

# PCM source

The voice service's `Clips` loader supplies tracks to the
[live mixer](./live-mixer.md). It checks the track's association with the
table, loads track/type metadata through the datastore, and opens the track with
`internal/hlsaudio`, which uses HLS segment durations from
[track uploads](../data/track-uploads.md) to determine the indexed PCM length.

## Windowed decoding

FFmpeg decodes audio lazily in five-second windows at 48 kHz stereo float PCM.
Each temporary window playlist references only the stored HLS segments needed
for that range, so seeking does not decode the entire track from its beginning.
Temporary manifests contain paths only and are removed after decoding; decoded
audio is not written to disk.

Each decode includes a 100 ms margin for AAC timestamp and frame rounding, then
clips or zero-pads to the indexed window length; larger shortfalls fail.
Crossfades and seeks access indexed windows, including loop-back windows.
Decoded samples are clamped to unity to accommodate AAC decoding overshoot.

## Caching

The source keeps a four-window least-recently-used cache, enough for both
crossfading cursors and their lookahead windows, and requests the next window
one second before playback reaches a boundary. This bounds PCM memory to
roughly 11 MiB of cached frames plus one in-flight decode window per active
track, regardless of track duration.

## Failures and stalls

FFmpeg failures are exposed on the corresponding live-mixer track status. If a
requested window is still decoding, that track temporarily renders silence
without advancing its playback cursor, so it resumes at the correct audio
rather than skipping frames.
