---
type: Workflow
title: "08: Playback envelopes"
description: Fade repeating play and pause independently of track gain.
resource: internal/mixer/mixer.go
tags: [tutorials, audio, envelopes]
---

# 08: Playback envelopes

## Learn

Requested playback state and current audibility are different. A paused track
can still emit its fading tail. Playback envelopes must be independent of
volume ramps or a volume adjustment can accidentally cancel a pause.

## Starting point

Use the [workbench](./pcm-workbench.md). Independently, build a one-track
cursor renderer with play/pause, a repeating flag, and the sample-counted
retargetable ramp specified in [lesson 07](./07-gain.md). A 30-second tone
avoids needing loop overlap for this exercise.

## Build

Give each track an envelope and a gain. Output is
`sourceSample * gain * envelope`, before the final mix clamp.
Repeating tracks retarget the envelope to 1 on play and 0 on pause over two
seconds. Non-repeating tracks change envelope immediately.

Set `Playing` to the requested state immediately. During pause's fade-out,
keep reading/advancing the outgoing cursor. Once the envelope is zero, emit
silence and freeze the cursor. If the end is reached during a pause fade,
stop/reset rather than looping. Resume from the current envelope value.

Use the long tone with repeat enabled: play for three seconds; pause for
three; resume for one; pause for one; resume for three. During the final
resume, change gain to 0.25 and confirm both ramps run independently.
Also record a short non-repeating version to compare immediate pauses.

## Listen

```sh
go run . lab08.wav
ffplay -nodisp -autoexit -loglevel error -volume 20 lab08.wav
```

Hear a two-second fade-in, a two-second fade-out followed by silence,
then interrupted fades without a jump back to zero or unity.
The non-repeating comparison cuts immediately; that distinction is intentional.

## Verify

At rate 2, a fresh repeating play returns envelope values
`0, 0.25, 0.5, 0.75`, then `1`.
Right after pause, state says `Playing:false` even though samples remain
audible. After four fading frames the position freezes on later renders.
Duplicate play/pause commands must not restart ramps.

A gain ramp still steps while fully paused; an envelope only steps along
the rendering path that actually processes available audio. Later source
stall labs make this distinction observable.
Reference `track.setPlaying`, `track.next`, and `ramp`.
Next, smooth the [loop boundary](./09-loop-crossfades.md).
