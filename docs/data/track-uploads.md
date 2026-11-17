---
type: Workflow
title: Track uploads and media files
description: Upload validation, FFmpeg HLS conversion, waveforms, and deletion.
resource: internal/server
tags: [data, media, ffmpeg, hls]
---

# Track uploads and media files

An upload is multipart audio plus a display name, type UUID, and optional table
UUID. The server validates the type, assigns a UUIDv7, stages the input, and
runs FFmpeg to produce AAC HLS playlists and `.ts` segments under
`<upload-dir>/<track-id>/`. It then saves metadata; table uploads save the
library entry and [association](./table-track-associations.md) in one database
transaction. Streaming reads files from that upload directory, scoped by the
[public table link](../backend/public-table-links.md).

## Waveforms

After conversion the server also computes a 400-bucket peak waveform with
FFmpeg and caches it as `waveform.json` beside the playlist; a failure there
only logs a warning. The GM-only `GET /api/v1/files/{trackID}/waveform`
endpoint serves that cache, generating it on first request for older tracks.
The [track editor](../frontend/track-editor.md) draws it as its timeline.

## Deletion

Deletion removes metadata and the associated HLS directory. These filesystem
and database operations are not a single atomic transaction.

The stored HLS segments are also the input for the
[Discord PCM source](../discord/pcm-source.md). The upload UI is described in
[Add Track dialog](../frontend/add-track-dialog.md).
