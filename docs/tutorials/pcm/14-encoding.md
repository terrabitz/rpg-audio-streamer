---
type: Workflow
title: "14: PCM byte encoding"
description: Serialize stereo frames as interleaved little-endian float32 PCM.
resource: internal/mixer/pcm.go
tags: [tutorials, audio, encoding]
---

# 14: PCM byte encoding

## Learn

A Go slice of `Frame{float64,float64}` is not an audio file or a portable
network packet. Production serializes interleaved float32 little-endian PCM:
left, right, left, right. Raw PCM has no header describing its rate or layout.

## Starting point

Use the [workbench](./pcm-workbench.md). For an independent lab, use
`join(tone(2,220,0.12,0), tone(2,660,0,0.12))` as already-rendered frames.
No engine is required to learn the encoding boundary. On the cumulative
path, render this same four-second signal through your engine.

## Build

Implement `EncodeFloat32([]Frame) []byte` without copying the in-memory
representation of a Go struct. Convert each float64 sample to float32, obtain
its IEEE-754 bits with `math.Float32bits`, and write four little-endian bytes
per channel using `encoding/binary`.

In `renderLab`, write those bytes to `lab14.f32` with `os.WriteFile`,
propagating any error. Also return the frames to the normal WAV writer.
There are eight bytes per stereo frame. The encoder converts; it does not
resample, add a WAV header, normalize, or replace the mixer's final clamp.

## Listen

```sh
go run . lab14.wav
ffplay -nodisp -autoexit -loglevel error -volume 20 lab14.wav
ffplay -nodisp -autoexit -loglevel error -volume 20 -f f32le -ar 48000 -ch_layout stereo lab14.f32
```

Both versions must sound like the same four-second left-low then right-high
signal. Small quantization differences are expected: the WAV helper writes
16-bit integers and the raw transport writes float32.
Never diagnose a guessed raw format by playing arbitrary bytes loudly.

## Verify

Four seconds require `4 * 48000 * 8 = 1536000` raw bytes.
Encoding `{Left:0.5, Right:-0.5}` gives hexadecimal
`00 00 00 3f 00 00 00 bf`. Decode your bytes back and compare with
`float64(float32(original))`, not the original float64 at `1e-9` tolerance.
An empty input produces an empty byte sequence.

If playback has the wrong speed or channel order, fix the declared raw format
or interleaving instead of changing the renderer.
Reference `internal/mixer/pcm.go`.
Next, prove [deterministic reconstruction](./15-determinism.md).
