---
type: UI Surface
title: Add Track dialog
description: Library selection, new-upload, and YouTube-link tabs for adding tracks.
resource: ui/src/components/AudioUploader.vue
tags: [frontend, uploads, library]
---

# Add Track dialog

**Add Track** (in the [app shell](./app-shell.md)) opens a dialog with library
selection, new-upload, and YouTube-link tabs; none adds a permanent control row to
the [track grid](./track-grid.md).

Uploads require a file, a nonblank name, and a valid library track type before
the Upload action is enabled. The server side of uploads is described in
[track uploads](../data/track-uploads.md).

## YouTube links

YouTube imports are also available from the library's upload dialog. Pasting
or editing a link triggers a debounced, cancellable
[metadata scan](../backend/youtube.md). It fills in the track name, which
remains disabled until a scan succeeds. Duration and audio size appear as
read-only text in the dialog's bottom-left footer without shifting the
track-type selector. Failed scans and limit violations turn the URL field red
with the specific error and disable Import.

Imports require a successful scan, nonblank display name, and track type.
Accepted uploads and imports close the dialog immediately and add a track with
the [processing presentation](./track-processing.md) until it is ready.

## Feedback

Upload results appear as compact, auto-dismissing bottom-center toasts attached
to the document body, independent of the uploader's app-bar or library
placement; upload failures remain inline while the dialog is open.
