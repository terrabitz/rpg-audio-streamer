---
type: UI Surface
title: Shared library
description: Library metadata table, local previews, and usage-checked deletion.
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
without opening a WebSocket or sending synchronization messages. Preview state
is cleared on leaving the library; it cannot become a table's playback state.

## Editing

Track editing is shared with the table tiles through the
[track editor](./track-editor.md) dialog's activator slot. Opening library
controls loads audio metadata without starting playback or sending
synchronization messages.

## Deletion

Before deleting a library track, the GM-only usage endpoint counts its current
table associations. When that count is nonzero, a separate follow-up dialog
states how many tables lose the track and must be confirmed; unused tracks
delete immediately. Deletion is blocked if the usage lookup fails. The deletion
compares the confirmed count atomically with the current associations; if it
changed, the GM must check usage again before deleting. See
[table track associations](../data/table-track-associations.md) for what
deletion removes.
