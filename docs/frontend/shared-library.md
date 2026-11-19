---
type: UI Surface
title: Shared library
description: Library metadata table, local previews, and bulk type changes and deletion.
resource: ui/src/components/FileList.vue
tags: [frontend, library]
---

# Shared library

The `/library` [route](./routes.md) uses a sortable, paginated metadata table
showing each track's name, type, and creation time.

## Previews

Row clicks (or keyboard activation) toggle a local preview, with at most one
preview playing at once. Resuming a paused preview continues from its current
position and retains its loaded duration and playback region. Library previews
reuse the HLS player (see [browser playback](../playback/browser-playback.md))
without sending synchronization messages. The library opens a GM WebSocket
to receive processing status and metadata updates through the
[WebSocket protocol](../playback/websocket-protocol.md).
Preview state is cleared on leaving the library; it cannot become a table's
playback state.

Processing tracks show a spinner instead of preview controls. Failed tracks
show a warning icon; clicking the row or warning opens the shared
[diagnostics dialog](./track-processing.md).

## Editing

Track editing is shared with the table tiles through the
[track editor](./track-editor.md) dialog's activator slot. Opening library
controls loads audio metadata without starting playback or sending
synchronization messages.

Select tracks using the row checkboxes, or use **Select all library tracks**
to select across pages. Tracks still processing cannot be selected. The
floating action bar shows the selection count and uses the same **Set track
type** menu as the [uploader](./add-track-dialog.md). In the library, choosing
a type saves it immediately for each selected track.

## Deletion

Before deleting a library track, the GM-only usage endpoint counts its current
table associations. When that count is nonzero, a separate follow-up dialog
states how many tables lose the track and must be confirmed; unused tracks
delete immediately. Deletion is blocked if the usage lookup fails. The deletion
compares the confirmed count atomically with the current associations; if it
changed, the GM must check usage again before deleting. See
[table track associations](../data/table-track-associations.md) for what
deletion removes.

Bulk deletion always asks for confirmation, even when none of the tracks are
used by tables. The confirmation shows the number of tracks to be deleted and
the table count for each track in use. If any usage lookup fails, nothing is
deleted. Bulk changes are saved per track; if some fail, those tracks stay
selected for retry.
