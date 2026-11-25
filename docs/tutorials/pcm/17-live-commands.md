---
type: Workflow
title: "17: Live command adapter"
description: Apply table-scoped snapshots and patches around the PCM core.
resource: internal/livemixer/service.go
tags: [tutorials, audio, commands]
---

# 17: Live command adapter

## Learn

An application speaks seconds, percentages, optional JSON fields, and table
IDs. The core speaks absolute frames and normalized gains. Keep this
translation outside DSP so commands can be validated before mutation.

## Starting point

Use the [workbench](./pcm-workbench.md) and `encoding/json`. For a standalone
slice, build two independent engines supporting add, play/pause, direct seek,
and gain. Synthetic slices are valid loader results; no HLS, server, socket,
or database is required. Add crossfade seeks, regions, and source transfer
from their lesson specifications for full parity.

Provide a loader taking a context, table ID, and file ID, returning either
frames or an indexed source, a track-type name, repeat default, and region
defaults. Exactly one of frames/source must be populated.

## Build

Implement per-table `Sync` (full replacement), `Update` (one partial track
patch), `Volumes`, `Render`, `Status`, `RemoveTrack`, and `CloseTable`.
Use pointer/optional fields for patch values: omitted is not the same as
false or zero. Start with these request shapes:

```json
{
  "tracks": [
    {"fileID": "music", "isPlaying": true, "currentTime": 2, "volume": 80}
  ],
  "typeVolumes": {"Music": 50}
}
```

```json
{"fileID": "music", "volume": 0}
```

The second is a mute update, not an instruction to pause, seek to zero,
change repeat, or restore a default volume. A track patch can also contain
`isRepeating` and
`playback: {"startTime": 0, "endTime": 18, "loopBackTime": 6}`.
Unset region end resolves to decoded length; unset loop-back to start.
Validate seconds before converting with `int(seconds * rate)`, then reject
regions that collapse to fewer than one frame after conversion.

Accept track percentages in `[0,200]` and type percentages in `[0,100]`,
all finite. Missing type volume defaults to 100. Compute
`min(1, trackVolume/100 * typeVolume/100)`. The supplied type map **already
includes the GM master multiplier**; do not apply master twice.

Validate a patch before applying it. Prepare a complete snapshot in a
replacement engine before swapping it in; a malformed or duplicate track
must not destroy the currently playing mix. Explicit empty `tracks: []`
clears a snapshot; an omitted `tracks` field is invalid. Snapshot positions
initialize silently with direct `Seek`, while audible patch seeks use
`CrossfadeSeek`. Apply region, repeat, gain, requested pause, seek, then
requested play, in that order.

Serialize commands. Do expensive loading outside the PCM lock, then apply
briefly under it. A per-table generation changed by `CloseTable` prevents
an in-flight load from reviving a discarded table. Snapshot reuse of a
source must transfer ownership before the old engine closes; preparation
failure closes only newly acquired sources.

Expose revision, rendered frame count, block peak, and sorted track status
with source time, requested playing state, target effective gain, and error.
Do not label target gain as the current interpolated envelope amplitude.
Use cancellation and bounded loading (production's per-load timeout is ten
seconds). Report failed commands explicitly.

## Listen

Use long markers for table `tavern` Music and table `forest` Ambience.
Keep both non-repeating for an uncomplicated gain comparison.
Record tavern and forest separately while applying this output-time script:

| Output second | Action |
| --- | --- |
| 0 | Snapshot tavern at source 2, volume 80, Music type volume 50; snapshot forest at source 12 |
| 3 | Patch tavern volume to zero |
| 6 | Patch tavern volume to 80 and source time to 14 |
| 9 | Submit invalid tavern snapshot containing a duplicate file ID |
| 11 | Snapshot tavern with an empty track list |
| 13 | End both recordings |

Return tavern frames for the normal WAV and write `lab17-forest.wav`
separately. Check the invalid request's error, then continue the scenario.

```sh
go run . lab17.wav
ffplay -nodisp -autoexit -loglevel error -volume 20 lab17.wav
ffplay -nodisp -autoexit -loglevel error -volume 20 lab17-forest.wav
```

Tavern begins low-left at effective gain 0.4, mutes smoothly, comes back
toward high-right, survives the invalid snapshot, then clears immediately
with the empty snapshot. Forest remains high-right until its own natural
end and must not respond to tavern's commands.

## Verify

Omitted fields preserve prior settings. A zero volume is not dropped.
Rejected snapshots preserve samples, revision, and existing source ownership.
Closing a table during source loading cannot recreate it on completion.
Remove a track from just one table, then implement the library-removal form
which removes that ID from all tables. Confirm close cancels owned decoders.

Map the application boundary without rebuilding its infrastructure:
authenticated GM connection produces `mixerReady`, a reconnect sends
`mixChanged`, canonical `playbackChanged` patches update transport, and
`mixVolumesChanged` updates type/master gain. Successful commands receive
`mixerApplied`; errors produce `mixerFailed` and logging. Table identity
comes from authenticated event metadata, not client JSON.

To model exclusive Music playback, feed a pause patch for the old Music
track followed by the new play patch; the core's independent envelopes
create the audible transition while Ambience continues. Exclusivity is
caller policy, not something `Mixer` infers from IDs. Do not promise
transactional rollback of an entire event batch: the adapter applies its
individual updates in order. Table deletion and last-GM disconnection
close the session; plain player playback snapshots do not mutate it.

References are `internal/livemixer/service.go` and
`internal/livemixer/events.go`; see the
[live-mixer event boundary](../discord/live-mixer.md).
Finally, add [clocked delivery and subscribers](./18-live-delivery.md).
