---
type: UI Surface
title: Track processing states
description: Processing indicators, failed-track diagnostics, and deletion in tables and the library.
resource: ui/src/components/AudioControls.vue
tags: [frontend, processing, library]
---

# Track processing states

Accepted [uploads and imports](./add-track-dialog.md) appear immediately with
playback disabled while [background processing](../data/track-processing.md)
runs. They do not load audio or waveforms until ready.

## Indicators

In the [track grid](./track-grid.md), the spinner replaces the three-dot menu
in its existing slot, keeping tile height unchanged. Hovering displays a
processing tooltip. Processing tracks remain reorderable by dragging or
keyboard. A warning icon replaces the spinner on failure without changing
tile height. The [shared library](./shared-library.md) displays the same
spinner-to-warning transition in its preview column.

## Failure diagnostics and deletion

Clicking a failed track or its warning icon, or activating it by keyboard,
opens a diagnostics dialog with a scrollable, selectable code-style log and a
prominent Delete track action. Failed tracks do not show playback or metadata
editing controls. [YouTube diagnostics](../backend/youtube.md) include the
command, output, and error.

Deleting from either view uses the library's
[usage-aware confirmation flow](./shared-library.md#deletion) and removes the
track from the library and every associated table. Ready table tracks retain
their normal Remove from table action.
