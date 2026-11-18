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

## Audio files and folders

GMs can drop audio files anywhere in the main area of the
[table](./track-grid.md) or [library](./shared-library.md). A drop opens the
upload tab with the files ready for review, without using the dialog button.
Browsers that expose folder contents during drag-and-drop include audio from
nested folders. Non-audio, empty, or unreadable dropped items are skipped and
reported.

Each track needs a nonblank name and a type. Names are prefilled from the
filenames without extensions. The batch upload table lets GMs edit names and
types per row, or select some or all rows and apply a type to just those rows.
Selection only controls bulk edits; all rows are uploaded on confirmation.
The dialog uploads each file separately to the current table and library, or
to the library alone when opened there. If some upload requests fail, only
those rows remain for retry, retaining their edited names and types. The
server-side processing is described in [track uploads](../data/track-uploads.md).

## YouTube links

YouTube imports are also available from the library's upload dialog. Pasting
or editing a link triggers a debounced, cancellable
[metadata scan](../backend/youtube.md). It fills in the track name, which
remains disabled until a scan succeeds. Duration and audio size appear as
read-only text in the dialog's bottom-left footer without shifting the
track-type selector. Failed scans and limit violations turn the URL field red
with the specific error and disable Import.

Imports require a successful scan, nonblank display name, and track type.
Each accepted upload or import adds a track with the
[processing presentation](./track-processing.md) until it is ready. The
dialog closes once all submissions are accepted, without waiting for processing.

## Feedback

Upload results appear as compact, auto-dismissing bottom-center toasts attached
to the document body, independent of the uploader's app-bar or library
placement; upload failures remain inline while the dialog is open.
