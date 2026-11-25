---
type: Workflow
title: Track uploads and media files
description: Upload validation, FFmpeg HLS conversion, waveforms, and deletion.
resource: internal/library
tags: [data, media, ffmpeg, hls]
---

# Track uploads and media files

An upload is multipart audio plus a display name, type UUID, and optional table
UUID. The library service validates the table and type and delegates to the
[music processing service](./track-processing.md), which assigns a UUIDv7 and
stages the input under `<upload-dir>/.processing/<track-id>/`. It saves a
processing library entry and optional
[association](./table-track-associations.md) in one database transaction,
then returns HTTP 202 with the track.

A service worker runs FFmpeg to produce AAC HLS playlists and `.ts` segments
under `<upload-dir>/<track-id>/`. Streaming reads files from that upload
directory only when the track is ready, scoped by the
[public table link](../backend/public-table-links.md).
[YouTube imports](../backend/youtube.md) use the same conversion path.

## Waveforms

After conversion the processing service also computes a 400-bucket peak
waveform with FFmpeg and stores it in the track's SQLite JSON column.
The GM-only `GET /api/v1/files/{trackID}/waveform` endpoint reads it from SQLite.
If a ready track has no stored waveform, the endpoint generates one from HLS
and saves it using the request context.
The [track editor](../frontend/track-editor.md) draws it as its timeline.

## Deletion

Deletion removes metadata and the associated HLS directory. These filesystem
and database operations are not a single atomic transaction.

The stored HLS segments are also the input for the
[Discord PCM source](../discord/pcm-source.md). The upload UI is described in
[Add Track dialog](../frontend/add-track-dialog.md).
