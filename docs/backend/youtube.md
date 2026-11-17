---
type: Component
title: YouTube retrieval
description: Canonical video URLs, yt-dlp metadata scanning, download limits, and diagnostics.
resource: internal/youtube/youtube.go
tags: [backend, youtube, media]
---

# YouTube retrieval

The YouTube service retrieves individual public videos through `yt-dlp`.
Watch, shortened, Shorts, embed, and live-video URLs are validated and reduced
to a canonical HTTPS video URL. Playlist-only and non-YouTube links are rejected;
playlist parameters on video links are discarded.

## Metadata scans

GMs scan a video through `POST /api/v1/files/scan`. The request retrieves
metadata without downloading audio, returning the title, duration, and selected
audio format's exact or estimated byte size; an unknown size remains null.
Live streams, videos over three hours, and known or estimated audio sizes over
200 MiB are rejected. The scan has a one-minute deadline and receives its
request context explicitly, so it is cancelled when the request ends.

The [Add Track dialog](../frontend/add-track-dialog.md) scans links before
enabling import. Download limits are also checked during retrieval because
estimates and upstream availability can change.

## Imports

`POST /api/v1/files/import` accepts a display name, track-type UUID, and optional
table UUID, then delegates to [background processing](../data/track-processing.md).
The downloader runs without user configuration, plugins, or shell interpolation,
selecting audio into the job's staging directory. Imports exclude live streams
and videos over three hours or 200 MiB of source audio.

Retrieval failures include the shell-quoted command, combined stdout/stderr,
and error. Captured output is limited to 1 MiB with an explicit truncation
notice. These diagnostics feed the GM-only
[failure dialog](../frontend/track-processing.md).

Local servers require a current `yt-dlp[default]` installation, Node.js 20 or
newer for YouTube JavaScript challenges, and FFmpeg on `PATH`. Runtime and
development container images include these dependencies. YouTube availability,
access restrictions, and extractor compatibility can still prevent retrieval.
