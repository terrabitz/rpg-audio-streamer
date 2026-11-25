---
type: Tool
title: PCM lab workbench
description: Set up an independent Go project with synthetic fixtures and workstation audio playback.
tags: [tutorials, audio, go]
---

# PCM lab workbench

This is the shared equipment for the [mixer course](./pcm-mixer-course.md),
not a mixer implementation. The tutorials intentionally contain lab procedures;
the rest of the documentation wiki remains a description of the application.

## Create an isolated project

Install Go and FFmpeg using your workstation's package manager if absent.
`ffmpeg -version`, `ffplay -version`, and `go version` must work in the
terminal where you do the labs. The standard library is the only Go dependency.
The repository's Go version works; the workbench requires Go 1.22 or later.

Use a new empty directory outside this repository for each independent attempt:

```sh
mkdir pcm-lab
cd pcm-lab
go mod init example.com/pcm-lab
```

Create `main.go` with the code below using your editor. Keep the helper functions
unchanged. Add your implementation in `engine.go` in the same `package main`,
and replace `renderLab` for each exercise. Do not import `internal/mixer`:
that would play the solution rather than your reconstruction.

```go
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

const sampleRate = 48000

type Frame struct {
	Left, Right float64
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run . output.wav")
		os.Exit(1)
	}
	frames, err := renderLab()
	if err == nil {
		err = writeWAV(os.Args[1], frames)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func renderLab() ([]Frame, error) {
	return tone(2, 440, 0.1, 0.1), nil
}

func tone(seconds, hz, left, right float64) []Frame {
	frames := make([]Frame, int(seconds*sampleRate))
	for i := range frames {
		value := math.Sin(2 * math.Pi * hz * float64(i) / sampleRate)
		frames[i] = Frame{
			Left:  left * value,
			Right: right * value,
		}
	}
	return frames
}

func silence(seconds float64) []Frame {
	return make([]Frame, int(seconds*sampleRate))
}

func join(parts ...[]Frame) []Frame {
	var frames []Frame
	for _, part := range parts {
		frames = append(frames, part...)
	}
	return frames
}

func writeWAV(path string, frames []Frame) error {
	if uint64(len(frames)) > (uint64(math.MaxUint32)-36)/4 {
		return fmt.Errorf("recording exceeds WAV size limit")
	}
	for i, frame := range frames {
		for _, sample := range []float64{frame.Left, frame.Right} {
			if math.IsNaN(sample) || math.IsInf(sample, 0) || sample < -1 || sample > 1 {
				return fmt.Errorf("invalid output sample at frame %d: %v", i, sample)
			}
		}
	}
	dataSize := uint32(len(frames)) * 4
	header := struct {
		RIFF          [4]byte
		Size          uint32
		WAVE          [4]byte
		Fmt           [4]byte
		FmtSize       uint32
		Format        uint16
		Channels      uint16
		Rate          uint32
		ByteRate      uint32
		BlockAlign    uint16
		BitsPerSample uint16
		Data          [4]byte
		DataSize      uint32
	}{
		RIFF:          [4]byte{'R', 'I', 'F', 'F'},
		Size:          36 + dataSize,
		WAVE:          [4]byte{'W', 'A', 'V', 'E'},
		Fmt:           [4]byte{'f', 'm', 't', ' '},
		FmtSize:       16,
		Format:        1,
		Channels:      2,
		Rate:          sampleRate,
		ByteRate:      sampleRate * 4,
		BlockAlign:    4,
		BitsPerSample: 16,
		Data:          [4]byte{'d', 'a', 't', 'a'},
		DataSize:      dataSize,
	}
	var data bytes.Buffer
	if err := binary.Write(&data, binary.LittleEndian, header); err != nil {
		return err
	}
	for _, frame := range frames {
		for _, sample := range []float64{frame.Left, frame.Right} {
			value := int16(math.Round(sample * 32767))
			if err := binary.Write(&data, binary.LittleEndian, value); err != nil {
				return err
			}
		}
	}
	return os.WriteFile(path, data.Bytes(), 0o600)
}
```

The WAV writer rejects invalid or unclipped output rather than hiding mixer
bugs. It converts the final float samples to 16-bit integers for convenient
workstation playback. The core itself stays float64; production's different
float32 transport encoding is your [lesson 14](./14-encoding.md).

## First sound and troubleshooting

Lower the workstation's output volume, then run:

```sh
go run . sanity.wav
ffplay -nodisp -autoexit -loglevel error -volume 20 sanity.wav
```

You should hear a quiet, centered A tone for two seconds. If ffplay has no local
audio device (common over SSH or in containers), copy the WAV to your
workstation and open it in a normal media player. Generating a WAV remotely
does not make it audible locally. Use speakers or stereo headphones at a
comfortable level; disable mono output and spatial effects for stereo labs.

If silent, play an unrelated known-good local audio file, select the correct
output device, and unmute the player. If the pitch or speed is wrong, check
sample rate before changing your engine. Do not increase gain to diagnose
silence. Tone fixture boundaries may click; later crossfade labs address
internal transitions, not every abrupt start of a WAV.

Inspect any result without an audio editor:

```sh
ffprobe -v error -show_entries stream=codec_name,sample_rate,channels -show_entries format=duration -of default=noprint_wrappers=1 sanity.wav
```

Expect `pcm_s16le`, `48000`, `2`, and a two-second duration.

## Fixture vocabulary

These are instructions for constructing inputs, not solved mixer algorithms:

| Name used in lessons | Construction |
| --- | --- |
| A | `tone(12, 220, 0.12, 0)`; low tone on the left |
| B | `tone(12, 660, 0, 0.12)`; high tone on the right |
| Marker clip | `join(tone(3, 220, 0.12, 0), tone(3, 440, 0.12, 0.12), tone(6, 660, 0, 0.12))` |
| Long marker | Twenty seconds: 220 Hz left for ten seconds, then 660 Hz right for ten seconds |
| Bed | `tone(30, 110, 0.04, 0.04)`; quiet continuous reference |
| Constant frames | Allocate a slice and fill each element with the specified `Frame`; numerical checks only, not a listening fixture |

The marker clip has landmarks at absolute seconds 0, 3, 6, and 12. Times in
scenario instructions are **output time** unless explicitly called a source
position. Convert seconds to frames with `int(seconds * sampleRate)`; never
multiply by the channel count. One frame contains both channels.

## Drive a timeline

Implement a helper in your own code which allocates `n` output frames, calls
your engine's `Render`, and appends the block to a recording. Controls happen
between render calls. For a command at output second 3, render exactly
`3 * sampleRate` frames first. Do not use `time.Sleep` for offline scenarios.
An input can be longer than the output or vice versa.

Use the production API names where convenient: `New`, `Add`, `Render`,
`Play`, `Pause`, `Seek`, `State`, `SetRegion`, `SetRepeating`, `SetGain`,
`CrossfadeSeek`, `AddSource`, `TrackError`, `Remove`, `DetachSource`, `Close`.
Fallible controls should return errors, and `renderLab` should propagate
unexpected errors instead of ignoring them. Tests should explicitly expect
errors for deliberately invalid commands.

Use a tiny engine rate, such as 2 or 6 frames per second, for exact
sample-by-sample checks with constant frames. Do not play those arrays at
48 kHz and call that an audible check; construct separate 48 kHz recordings
from the tone fixtures.

For each standalone lab, start with this workbench and its page's minimal
starting point. For the cumulative path, retain the engine and change only
the scenario. Start with [samples become sound](./01-samples.md).
